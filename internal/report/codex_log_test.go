package report

import (
	"os"
	"path/filepath"
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

func TestBuildCodexReportCurrentLogSmoke(t *testing.T) {
	if os.Getenv("NETCHECK_TEST_CURRENT_CODEX_LOG") != "1" {
		t.Skip("设置 NETCHECK_TEST_CURRENT_CODEX_LOG=1 时读取当前本机 Codex 日志做人工核对")
	}
	end := time.Now()
	report := buildCodexReport(end.Add(-24*time.Hour), end)
	if !report.Available {
		t.Fatalf("当前 Codex 日志不可用: %s", report.Error)
	}
	t.Logf("Codex 近 24h: retry=%d affected_turns=%d tool_errors=%d network_candidates=%d apps_errors=%d noise=%d",
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
