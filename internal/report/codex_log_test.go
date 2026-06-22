package report

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	testThreadID = "019dfc09-1075-7f80-bc38-2f15f14503c6"
	testTurnID   = "019dfc21-8a2d-7f52-bafe-556a0b2cc12c"
)

func TestParseCodexLogClassifiesEventsAndNoise(t *testing.T) {
	start := time.Date(2026, 5, 6, 14, 0, 0, 0, time.Local)
	end := start.Add(time.Hour)
	log := strings.Join([]string{
		codexLine("2026-05-06T06:01:00.000000Z", "INFO", "model_client.stream_responses_websocket{model=gpt-5.5 wire_api=responses}: codex_core::client: close time.busy=1ms time.idle=6.2s"),
		codexLine("2026-05-06T06:01:01.000000Z", "WARN", "codex_core::session::turn: stream disconnected - retrying sampling request (1/5 in 195ms)..."),
		codexLine("2026-05-06T06:01:02.000000Z", "INFO", `codex_core::stream_events_utils: ToolCall: exec_command {"cmd":"rg stream disconnected - retrying sampling request /tmp/log"}`),
		codexLine("2026-05-06T06:02:00.000000Z", "ERROR", "codex_core::tools::router: error=write_stdin failed: stdin is closed for this session; rerun exec_command with tty=true to keep stdin open"),
		codexLine("2026-05-06T06:03:00.000000Z", "WARN", "codex_core::session::turn: failed to load discoverable tool suggestions: request failed with status 403 Forbidden: <html>"),
		codexLine("2026-05-06T06:04:00.000000Z", "WARN", "codex_core::codex: startup websocket prewarm setup failed: unexpected status 503 Service Unavailable"),
		codexLine("2026-05-06T06:05:00.000000Z", "ERROR", "codex_core::session: failed to record rollout items: thread 019dfc21-8a2d-7f52-bafe-556a0b2cc12c not found"),
		codexLine("2026-05-06T06:06:00.000000Z", "WARN", "codex_core_skills::loader: ignoring interface.icon_small: icon path must not contain '..'"),
		codexLine("2026-05-06T06:07:00.000000Z", "INFO", "codex_core::tasks: close time.busy=1ms time.idle=70s codex.turn.token_usage.input_tokens=100 codex.turn.token_usage.output_tokens=20 codex.turn.token_usage.reasoning_output_tokens=5"),
		codexLine("2026-05-06T06:08:00.000000Z", "WARN", "codex_core::app_server: app-server event consumer lagged; dropping ignored events skipped=753"),
	}, "\n")

	parsed, err := parseCodexLog(strings.NewReader(log), start, end)
	if err != nil {
		t.Fatalf("解析 Codex 日志失败: %v", err)
	}
	report := parsed.finalize(codexReport{}, start, end)
	if !report.Available {
		t.Fatal("期望 Codex 报告可用")
	}
	summary := report.Summary
	if summary.StreamRequests != 1 || summary.CompletedTurns != 1 {
		t.Fatalf("采样流或 turn 统计不符合预期: %#v", summary)
	}
	if summary.RetryEvents != 1 || summary.RetryAffectedTurns != 1 || summary.MaxRetryAttempt != "1/5" {
		t.Fatalf("重试统计不符合预期: %#v", summary)
	}
	if summary.ToolErrors != 1 || summary.AppsErrors != 1 || summary.NetworkCandidates != 1 || summary.RolloutErrors != 1 {
		t.Fatalf("异常分类统计不符合预期: %#v", summary)
	}
	if summary.UnknownEvents != 1 {
		t.Fatalf("未知 WARN 统计不符合预期: %#v", summary)
	}
	if summary.NoiseWarnings != 1 {
		t.Fatalf("期望降噪 WARN 为 1，实际为 %d", summary.NoiseWarnings)
	}
	if len(report.Events) != 2 {
		t.Fatalf("主事件明细只应保留网络相关事件，实际为 %d: %#v", len(report.Events), report.Events)
	}
	if len(report.OtherSummary) == 0 {
		t.Fatal("非网络 WARN/ERROR 应进入统计表")
	}
	for _, event := range report.Events {
		if strings.Contains(event.Summary, "ToolCall") {
			t.Fatalf("ToolCall 回显不应进入异常事件: %#v", event)
		}
		if event.Kind != "stream_retry" && event.Kind != "network_candidate" {
			t.Fatalf("非网络事件不应进入主明细: %#v", event)
		}
	}
}

func TestBuildCodexReportClampsToTwentyFourHours(t *testing.T) {
	end := time.Date(2026, 5, 6, 15, 0, 0, 0, time.Local)
	start := end.Add(-48 * time.Hour)
	dir := t.TempDir()
	logPath := filepath.Join(dir, "codex-tui.log")
	log := strings.Join([]string{
		codexLine(end.Add(-25*time.Hour).UTC().Format(time.RFC3339Nano), "WARN", "codex_core::session::turn: stream disconnected - retrying sampling request (1/5 in 195ms)..."),
		codexLine(end.Add(-time.Hour).UTC().Format(time.RFC3339Nano), "WARN", "codex_core::session::turn: stream disconnected - retrying sampling request (1/5 in 195ms)..."),
	}, "\n")
	if err := os.WriteFile(logPath, []byte(log), 0o644); err != nil {
		t.Fatalf("写入测试日志失败: %v", err)
	}
	report := buildCodexReportFromLog(logPath, start, end)
	if !report.Clamped {
		t.Fatal("期望超过 24 小时时 Codex 统计窗口被截断")
	}
	if report.Summary.RetryEvents != 1 {
		t.Fatalf("期望只统计最近 24 小时的 retry，实际为 %d", report.Summary.RetryEvents)
	}
}

func TestBuildCodexReportMissingLogIsNonFatal(t *testing.T) {
	report := buildCodexReportFromLog(filepath.Join(t.TempDir(), "missing.log"), time.Now().Add(-time.Hour), time.Now())
	if report.Available {
		t.Fatal("日志不存在时不应标记为可用")
	}
	if report.Error == "" {
		t.Fatal("日志不存在时应返回可解释错误")
	}
}

func TestBuildCodexReportFromSQLiteLog(t *testing.T) {
	start := time.Date(2026, 5, 6, 14, 0, 0, 0, time.Local)
	end := start.Add(time.Hour)
	dbPath := filepath.Join(t.TempDir(), "logs_2.sqlite")
	writeCodexSQLiteLog(t, dbPath, []codexSQLiteTestRow{
		{
			ts:     start.Add(time.Minute),
			level:  "WARN",
			target: "codex_core::responses_retry",
			body:   codexSQLiteBody("stream disconnected - retrying sampling request (1/5 in 194ms)..."),
		},
		{
			ts:     start.Add(2 * time.Minute),
			level:  "DEBUG",
			target: "codex_core::goals",
			body:   codexSQLiteBody("codex.turn.token_usage.input_tokens=100 codex.turn.token_usage.output_tokens=20 codex.turn.token_usage.reasoning_output_tokens=5"),
		},
		{
			ts:     start.Add(3 * time.Minute),
			level:  "INFO",
			target: "codex_core::stream_events_utils",
			body:   `ToolCall: exec_command {"cmd":"codex.turn.token_usage.input_tokens=999"}`,
		},
	})

	report := buildCodexReportFromSQLite(dbPath, start, end)
	if !report.Available {
		t.Fatalf("期望 SQLite Codex 报告可用: %s", report.Error)
	}
	if report.Summary.RetryEvents != 1 || report.Summary.RetryAffectedTurns != 1 || report.Summary.CompletedTurns != 1 {
		t.Fatalf("SQLite Codex 指标不符合预期: %#v", report.Summary)
	}
	if report.Events[0].Kind != "stream_retry" || report.Events[0].Attempt != "1/5" {
		t.Fatalf("SQLite 断流事件解析不符合预期: %#v", report.Events)
	}
	if report.Events[0].InputTokens != 100 || report.Events[0].OutputTokens != 20 || report.Events[0].ReasonTokens != 5 {
		t.Fatalf("SQLite turn token 统计不符合预期: %#v", report.Events[0])
	}
}

func TestBuildCodexReportFromSQLiteModernResponseCompleted(t *testing.T) {
	start := time.Date(2026, 6, 22, 11, 0, 0, 0, time.Local)
	end := start.Add(time.Hour)
	dbPath := filepath.Join(t.TempDir(), "logs_2.sqlite")
	writeCodexSQLiteLog(t, dbPath, []codexSQLiteTestRow{
		{
			ts:     start.Add(time.Minute),
			level:  "WARN",
			target: "codex_core::responses_retry",
			body:   codexSQLiteBody("stream disconnected - retrying sampling request (1/5 in 194ms)..."),
		},
		{
			ts:     start.Add(2 * time.Minute),
			level:  "TRACE",
			target: "codex_api::endpoint::responses_websocket",
			body:   codexSQLiteBody(`stream_request:model_client.stream_responses_websocket{}:responses_websocket.stream_request{}: websocket event: ` + codexResponseCompletedJSON("resp_1", 100, 20, 5)),
		},
		{
			ts:     start.Add(3 * time.Minute),
			level:  "TRACE",
			target: "codex_api::endpoint::responses_websocket",
			body:   codexSQLiteBody(`stream_request:model_client.stream_responses_websocket{}:responses_websocket.stream_request{}: websocket event: ` + codexResponseCompletedJSON("resp_2", 120, 30, 6)),
		},
		{
			ts:     start.Add(4 * time.Minute),
			level:  "TRACE",
			target: "codex_core::session::turn",
			body:   codexSQLiteBody("run_turn: post sampling token usage turn_id=" + testTurnID + " total_usage_tokens=150 model_needs_follow_up=false has_pending_input=false needs_follow_up=false"),
		},
	})

	report := buildCodexReportFromSQLite(dbPath, start, end)
	if !report.Available {
		t.Fatalf("期望 SQLite Codex 报告可用: %s", report.Error)
	}
	if report.Summary.StreamRequests != 1 || report.Summary.CompletedTurns != 1 || report.Summary.RetryAffectedTurns != 1 {
		t.Fatalf("现代 Codex SQLite 指标不符合预期: %#v", report.Summary)
	}
	if len(report.Events) != 1 {
		t.Fatalf("期望保留 1 条重试事件，实际为 %d: %#v", len(report.Events), report.Events)
	}
	event := report.Events[0]
	if event.InputTokens != 120 || event.OutputTokens != 30 || event.ReasonTokens != 6 {
		t.Fatalf("应使用最终采样的 token usage 填充事件: %#v", event)
	}
}

func TestBuildCodexReportFromSQLiteModernPostSamplingRequestDenominator(t *testing.T) {
	start := time.Date(2026, 6, 22, 11, 0, 0, 0, time.Local)
	end := start.Add(time.Hour)
	dbPath := filepath.Join(t.TempDir(), "logs_2.sqlite")
	writeCodexSQLiteLog(t, dbPath, []codexSQLiteTestRow{
		{
			ts:     start.Add(time.Minute),
			level:  "WARN",
			target: "codex_core::responses_retry",
			body:   codexSQLiteBody("stream disconnected - retrying sampling request (1/5 in 194ms)..."),
		},
		{
			ts:     start.Add(2 * time.Minute),
			level:  "TRACE",
			target: "codex_core::session::turn",
			body:   codexSQLiteBody("run_turn: post sampling token usage turn_id=" + testTurnID + " total_usage_tokens=100 model_needs_follow_up=true has_pending_input=false needs_follow_up=true"),
		},
		{
			ts:     start.Add(3 * time.Minute),
			level:  "TRACE",
			target: "codex_core::session::turn",
			body:   codexSQLiteBody("run_turn: post sampling token usage turn_id=" + testTurnID + " total_usage_tokens=120 model_needs_follow_up=true has_pending_input=false needs_follow_up=true"),
		},
		{
			ts:     start.Add(4 * time.Minute),
			level:  "TRACE",
			target: "codex_api::endpoint::responses_websocket",
			body:   codexSQLiteBody(`stream_request:model_client.stream_responses_websocket{}:responses_websocket.stream_request{}: websocket event: ` + codexResponseCompletedJSON("resp_1", 100, 20, 5)),
		},
		{
			ts:     start.Add(5 * time.Minute),
			level:  "TRACE",
			target: "codex_core::session::turn",
			body:   codexSQLiteBody("run_turn: post sampling token usage turn_id=" + testTurnID + " total_usage_tokens=150 model_needs_follow_up=false has_pending_input=false needs_follow_up=false"),
		},
		{
			ts:     start.Add(6 * time.Minute),
			level:  "TRACE",
			target: "codex_api::sse::responses",
			body:   codexSQLiteBody("assistant text mentions stream disconnected - retrying sampling request (2/5 in 401ms)..."),
		},
	})

	report := buildCodexReportFromSQLite(dbPath, start, end)
	if !report.Available {
		t.Fatalf("期望 SQLite Codex 报告可用: %s", report.Error)
	}
	if report.Summary.StreamRequests != 3 {
		t.Fatalf("现代采样请求应按 post sampling 计数且不叠加 response.completed，实际为 %#v", report.Summary)
	}
	if report.Summary.CompletedTurns != 1 || report.Summary.RetryEvents != 1 || report.Summary.RetryAffectedTurns != 1 {
		t.Fatalf("现代 Codex 完成 turn / retry 指标不符合预期: %#v", report.Summary)
	}
	if report.Summary.RetryEventRate != "33.3%" || report.Summary.RetryAffectedTurnRate != "100.0%" {
		t.Fatalf("retry 比率应分别使用采样请求和完成 turn 作为分母: %#v", report.Summary)
	}

	streamRequests := 0
	for _, point := range report.Timeline {
		streamRequests += point.StreamRequests
	}
	if streamRequests != 3 {
		t.Fatalf("timeline 应同步输出采样请求数，实际为 %d: %#v", streamRequests, report.Timeline)
	}
	if len(report.Events) != 1 || report.Events[0].Kind != "stream_retry" {
		t.Fatalf("只有真实 responses_retry 日志应进入 retry 事件，实际为 %#v", report.Events)
	}
}

func TestBuildCodexReportUsesSQLiteWhenTextLogMissing(t *testing.T) {
	start := time.Date(2026, 5, 6, 14, 0, 0, 0, time.Local)
	end := start.Add(time.Hour)
	home := t.TempDir()
	t.Setenv("HOME", home)
	dbPath := filepath.Join(home, ".codex", "logs_2.sqlite")
	writeCodexSQLiteLog(t, dbPath, []codexSQLiteTestRow{
		{
			ts:     start.Add(time.Minute),
			level:  "WARN",
			target: "codex_core::responses_retry",
			body:   codexSQLiteBody("stream disconnected - retrying sampling request (1/5 in 194ms)..."),
		},
	})

	report := buildCodexReport(start, end)
	if !report.Available {
		t.Fatalf("期望默认入口能读取 SQLite Codex 日志: %s", report.Error)
	}
	if report.LogPath != dbPath {
		t.Fatalf("期望默认入口选择 SQLite 日志，实际为 %s", report.LogPath)
	}
	if report.Summary.RetryEvents != 1 {
		t.Fatalf("期望读取 SQLite retry，实际为 %#v", report.Summary)
	}
}

func TestBuildCodexReportCurrentLogSmoke(t *testing.T) {
	if os.Getenv("NETCHECK_TEST_CURRENT_CODEX_LOG") != "1" {
		t.Skip("设置 NETCHECK_TEST_CURRENT_CODEX_LOG=1 时读取当前本机 Codex 日志做人工核对")
	}
	end := time.Now()
	window := 24 * time.Hour
	if raw := os.Getenv("NETCHECK_TEST_CODEX_WINDOW_HOURS"); raw != "" {
		hours, err := strconv.ParseFloat(raw, 64)
		if err != nil || hours <= 0 {
			t.Fatalf("NETCHECK_TEST_CODEX_WINDOW_HOURS 必须是正数小时，实际为 %q", raw)
		}
		window = time.Duration(hours * float64(time.Hour))
	}
	report := buildCodexReport(end.Add(-window), end)
	if !report.Available {
		t.Fatalf("当前 Codex 日志不可用: %s", report.Error)
	}
	t.Logf("Codex 近 %.1fh: retry=%d affected_turns=%d tool_errors=%d network_candidates=%d apps_errors=%d noise=%d",
		window.Hours(),
		report.Summary.RetryEvents,
		report.Summary.RetryAffectedTurns,
		report.Summary.ToolErrors,
		report.Summary.NetworkCandidates,
		report.Summary.AppsErrors,
		report.Summary.NoiseWarnings,
	)
	for index, event := range report.Events {
		if index >= 10 {
			break
		}
		t.Logf("event[%d] %s %s line=%d %s", index, event.Time, event.Kind, event.Line, event.Summary)
	}
}

func codexLine(ts, level, message string) string {
	return ts + " " + level + " session_loop{thread_id=" + testThreadID + "}:submission_dispatch{otel.name=\"op.dispatch.user_input_with_turn_context\" submission.id=\"" + testTurnID + "\" codex.op=\"user_input_with_turn_context\"}:turn{otel.name=\"session_task.turn\" thread.id=" + testThreadID + " turn.id=" + testTurnID + " model=gpt-5.5}: " + message
}

type codexSQLiteTestRow struct {
	ts     time.Time
	level  string
	target string
	body   string
}

func writeCodexSQLiteLog(t *testing.T, dbPath string, rows []codexSQLiteTestRow) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("创建 Codex SQLite 测试目录失败: %v", err)
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", dbPath))
	if err != nil {
		t.Fatalf("打开 Codex SQLite 测试库失败: %v", err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts INTEGER NOT NULL,
		ts_nanos INTEGER NOT NULL,
		level TEXT NOT NULL,
		target TEXT NOT NULL,
		feedback_log_body TEXT,
		thread_id TEXT
	)`)
	if err != nil {
		t.Fatalf("创建 Codex SQLite 测试表失败: %v", err)
	}
	for _, row := range rows {
		_, err := db.Exec(
			`INSERT INTO logs(ts, ts_nanos, level, target, feedback_log_body, thread_id) VALUES (?, ?, ?, ?, ?, ?)`,
			row.ts.Unix(),
			row.ts.Nanosecond(),
			row.level,
			row.target,
			row.body,
			testThreadID,
		)
		if err != nil {
			t.Fatalf("写入 Codex SQLite 测试日志失败: %v", err)
		}
	}
}

func codexSQLiteBody(message string) string {
	return "session_loop{thread_id=" + testThreadID + "}:submission_dispatch{otel.name=\"op.dispatch.user_input\" submission.id=\"" + testTurnID + "\" codex.op=\"user_input\"}:turn{otel.name=\"session_task.turn\" thread.id=" + testThreadID + " turn.id=" + testTurnID + " model=gpt-5.5}: " + message
}

func codexResponseCompletedJSON(responseID string, inputTokens, outputTokens, reasoningTokens int) string {
	return fmt.Sprintf(`{"type":"response.completed","response":{"id":%q,"created_at":1782100000,"status":"completed","completed_at":1782100007,"usage":{"input_tokens":%d,"input_tokens_details":{"cached_tokens":0},"output_tokens":%d,"output_tokens_details":{"reasoning_tokens":%d},"total_tokens":%d}}}`,
		responseID,
		inputTokens,
		outputTokens,
		reasoningTokens,
		inputTokens+outputTokens,
	)
}
