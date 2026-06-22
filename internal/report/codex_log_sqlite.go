package report

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const maxCodexSQLiteRows = 200000

func buildCodexReportFromSQLite(dbPath string, start, end time.Time) codexReport {
	codexStart := start
	clamped := false
	if end.Sub(start) > maxCodexWindow {
		codexStart = end.Add(-maxCodexWindow)
		clamped = true
	}
	report := codexReport{
		LogPath:    dbPath,
		RangeLabel: fmt.Sprintf("%s ~ %s", codexStart.Local().Format("2006-01-02 15:04"), end.Local().Format("2006-01-02 15:04")),
		RangeStart: codexStart.Local().Format("2006-01-02 15:04"),
		RangeEnd:   end.Local().Format("2006-01-02 15:04"),
		Clamped:    clamped,
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)", dbPath))
	if err != nil {
		report.Error = fmt.Sprintf("打开 Codex SQLite 日志失败: %v", err)
		return report
	}
	defer db.Close()

	parsed, err := parseCodexSQLiteRows(db, codexStart, end)
	if err != nil {
		report.Error = fmt.Sprintf("读取 Codex SQLite 日志失败: %v", err)
		return report
	}
	return parsed.finalize(report, codexStart, end)
}

func parseCodexSQLiteRows(db *sql.DB, start, end time.Time) (codexParseResult, error) {
	rows, err := db.Query(`
		SELECT ts, ts_nanos, level, target, COALESCE(feedback_log_body, '')
		FROM logs
		WHERE ts >= ?
		  AND ts <= ?
		  AND target <> 'log'
		  AND feedback_log_body NOT LIKE '%ToolCall:%'
		  AND feedback_log_body NOT LIKE '%event.name="codex.tool_result"%'
		  AND (
		      level IN ('WARN', 'ERROR')
		      OR feedback_log_body LIKE '%stream disconnected - retrying sampling request%'
		      OR feedback_log_body GLOB '*codex.turn.token_usage.input_tokens=[0-9]*'
		      OR (feedback_log_body LIKE '%codex_core::tasks: close%' AND feedback_log_body LIKE '%session_task.turn%')
		      OR (
		          target = 'codex_api::endpoint::responses_websocket'
		          AND feedback_log_body LIKE '%websocket event: {"type":"response.completed"%'
		      )
		      OR (
		          target = 'codex_core::session::turn'
		          AND feedback_log_body LIKE '%post sampling token usage%'
		          AND feedback_log_body LIKE '%session_task.turn%'
		          AND feedback_log_body NOT LIKE '%ToolCall:%'
		      )
		  )
		ORDER BY ts ASC, ts_nanos ASC
		LIMIT ?`,
		start.Unix()-1,
		end.Unix()+1,
		maxCodexSQLiteRows,
	)
	if err != nil {
		return codexParseResult{}, err
	}
	defer rows.Close()

	var builder strings.Builder
	for rows.Next() {
		var (
			seconds int64
			nanos   int64
			level   string
			target  string
			body    string
		)
		if err := rows.Scan(&seconds, &nanos, &level, &target, &body); err != nil {
			return codexParseResult{}, err
		}
		builder.WriteString(formatCodexSQLiteLine(seconds, nanos, level, target, body))
		builder.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		return codexParseResult{}, err
	}
	return parseCodexLogWithLineMode(strings.NewReader(builder.String()), start, end, false)
}

func formatCodexSQLiteLine(seconds, nanos int64, level, target, body string) string {
	timestamp := time.Unix(seconds, nanos).UTC().Format(time.RFC3339Nano)
	body = strings.TrimSpace(body)
	if strings.TrimSpace(target) == "" {
		return fmt.Sprintf("%s %s %s", timestamp, level, body)
	}
	return fmt.Sprintf("%s %s %s: %s", timestamp, level, target, body)
}
