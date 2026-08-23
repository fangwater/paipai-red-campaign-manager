package maituo

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParsePartialWorkbook(t *testing.T) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", SheetKPI); err != nil {
		t.Fatal(err)
	}
	if _, err := workbook.NewSheet("说明"); err != nil {
		t.Fatal(err)
	}
	if _, err := workbook.NewSheet(SheetNotes); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetSheetRow(SheetKPI, "A1", &[]interface{}{"指标", "数值", "数据口径"}); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetSheetRow(SheetKPI, "A2", &[]interface{}{"消耗(元)", 123.45, "宽表加总"}); err != nil {
		t.Fatal(err)
	}
	noteHeader := make([]interface{}, len(expectedHeaders[SheetNotes]))
	for index, header := range expectedHeaders[SheetNotes] {
		noteHeader[index] = header
	}
	if err := workbook.SetSheetRow(SheetNotes, "A1", &noteHeader); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetSheetRow(SheetNotes, "A2", &[]interface{}{"note-1", "https://example.com/note-1", "信息流", "搜索", "", 123.45, 4, 30.86, 19.44, 4.2, 1.2, 3.4}); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := workbook.Write(&data); err != nil {
		t.Fatal(err)
	}
	if err := workbook.Close(); err != nil {
		t.Fatal(err)
	}

	snapshot, err := Parse(bytes.NewReader(data.Bytes()), "2026-07-01-MaiTuo.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.PresentSheets) != 2 || snapshot.PresentSheets[0] != SheetKPI || snapshot.PresentSheets[1] != SheetNotes {
		t.Fatalf("present sheets = %v", snapshot.PresentSheets)
	}
	missing := MissingSheets(snapshot.PresentSheets)
	if len(missing) != 0 {
		t.Fatalf("missing sheets = %v", missing)
	}
	if len(snapshot.KPIs) != 1 || snapshot.KPIs[0].Value != 123.45 || len(snapshot.Notes) != 1 {
		t.Fatalf("KPIs = %+v, notes = %+v", snapshot.KPIs, snapshot.Notes)
	}
}

func TestParseNotesWithoutCategoryColumn(t *testing.T) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", SheetNotes); err != nil {
		t.Fatal(err)
	}
	header := make([]interface{}, len(noteHeadersWithoutCategory))
	for index, value := range noteHeadersWithoutCategory {
		header[index] = value
	}
	if err := workbook.SetSheetRow(SheetNotes, "A1", &header); err != nil {
		t.Fatal(err)
	}
	row := []interface{}{"note-1", "https://example.com/note-1", "搜索", "品牌词", 100.5, 10, 10.05, 9.5, 4.2, 0.8, 1.2}
	if err := workbook.SetSheetRow(SheetNotes, "A2", &row); err != nil {
		t.Fatal(err)
	}
	snapshot := parseTestWorkbook(t, workbook)
	if len(snapshot.Notes) != 1 {
		t.Fatalf("notes = %+v", snapshot.Notes)
	}
	note := snapshot.Notes[0]
	if note.Category != defaultNoteCategory || note.Placement != "搜索" || note.Spend != 100.5 || note.CTRPct != 1.2 {
		t.Fatalf("note = %+v", note)
	}
	if note.EstimatedPostbackCost == nil || *note.EstimatedPostbackCost != 6.33 {
		t.Fatalf("estimated postback cost = %v, want 6.33", note.EstimatedPostbackCost)
	}
}

func TestParseLegacyNotesWithoutCategoryColumn(t *testing.T) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", SheetNotes); err != nil {
		t.Fatal(err)
	}
	header := make([]interface{}, len(legacyNoteHeadersWithoutCategory))
	for index, value := range legacyNoteHeadersWithoutCategory {
		header[index] = value
	}
	if err := workbook.SetSheetRow(SheetNotes, "A1", &header); err != nil {
		t.Fatal(err)
	}
	row := []interface{}{"note-1", "https://example.com/note-1", "测试子账户", "测试计划", "搜索", "品牌词", 100, 10, 10, 6.3, 10, 1, 10}
	if err := workbook.SetSheetRow(SheetNotes, "A2", &row); err != nil {
		t.Fatal(err)
	}
	snapshot := parseTestWorkbook(t, workbook)
	if len(snapshot.Notes) != 1 || snapshot.Notes[0].Category != defaultNoteCategory || snapshot.Notes[0].Placement != "搜索" {
		t.Fatalf("notes = %+v", snapshot.Notes)
	}
}

func TestParseNotesMergesSubaccountsAndCampaigns(t *testing.T) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", SheetNotes); err != nil {
		t.Fatal(err)
	}
	header := make([]interface{}, len(legacyNoteHeaders))
	for index, value := range legacyNoteHeaders {
		header[index] = value
	}
	if err := workbook.SetSheetRow(SheetNotes, "A1", &header); err != nil {
		t.Fatal(err)
	}
	rows := [][]interface{}{
		{"note-1", "https://example.com/note-1", "测评", "账户 A", "计划 A", "搜索", "品类词", 100, 4, 25, 15.75, 8, 2, 10},
		{"note-1", "https://example.com/note-1", "测评", "账户 B", "计划 B", "搜索", "品类词", 50, 1, 50, 31.5, 5, 2.5, 20},
		{"note-precision", "https://example.com/note-precision", "测评", "账户 A", "计划 A", "搜索", "品类词", 0.123456, 0, "", "", "", 0.1, 10},
		{"note-precision", "https://example.com/note-precision", "测评", "账户 B", "计划 B", "搜索", "品类词", 0.234567, 0, "", "", "", 0.1, 10},
	}
	for index, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := workbook.SetSheetRow(SheetNotes, cell, &row); err != nil {
			t.Fatal(err)
		}
	}

	snapshot := parseTestWorkbook(t, workbook)
	if len(snapshot.Notes) != 2 {
		t.Fatalf("notes = %+v", snapshot.Notes)
	}
	note := snapshot.Notes[0]
	if note.NoteID != "note-1" || note.Placement != "搜索" || note.Spend != 150 || note.SearchUsers != 5 {
		t.Fatalf("identity and totals = %+v", note)
	}
	if note.SearchCost == nil || *note.SearchCost != 30 || note.EstimatedPostbackCost == nil || *note.EstimatedPostbackCost != 18.9 {
		t.Fatalf("costs = %+v", note)
	}
	if note.SearchRatePct == nil || *note.SearchRatePct != 7.1429 || note.CPC != 2.1429 || note.CTRPct != 11.6667 {
		t.Fatalf("rates = %+v", note)
	}
	if precisionNote := snapshot.Notes[1]; math.Abs(precisionNote.Spend-0.358023) > 1e-12 {
		t.Fatalf("merged spend lost source precision: %+v", precisionNote)
	}
}

func TestParseNotesDerivesMergedPostbackCostWhenSourceValuesAreNil(t *testing.T) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", SheetNotes); err != nil {
		t.Fatal(err)
	}
	header := make([]interface{}, len(legacyNoteHeaders))
	for index, value := range legacyNoteHeaders {
		header[index] = value
	}
	if err := workbook.SetSheetRow(SheetNotes, "A1", &header); err != nil {
		t.Fatal(err)
	}
	rows := [][]interface{}{
		{"note-1", "https://example.com/note-1", "测评", "账户 A", "计划 A", "搜索", "品类词", 100, 4, 25, "", 8, 2, 10},
		{"note-1", "https://example.com/note-1", "测评", "账户 B", "计划 B", "搜索", "品类词", 50, 1, 50, "", 5, 2.5, 20},
	}
	for index, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := workbook.SetSheetRow(SheetNotes, cell, &row); err != nil {
			t.Fatal(err)
		}
	}

	snapshot := parseTestWorkbook(t, workbook)
	if len(snapshot.Notes) != 1 || snapshot.Notes[0].EstimatedPostbackCost == nil || *snapshot.Notes[0].EstimatedPostbackCost != 18.9 {
		t.Fatalf("notes = %+v", snapshot.Notes)
	}
}

func TestParseNotesUsesRoundedSearchCostForPostbackCost(t *testing.T) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", SheetNotes); err != nil {
		t.Fatal(err)
	}
	header := make([]interface{}, len(expectedHeaders[SheetNotes]))
	for index, value := range expectedHeaders[SheetNotes] {
		header[index] = value
	}
	if err := workbook.SetSheetRow(SheetNotes, "A1", &header); err != nil {
		t.Fatal(err)
	}
	rows := [][]interface{}{
		{"note-1", "https://example.com/note-1", "测评", "搜索", "品类词", 7.499, 1, 7.499, "", 8, 1, 10},
		{"note-1", "https://example.com/note-1", "测评", "搜索", "品类词", 7.499, 1, 7.499, "", 8, 1, 10},
	}
	for index, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := workbook.SetSheetRow(SheetNotes, cell, &row); err != nil {
			t.Fatal(err)
		}
	}

	note := parseTestWorkbook(t, workbook).Notes[0]
	if note.SearchCost == nil || *note.SearchCost != 7.5 || note.EstimatedPostbackCost == nil || *note.EstimatedPostbackCost != 4.73 {
		t.Fatalf("costs = %+v, want search cost 7.50 and estimated postback cost 4.73", note)
	}
}

func TestEstimatedPostbackCostUsesDecimalRounding(t *testing.T) {
	tests := []struct {
		searchCost float64
		want       float64
	}{
		{searchCost: 1.005, want: 0.64},
		{searchCost: 10.075, want: 6.35},
		{searchCost: -1.005, want: -0.64},
	}
	for _, test := range tests {
		cost := estimatedPostbackCost(&test.searchCost)
		if cost == nil || *cost != test.want {
			t.Errorf("estimatedPostbackCost(%v) = %v, want %v", test.searchCost, cost, test.want)
		}
	}
}

func TestParseCanonicalNotesDefaultsBlankCategory(t *testing.T) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", SheetNotes); err != nil {
		t.Fatal(err)
	}
	header := make([]interface{}, len(expectedHeaders[SheetNotes]))
	for index, value := range expectedHeaders[SheetNotes] {
		header[index] = value
	}
	if err := workbook.SetSheetRow(SheetNotes, "A1", &header); err != nil {
		t.Fatal(err)
	}
	row := []interface{}{"note-1", "https://example.com/note-1", "", "信息流", "", 20, 2, 10, "", 3.5, 0.7, 1.1}
	if err := workbook.SetSheetRow(SheetNotes, "A2", &row); err != nil {
		t.Fatal(err)
	}
	snapshot := parseTestWorkbook(t, workbook)
	if len(snapshot.Notes) != 1 || snapshot.Notes[0].Category != defaultNoteCategory {
		t.Fatalf("notes = %+v", snapshot.Notes)
	}
	note := snapshot.Notes[0]
	if note.SearchCost == nil || *note.SearchCost != 10 || note.EstimatedPostbackCost == nil || *note.EstimatedPostbackCost != 6.3 || note.SearchRatePct == nil || *note.SearchRatePct != 3.5 || note.CPC != 0.7 || note.CTRPct != 1.1 {
		t.Fatalf("single component metrics were changed: %+v", note)
	}
}

func TestParseSubaccountsDerivesPostbackCostFromSearchCost(t *testing.T) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", SheetSubaccount); err != nil {
		t.Fatal(err)
	}
	header := make([]interface{}, len(expectedHeaders[SheetSubaccount]))
	for index, value := range expectedHeaders[SheetSubaccount] {
		header[index] = value
	}
	if err := workbook.SetSheetRow(SheetSubaccount, "A1", &header); err != nil {
		t.Fatal(err)
	}
	rows := [][]interface{}{
		{"辅酶", "账户 A", "搜索", 10.05, 999, 100.5, 10, 4.2, 0.8, 1.2, 2},
		{"辅酶", "账户 A", "信息流", "", 999, 20, 0, "", "", "", 1},
	}
	for index, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := workbook.SetSheetRow(SheetSubaccount, cell, &row); err != nil {
			t.Fatal(err)
		}
	}

	snapshot := parseTestWorkbook(t, workbook)
	if len(snapshot.Subaccounts) != 2 {
		t.Fatalf("subaccounts = %+v", snapshot.Subaccounts)
	}
	if cost := snapshot.Subaccounts[0].EstimatedPostbackCost; cost == nil || *cost != 6.33 {
		t.Fatalf("estimated postback cost = %v, want 6.33", cost)
	}
	if snapshot.Subaccounts[1].EstimatedPostbackCost != nil {
		t.Fatalf("estimated postback cost without search cost = %v, want nil", snapshot.Subaccounts[1].EstimatedPostbackCost)
	}
}

func TestParseNotesRejectsConflictingCanonicalMetadata(t *testing.T) {
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", SheetNotes); err != nil {
		t.Fatal(err)
	}
	header := make([]interface{}, len(expectedHeaders[SheetNotes]))
	for index, value := range expectedHeaders[SheetNotes] {
		header[index] = value
	}
	if err := workbook.SetSheetRow(SheetNotes, "A1", &header); err != nil {
		t.Fatal(err)
	}
	rows := [][]interface{}{
		{"note-1", "https://example.com/note-1", "测评", "搜索", "品类词", 100, 4, 25, 15.75, 8, 2, 10},
		{"note-1", "https://example.com/other", "测评", "搜索", "品类词", 50, 1, 50, 31.5, 5, 2.5, 20},
	}
	for index, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := workbook.SetSheetRow(SheetNotes, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	var data bytes.Buffer
	if err := workbook.Write(&data); err != nil {
		t.Fatal(err)
	}
	if err := workbook.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := Parse(bytes.NewReader(data.Bytes()), "2026-08-13-MaiTuo-客户日报.xlsx")
	if !errors.Is(err, ErrInvalidWorkbook) || !strings.Contains(err.Error(), "基础信息") {
		t.Fatalf("error = %v", err)
	}
}

func parseTestWorkbook(t *testing.T, workbook *excelize.File) Snapshot {
	t.Helper()
	var data bytes.Buffer
	if err := workbook.Write(&data); err != nil {
		t.Fatal(err)
	}
	if err := workbook.Close(); err != nil {
		t.Fatal(err)
	}
	snapshot, err := Parse(bytes.NewReader(data.Bytes()), "2026-08-13-MaiTuo-客户日报.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
