package coenzyme

import (
	"errors"
	"testing"
)

func TestParseDailyValuesSkipsSummaryRowsAndPreservesNulls(t *testing.T) {
	rows := [][]interface{}{
		{"时间", "消耗", "展现量", "点击量", "CTR", "CPC", "CPM", "全成交GMV", "全店ROI", "退后GMV", "退后ROI", "辅酶成交GMV", "辅酶成交ROI", "当日GMV", "当日ROI ", "搜索消耗", "搜GMV", "搜ROI", "搜索投放占比"},
		{"TTL", 30.0},
		{"7月", 10.0},
		{"时间", "消费", "展现量"},
		{"2026/7/7", 250.76, 6044.0, 542.0, 0.0897, 0.46, 41.49, 0.0, 0.0, nil, nil, 0.0, 0.0, nil, nil, nil, nil, nil, nil},
		{"2026/8/6", 7679.53, 32915.0, 5613.0, "17.05%", 1.37, 233.31, 19959.69, 2.6, 17628.79, 2.2955, 17821.04, 2.3205, 15024.3, 1.96, 7067.07, 16679.17, 2.1718, 0.9202},
	}

	records, err := ParseDailyValues(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].ReportDate.Format("2006-01-02") != "2026-07-07" || records[1].ReportDate.Format("2006-01-02") != "2026-08-06" {
		t.Fatalf("records = %+v", records)
	}
	if records[0].PostRefundGMV != nil || records[0].SameDayGMV != nil || records[0].SearchSpend != nil {
		t.Fatalf("null values were not preserved: %+v", records[0])
	}
	if records[1].CTR == nil || *records[1].CTR != 0.1705 {
		t.Fatalf("percentage CTR = %v", records[1].CTR)
	}
	if records[0].ContentHash == "" || records[0].ContentHash == records[1].ContentHash {
		t.Fatalf("content hashes = %q / %q", records[0].ContentHash, records[1].ContentHash)
	}
}

func TestParseDailyValuesRejectsDuplicateDates(t *testing.T) {
	header := []interface{}{"时间", "消耗", "展现量", "点击量", "CTR", "CPC", "CPM", "全成交GMV", "全店ROI", "退后GMV", "退后ROI", "辅酶成交GMV", "辅酶成交ROI", "当日GMV", "当日ROI", "搜索消耗", "搜GMV", "搜ROI", "搜索投放占比"}
	_, err := ParseDailyValues([][]interface{}{header, {"2026/8/1"}, {"2026-08-01"}})
	if !errors.Is(err, ErrInvalidDailySheet) {
		t.Fatalf("error = %v", err)
	}
}

func TestParseDailyValuesRejectsMissingColumns(t *testing.T) {
	_, err := ParseDailyValues([][]interface{}{{"时间", "消耗"}, {"2026/8/1", 1.0}})
	if !errors.Is(err, ErrInvalidDailySheet) {
		t.Fatalf("error = %v", err)
	}
}
