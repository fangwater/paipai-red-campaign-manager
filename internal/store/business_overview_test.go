package store

import "testing"

func floatPointer(value float64) *float64 {
	return &value
}

func TestOverviewMetricUsesTotalsForVolumeMetrics(t *testing.T) {
	current := []overviewTrendRow{{Date: "2026-07-01", Spend: floatPointer(10)}, {Date: "2026-07-02", Spend: nil}, {Date: "2026-07-03", Spend: floatPointer(20)}}
	previous := []overviewTrendRow{{Date: "2026-06-28", Spend: floatPointer(10)}, {Date: "2026-06-29", Spend: floatPointer(10)}, {Date: "2026-06-30", Spend: floatPointer(10)}}

	metric := overviewMetric("spend", "每日消耗", "currency", current, previous, func(row overviewTrendRow) *float64 { return row.Spend }, false)

	if metric.CurrentValue == nil || *metric.CurrentValue != 30 {
		t.Fatalf("current=%v want=30", metric.CurrentValue)
	}
	if metric.ChangePct == nil || *metric.ChangePct != 0 {
		t.Fatalf("change=%v want=0", metric.ChangePct)
	}
	if metric.Points[1].Value != nil {
		t.Fatalf("missing day value=%v want=nil", metric.Points[1].Value)
	}
}

func TestOverviewMetricUsesAvailableDayAverageForCost(t *testing.T) {
	current := []overviewTrendRow{{SearchCost: floatPointer(10)}, {SearchCost: nil}, {SearchCost: floatPointer(20)}}
	previous := []overviewTrendRow{{SearchCost: floatPointer(10)}, {SearchCost: floatPointer(10)}, {SearchCost: floatPointer(10)}}

	metric := overviewMetric("search_cost", "回搜成本", "currency", current, previous, func(row overviewTrendRow) *float64 { return row.SearchCost }, true)

	if metric.CurrentValue == nil || *metric.CurrentValue != 15 {
		t.Fatalf("current=%v want=15", metric.CurrentValue)
	}
	if metric.ChangePct == nil || *metric.ChangePct != 0.5 {
		t.Fatalf("change=%v want=0.5", metric.ChangePct)
	}
}
