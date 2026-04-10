package model

import "time"

type Sample struct {
	ID        int64
	Timestamp time.Time
	Layer     string
	Metric    string
	Target    string
	Success   bool
	LatencyMs float64
	Value     float64
	BytesRX   int64
	Detail    string
	ErrorText string
}

type Event struct {
	ID          int64
	Name        string
	Status      string
	Summary     string
	Evidence    string
	StartedAt   time.Time
	EndedAt     *time.Time
	DurationSec int64
	IsOpen      bool
}

type LayerSnapshot struct {
	Layer         string
	Degraded      bool
	Summary       string
	Evidence      string
	LastEvaluated time.Time
}
