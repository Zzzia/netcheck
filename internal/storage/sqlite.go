package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"netcheck/internal/model"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", filepath.ToSlash(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 sqlite 数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) init() error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts TEXT NOT NULL,
			layer TEXT NOT NULL,
			metric TEXT NOT NULL,
			target TEXT NOT NULL,
			success INTEGER NOT NULL,
			latency_ms REAL,
			value REAL,
			bytes_rx INTEGER,
			detail TEXT,
			error_text TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples(ts)`,
		`CREATE INDEX IF NOT EXISTS idx_samples_layer_metric ON samples(layer, metric, ts)`,
		`CREATE TABLE IF NOT EXISTS state_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			summary TEXT NOT NULL,
			evidence TEXT NOT NULL,
			started_at TEXT NOT NULL,
			ended_at TEXT,
			duration_sec INTEGER NOT NULL DEFAULT 0,
			is_open INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE INDEX IF NOT EXISTS idx_state_events_time ON state_events(started_at, ended_at)`,
	}
	for _, statement := range schema {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("初始化数据库失败: %w", err)
		}
	}
	return nil
}

func (s *Store) InsertSample(sample model.Sample) error {
	_, err := s.db.Exec(
		`INSERT INTO samples(ts, layer, metric, target, success, latency_ms, value, bytes_rx, detail, error_text)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sample.Timestamp.UTC().Format(time.RFC3339Nano),
		sample.Layer,
		sample.Metric,
		sample.Target,
		boolToInt(sample.Success),
		nullableFloat(sample.LatencyMs),
		nullableFloat(sample.Value),
		nullableInt(sample.BytesRX),
		sample.Detail,
		sample.ErrorText,
	)
	if err != nil {
		return fmt.Errorf("写入采样失败: %w", err)
	}
	return nil
}

func (s *Store) BeginEvent(name, status, summary, evidence string, startedAt time.Time) (int64, error) {
	result, err := s.db.Exec(
		`INSERT INTO state_events(name, status, summary, evidence, started_at, is_open)
		 VALUES (?, ?, ?, ?, ?, 1)`,
		name,
		status,
		summary,
		evidence,
		startedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, fmt.Errorf("写入事件开始失败: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取事件 ID 失败: %w", err)
	}
	return id, nil
}

func (s *Store) EndEvent(id int64, endedAt time.Time) error {
	row := s.db.QueryRow(`SELECT started_at FROM state_events WHERE id = ?`, id)
	var startedAtRaw string
	if err := row.Scan(&startedAtRaw); err != nil {
		return fmt.Errorf("读取事件开始时间失败: %w", err)
	}
	startedAt, err := time.Parse(time.RFC3339Nano, startedAtRaw)
	if err != nil {
		return fmt.Errorf("解析事件开始时间失败: %w", err)
	}
	durationSec := int64(endedAt.Sub(startedAt).Seconds())
	if durationSec < 0 {
		durationSec = 0
	}
	_, err = s.db.Exec(
		`UPDATE state_events
		 SET ended_at = ?, duration_sec = ?, is_open = 0
		 WHERE id = ?`,
		endedAt.UTC().Format(time.RFC3339Nano),
		durationSec,
		id,
	)
	if err != nil {
		return fmt.Errorf("结束事件失败: %w", err)
	}
	return nil
}

func (s *Store) LoadOpenEvents() ([]model.Event, error) {
	rows, err := s.db.Query(
		`SELECT id, name, status, summary, evidence, started_at, ended_at, duration_sec, is_open
		 FROM state_events
		 WHERE is_open = 1
		 ORDER BY started_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询打开事件失败: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

func (s *Store) LoadSamplesSince(since time.Time) ([]model.Sample, error) {
	rows, err := s.db.Query(
		`SELECT id, ts, layer, metric, target, success, COALESCE(latency_ms, 0), COALESCE(value, 0), COALESCE(bytes_rx, 0), COALESCE(detail, ''), COALESCE(error_text, '')
		 FROM samples
		 WHERE ts >= ?
		 ORDER BY ts ASC`,
		since.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("查询采样失败: %w", err)
	}
	defer rows.Close()

	var samples []model.Sample
	for rows.Next() {
		var (
			item model.Sample
			ts   string
			ok   int
		)
		if err := rows.Scan(&item.ID, &ts, &item.Layer, &item.Metric, &item.Target, &ok, &item.LatencyMs, &item.Value, &item.BytesRX, &item.Detail, &item.ErrorText); err != nil {
			return nil, fmt.Errorf("读取采样失败: %w", err)
		}
		item.Success = ok == 1
		item.Timestamp, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmt.Errorf("解析采样时间失败: %w", err)
		}
		samples = append(samples, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历采样结果失败: %w", err)
	}
	return samples, nil
}

func (s *Store) LoadEventsSince(since time.Time) ([]model.Event, error) {
	rows, err := s.db.Query(
		`SELECT id, name, status, summary, evidence, started_at, ended_at, duration_sec, is_open
		 FROM state_events
		 WHERE started_at >= ? OR (ended_at IS NOT NULL AND ended_at >= ?) OR is_open = 1
		 ORDER BY started_at ASC`,
		since.UTC().Format(time.RFC3339Nano),
		since.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("查询事件失败: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

func scanEvents(rows *sql.Rows) ([]model.Event, error) {
	var events []model.Event
	for rows.Next() {
		var (
			item        model.Event
			startedAt   string
			endedAtRaw  sql.NullString
			isOpenValue int
		)
		if err := rows.Scan(&item.ID, &item.Name, &item.Status, &item.Summary, &item.Evidence, &startedAt, &endedAtRaw, &item.DurationSec, &isOpenValue); err != nil {
			return nil, fmt.Errorf("读取事件失败: %w", err)
		}
		item.IsOpen = isOpenValue == 1
		parsedStartedAt, parseErr := time.Parse(time.RFC3339Nano, startedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("解析事件开始时间失败: %w", parseErr)
		}
		item.StartedAt = parsedStartedAt
		if endedAtRaw.Valid {
			endedAt, parseErr := time.Parse(time.RFC3339Nano, endedAtRaw.String)
			if parseErr != nil {
				return nil, fmt.Errorf("解析事件结束时间失败: %w", parseErr)
			}
			item.EndedAt = &endedAt
		}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历事件结果失败: %w", err)
	}
	return events, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableFloat(value float64) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullableInt(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
