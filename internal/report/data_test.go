package report

import (
	"testing"
	"time"

	"netcheck/internal/model"
)

func TestBuildSummaryCardsUsesLowerTailForDownload(t *testing.T) {
	samples := []model.Sample{
		{Layer: "domestic", Metric: "tcp_connect", Success: true, LatencyMs: 30},
		{Layer: "domestic", Metric: "tcp_connect", Success: true, LatencyMs: 40},
		{Layer: "domestic", Metric: "download", Success: true, Value: 12},
		{Layer: "domestic", Metric: "download", Success: true, Value: 48},
		{Layer: "domestic", Metric: "download", Success: true, Value: 96},
	}

	cards := buildSummaryCards(samples)
	if len(cards) < 2 {
		t.Fatalf("摘要卡片数量不足: %d", len(cards))
	}
	if got := cards[1].Metrics[1].Label; got != "下载" {
		t.Fatalf("期望第二个指标为下载，实际为 %s", got)
	}
	if len(cards[1].Metrics[1].Lines) != 2 {
		t.Fatalf("期望下载指标包含两行，实际为 %d", len(cards[1].Metrics[1].Lines))
	}
	if got := cards[1].Metrics[1].Lines[1].Label; got != "P5" {
		t.Fatalf("期望下载低位标签为 P5，实际为 %s", got)
	}
	if got := cards[1].Metrics[1].Lines[1].Value; got != "12.00 Mbps" {
		t.Fatalf("期望下载低位值为最低位分位，实际为 %s", got)
	}
}

func TestBuildReportDataDoesNotBlockOnCodexLog(t *testing.T) {
	now := time.Date(2026, 5, 6, 15, 0, 0, 0, time.Local)
	payload := buildReportData(nil, nil, now.Add(-time.Hour), now)
	if payload.Codex != nil {
		t.Fatal("动态网络报表不应同步附带 Codex 日志统计")
	}
}

func TestBuildEventRowsKeepsLatestTen(t *testing.T) {
	base := time.Date(2026, 4, 10, 10, 0, 0, 0, time.Local)
	var events []model.Event
	for index := 0; index < 12; index++ {
		startedAt := base.Add(time.Duration(index) * time.Minute)
		endedAt := startedAt.Add(2 * time.Minute)
		events = append(events, model.Event{
			Name:      "local",
			Status:    "degraded",
			Summary:   "summary",
			Evidence:  "evidence",
			StartedAt: startedAt,
			EndedAt:   &endedAt,
		})
	}

	rows := buildEventRows(events, base, base.Add(30*time.Minute))
	if len(rows) != 10 {
		t.Fatalf("期望仅保留最近 10 条事件，实际为 %d", len(rows))
	}
	if rows[0].StartedAt != base.Add(11*time.Minute).Format("2006-01-02 15:04:05") {
		t.Fatalf("期望第一条为最近事件，实际为 %s", rows[0].StartedAt)
	}
	if rows[9].StartedAt != base.Add(2*time.Minute).Format("2006-01-02 15:04:05") {
		t.Fatalf("期望第十条为倒数第十个事件，实际为 %s", rows[9].StartedAt)
	}
}

func TestFormatPercentKeepsSmallNonZeroLossVisible(t *testing.T) {
	if got := formatPercent(0.000293); got != "0.029%" {
		t.Fatalf("期望小丢包率保留可见精度，实际为 %s", got)
	}
	if got := formatPercent(0); got != "0%" {
		t.Fatalf("期望零丢包率显示为 0%%，实际为 %s", got)
	}
}

func TestBuildCauseRowsOnlyCountsThreeLinks(t *testing.T) {
	base := time.Date(2026, 4, 10, 10, 0, 0, 0, time.Local)
	endLocal := base.Add(10 * time.Minute)
	endDomestic := base.Add(5 * time.Minute)
	rows := buildCauseRows([]model.Event{
		{Name: "local", Status: "degraded", StartedAt: base, EndedAt: &endLocal},
		{Name: "domestic", Status: "degraded", StartedAt: base, EndedAt: &endDomestic},
		{Name: "root_cause", Status: "global", StartedAt: base, EndedAt: &endLocal},
	}, base, base.Add(30*time.Minute))
	if len(rows) != 2 {
		t.Fatalf("期望只统计三个链路事件，实际为 %d", len(rows))
	}
	if rows[0].Name != "本地链路" || rows[1].Name != "国内链路" {
		t.Fatalf("链路异常统计名称不符合预期: %#v", rows)
	}
}

func TestChooseChartBucketByRange(t *testing.T) {
	tests := []struct {
		name       string
		span       time.Duration
		duration   time.Duration
		timeFormat string
	}{
		{name: "short-range", span: time.Minute, duration: time.Second, timeFormat: "second"},
		{name: "mid-range", span: time.Hour, duration: time.Minute, timeFormat: "minute"},
		{name: "long-range", span: 24 * time.Hour, duration: time.Hour, timeFormat: "hour"},
	}

	for _, test := range tests {
		bucket := chooseChartBucket(test.span)
		if bucket.Duration != test.duration || bucket.TimeFormat != test.timeFormat {
			t.Fatalf("%s 桶选择不符合预期: %#v", test.name, bucket)
		}
	}
}

func TestBuildSummaryCardsShowsRemoteFailureRatio(t *testing.T) {
	samples := []model.Sample{
		{Layer: "international", Metric: "tcp_connect", Success: true, LatencyMs: 120},
		{Layer: "international", Metric: "tcp_connect", Success: false},
		{Layer: "international", Metric: "download", Success: true, Value: 20},
	}

	cards := buildSummaryCards(samples)
	if len(cards) < 3 {
		t.Fatalf("摘要卡片数量不足: %d", len(cards))
	}
	lastMetric := cards[2].Metrics[len(cards[2].Metrics)-1]
	if lastMetric.Label != "失败率" {
		t.Fatalf("期望最后一个指标为失败率，实际为 %s", lastMetric.Label)
	}
	if lastMetric.Value != "50.0%" {
		t.Fatalf("期望国外失败率为 50.0%%，实际为 %s", lastMetric.Value)
	}
}

func TestBuildSummaryCardsMergesAverageAndP95(t *testing.T) {
	samples := []model.Sample{
		{Layer: "local", Metric: "ping", Success: true, LatencyMs: 10},
		{Layer: "local", Metric: "ping", Success: true, LatencyMs: 40},
	}
	cards := buildSummaryCards(samples)
	if len(cards) == 0 || len(cards[0].Metrics) == 0 {
		t.Fatal("期望存在网关摘要卡")
	}
	if cards[0].Metrics[0].Label != "RTT" {
		t.Fatalf("期望第一个网关指标为 RTT，实际为 %s", cards[0].Metrics[0].Label)
	}
	if len(cards[0].Metrics[0].Lines) != 2 {
		t.Fatalf("期望 RTT 指标合并平均和 P95 两行，实际为 %d", len(cards[0].Metrics[0].Lines))
	}
}

func TestBuildSeriesPointsUsesZeroToOneForLoss(t *testing.T) {
	points := buildSeriesPoints(map[time.Time]*aggregate{
		time.Date(2026, 4, 10, 10, 0, 0, 0, time.Local): {count: 5, fail: 2},
	}, "loss")
	if len(points) != 1 {
		t.Fatalf("期望仅生成 1 个点，实际为 %d", len(points))
	}
	if points[0].Value != 0.4 {
		t.Fatalf("期望失败率图使用 0~1 比例值，实际为 %v", points[0].Value)
	}
}

func TestBuildChartGroupsStartsLatencyAtZero(t *testing.T) {
	groups := buildChartGroups(nil, time.Now().Add(-time.Hour), time.Now())
	for _, group := range groups {
		for _, chart := range group.Charts {
			if chart.Unit == "ms" && !chart.StartAtZero {
				t.Fatalf("期望延迟图从 0 开始，实际图表 %s 未开启", chart.Title)
			}
		}
	}
}
