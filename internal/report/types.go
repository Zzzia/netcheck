package report

import "html/template"

type summaryMetric struct {
	Label string              `json:"label"`
	Value string              `json:"value"`
	Lines []summaryMetricLine `json:"lines,omitempty"`
}

type summaryMetricLine struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type summaryCard struct {
	Title   string          `json:"title"`
	Metrics []summaryMetric `json:"metrics"`
}

type chartPoint struct {
	Ts    string  `json:"ts"`
	Value float64 `json:"value"`
}

type chartSeries struct {
	Name   string       `json:"name"`
	Color  string       `json:"color"`
	Points []chartPoint `json:"points"`
}

type chart struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Unit        string        `json:"unit"`
	TimeFormat  string        `json:"time_format"`
	StartAtZero bool          `json:"start_at_zero"`
	Series      []chartSeries `json:"series"`
}

type chartGroup struct {
	Title  string  `json:"title"`
	Charts []chart `json:"charts"`
}

type eventRow struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	Evidence  string `json:"evidence"`
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at"`
	Duration  string `json:"duration"`
}

type causeRow struct {
	Name        string `json:"name"`
	Duration    string `json:"duration"`
	DurationSec int64  `json:"duration_sec"`
}

type eventSummary struct {
	Count   int    `json:"count"`
	Longest string `json:"longest"`
}

type reportData struct {
	GeneratedAt string        `json:"generated_at"`
	RangeLabel  string        `json:"range_label"`
	RangeStart  string        `json:"range_start"`
	RangeEnd    string        `json:"range_end"`
	Summary     []summaryCard `json:"summary"`
	Groups      []chartGroup  `json:"groups"`
	Causes      []causeRow    `json:"causes"`
	Events      []eventRow    `json:"events"`
	EventMeta   eventSummary  `json:"event_meta"`
	Codex       *codexReport  `json:"codex,omitempty"`
}

type templatePageData struct {
	LiveMode     bool
	InitialJSON  template.JS
	DefaultMode  string
	ReportScript template.JS
	CodexScript  template.JS
}

type metricAggregate struct {
	Values    []float64
	Failures  int
	Successes int
}

type chartKey struct {
	Group       string
	Layer       string
	Metric      string
	ID          string
	Title       string
	Unit        string
	Color       string
	StartAtZero bool
	Mode        string
}

type aggregate struct {
	count int
	sum   float64
	fail  int
	vals  []float64
}

type codexReport struct {
	Available    bool                 `json:"available"`
	Error        string               `json:"error,omitempty"`
	LogPath      string               `json:"log_path"`
	RangeLabel   string               `json:"range_label"`
	RangeStart   string               `json:"range_start"`
	RangeEnd     string               `json:"range_end"`
	Clamped      bool                 `json:"clamped"`
	Summary      codexSummary         `json:"summary"`
	Timeline     []codexTimelinePoint `json:"timeline"`
	Events       []codexEventRow      `json:"events"`
	NoiseSummary []codexNoiseRow      `json:"noise_summary"`
	OtherSummary []codexIssueRow      `json:"other_summary"`
}

type codexSummary struct {
	StreamRequests        int    `json:"stream_requests"`
	CompletedTurns        int    `json:"completed_turns"`
	RetryEvents           int    `json:"retry_events"`
	RetryAffectedTurns    int    `json:"retry_affected_turns"`
	RetryEventRate        string `json:"retry_event_rate"`
	RetryAffectedTurnRate string `json:"retry_affected_turn_rate"`
	MaxRetryAttempt       string `json:"max_retry_attempt"`
	ToolErrors            int    `json:"tool_errors"`
	NetworkCandidates     int    `json:"network_candidates"`
	AppsErrors            int    `json:"apps_errors"`
	RolloutErrors         int    `json:"rollout_errors"`
	UnknownEvents         int    `json:"unknown_events"`
	NoiseWarnings         int    `json:"noise_warnings"`
}

type codexTimelinePoint struct {
	Ts               string `json:"ts"`
	CompletedTurns   int    `json:"completed_turns"`
	StreamRequests   int    `json:"stream_requests"`
	StreamRetry      int    `json:"stream_retry"`
	ToolError        int    `json:"tool_error"`
	NetworkCandidate int    `json:"network_candidate"`
	AppsError        int    `json:"apps_error"`
	RolloutError     int    `json:"rollout_error"`
	Unknown          int    `json:"unknown"`
	Total            int    `json:"total"`
}

type codexEventRow struct {
	Kind         string `json:"kind"`
	KindLabel    string `json:"kind_label"`
	Level        string `json:"level"`
	Time         string `json:"time"`
	Ts           string `json:"ts"`
	Line         int    `json:"line,omitempty"`
	Model        string `json:"model,omitempty"`
	ThreadID     string `json:"thread_id,omitempty"`
	TurnID       string `json:"turn_id,omitempty"`
	Attempt      string `json:"attempt,omitempty"`
	Backoff      string `json:"backoff,omitempty"`
	TurnDuration string `json:"turn_duration,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
	ReasonTokens int    `json:"reason_tokens,omitempty"`
	Summary      string `json:"summary"`
	Evidence     string `json:"evidence"`
}

type codexNoiseRow struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type codexIssueRow struct {
	Kind   string `json:"kind"`
	Level  string `json:"level"`
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Sample string `json:"sample"`
}
