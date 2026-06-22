package report

import (
	"strings"
	"time"
)

func isModernStreamRequest(raw string) bool {
	return isPostSamplingTurn(raw)
}

func isLegacyStreamRequest(raw string) bool {
	return isStreamClose(raw) || isResponseCompletedSampling(raw)
}

func (r *codexParseResult) streamRequests() int {
	if r.modernStreamRequests > 0 {
		return r.modernStreamRequests
	}
	return r.legacyStreamRequests
}

func (r *codexParseResult) streamRequestsForBucket(bucket time.Time) int {
	if r.modernStreamRequests > 0 {
		return r.modernStreamTimeline[bucket]
	}
	return r.legacyStreamTimeline[bucket]
}

func (r *codexParseResult) addModernStreamRequest(bucket time.Time) {
	r.modernStreamRequests++
	r.modernStreamTimeline[bucket]++
}

func (r *codexParseResult) addLegacyStreamRequest(bucket time.Time) {
	r.legacyStreamRequests++
	r.legacyStreamTimeline[bucket]++
}

func parseCodexTarget(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) < 3 {
		return ""
	}
	return strings.TrimSuffix(fields[2], ":")
}

func isTrueRetryLog(item codexParsedLine) bool {
	if isCodexToolEcho(item.raw) {
		return false
	}
	if item.target == "codex_core::responses_retry" {
		return true
	}
	return item.level == "WARN" &&
		strings.Contains(item.raw, "codex_core::session::turn: stream disconnected - retrying sampling request")
}
