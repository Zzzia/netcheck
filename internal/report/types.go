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
}

type templatePageData struct {
	LiveMode    bool
	InitialJSON template.JS
	DefaultMode string
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
