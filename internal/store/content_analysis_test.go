package store

import (
	"strings"
	"testing"

	"paipai-red-campaign-manager/internal/model"
)

func TestBuildContentAnalysisUsesEligibleCostAsBoomDenominator(t *testing.T) {
	cost10, cost30, roi15, roi05 := 10.0, 30.0, 1.5, 0.5
	result := model.ContentAnalysis{Dimension: "audience", Types: []string{}, Dimensions: []string{}, Cells: []model.ContentAnalysisCell{}}
	notes := []model.ContentAnalysisNote{
		{NoteID: "1", ContentType: "科普", Audience: "职场人", Scenario: "精力疲惫", DandelionCost: &cost10, Boom: true, FlowEvaluated: true, FlowQualified: true, ROI: &roi15, ROIQualified: true, AllQualified: true},
		{NoteID: "2", ContentType: "科普", Audience: "职场人", Scenario: "熬夜心悸", DandelionCost: &cost30, FlowEvaluated: true, ROI: &roi05},
		{NoteID: "3", ContentType: "科普", Audience: "职场人", Scenario: "熬夜心悸"},
		{NoteID: "4", ContentType: contentAnalysisUnlabeled, Audience: contentAnalysisUnlabeled, Scenario: contentAnalysisUnlabeled},
	}

	buildContentAnalysis(&result, notes)

	if result.Coverage.TotalNotes != 4 || result.Coverage.DandelionCostNotes != 2 || result.Coverage.AllMetricsNotes != 2 {
		t.Fatalf("coverage=%+v", result.Coverage)
	}
	var target *model.ContentAnalysisCell
	for index := range result.Cells {
		if result.Cells[index].ContentType == "科普" && result.Cells[index].Dimension == "职场人" {
			target = &result.Cells[index]
		}
	}
	if target == nil {
		t.Fatal("missing 科普 x 职场人 cell")
	}
	if target.TotalNotes != 3 || target.DandelionEligible != 2 || target.BoomCount != 1 || target.BoomRate == nil || *target.BoomRate != 0.5 {
		t.Fatalf("cell=%+v", *target)
	}
	if target.FlowQualified != 1 || target.ROIQualified != 1 || target.AllQualified != 1 {
		t.Fatalf("qualification counts=%+v", *target)
	}
}

func TestNormalizeContentAnalysisLabel(t *testing.T) {
	tests := map[string]string{
		"audience:中老年人":   "中老年",
		"audience:考公考研人":  "考公考研",
		"audience:备孕女性":   "备孕女生",
		"scenario:还原vs氧化": "还原VS氧化",
		"scenario:健身":     "运动恢复",
		"type:":           contentAnalysisUnlabeled,
	}
	for input, want := range tests {
		parts := strings.SplitN(input, ":", 2)
		if got := normalizeContentAnalysisLabel(parts[0], parts[1]); got != want {
			t.Errorf("%s=%q want=%q", input, got, want)
		}
	}
}
