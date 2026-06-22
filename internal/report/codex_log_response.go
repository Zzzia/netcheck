package report

import (
	"encoding/json"
	"strings"
)

type codexResponseCompletedLog struct {
	Type     string `json:"type"`
	Response struct {
		Status      string `json:"status"`
		CreatedAt   int64  `json:"created_at"`
		CompletedAt int64  `json:"completed_at"`
		Usage       *struct {
			InputTokens        int `json:"input_tokens"`
			InputTokensDetails *struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
			OutputTokens        int `json:"output_tokens"`
			OutputTokensDetails *struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	} `json:"response"`
}

func isResponseCompletedSampling(raw string) bool {
	return strings.Contains(raw, `websocket event: {"type":"response.completed"`) ||
		strings.Contains(raw, `Received message {"type":"response.completed"`)
}

func parseResponseCompletedTurnStats(raw string) (codexTurnStats, bool) {
	if !isResponseCompletedSampling(raw) {
		return codexTurnStats{}, false
	}
	jsonStart := strings.Index(raw, `{"type":"response.completed"`)
	if jsonStart < 0 {
		return codexTurnStats{}, false
	}

	var event codexResponseCompletedLog
	decoder := json.NewDecoder(strings.NewReader(raw[jsonStart:]))
	if err := decoder.Decode(&event); err != nil {
		return codexTurnStats{}, false
	}
	if event.Type != "response.completed" || event.Response.Status != "completed" {
		return codexTurnStats{}, false
	}

	stats := codexTurnStats{}
	if event.Response.CompletedAt >= event.Response.CreatedAt && event.Response.CreatedAt > 0 {
		stats.durationSec = float64(event.Response.CompletedAt - event.Response.CreatedAt)
	}
	if usage := event.Response.Usage; usage != nil {
		stats.inputTokens = usage.InputTokens
		stats.outputTokens = usage.OutputTokens
		if details := usage.OutputTokensDetails; details != nil {
			stats.reasonTokens = details.ReasoningTokens
		}
	}
	return stats, true
}

func parseCompletedTurnStats(raw string, latestSamplingStats codexTurnStats) (codexTurnStats, bool) {
	if stats, ok := parseLegacyCompletedTurnStats(raw); ok {
		return stats, true
	}
	if isFinalPostSamplingTurn(raw) {
		return latestSamplingStats, true
	}
	return codexTurnStats{}, false
}

func parseLegacyCompletedTurnStats(raw string) (codexTurnStats, bool) {
	duration, ok := parseLegacyTurnDuration(raw)
	if !ok {
		return codexTurnStats{}, false
	}
	return codexTurnStats{
		durationSec:  duration,
		inputTokens:  firstIntMatch(inputTokenPattern, raw),
		outputTokens: firstIntMatch(outputTokenPattern, raw),
		reasonTokens: firstIntMatch(reasoningTokenPattern, raw),
	}, true
}

func parseLegacyTurnDuration(raw string) (float64, bool) {
	if strings.Contains(raw, "ToolCall:") || !strings.Contains(raw, "session_task.turn") {
		return 0, false
	}
	if !strings.Contains(raw, "codex_core::tasks: close") {
		return 0, inputTokenPattern.FindStringSubmatch(raw) != nil
	}
	match := turnIdlePattern.FindStringSubmatch(raw)
	if match == nil {
		return 0, false
	}
	return parseCodexDuration(match[1], match[2]), true
}

func isFinalPostSamplingTurn(raw string) bool {
	return isPostSamplingTurn(raw) &&
		strings.Contains(raw, "model_needs_follow_up=false") &&
		strings.Contains(raw, "has_pending_input=false") &&
		strings.Contains(raw, "needs_follow_up=false")
}

func isPostSamplingTurn(raw string) bool {
	return strings.Contains(raw, "post sampling token usage") &&
		strings.Contains(raw, "session_task.turn") &&
		!isCodexToolEcho(raw)
}
