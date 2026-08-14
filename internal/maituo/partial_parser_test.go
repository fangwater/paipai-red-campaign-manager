package maituo

import (
	"bytes"
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
	if err := workbook.SetSheetRow(SheetKPI, "A1", &[]interface{}{"指标", "数值", "数据口径"}); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetSheetRow(SheetKPI, "A2", &[]interface{}{"消耗(元)", 123.45, "宽表加总"}); err != nil {
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
	if len(snapshot.PresentSheets) != 1 || snapshot.PresentSheets[0] != SheetKPI {
		t.Fatalf("present sheets = %v", snapshot.PresentSheets)
	}
	missing := MissingSheets(snapshot.PresentSheets)
	if len(missing) != 4 || missing[0] != SheetNotes || missing[3] != SheetTrend {
		t.Fatalf("missing sheets = %v", missing)
	}
	if len(snapshot.KPIs) != 1 || snapshot.KPIs[0].Value != 123.45 {
		t.Fatalf("KPIs = %+v", snapshot.KPIs)
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
	row := []interface{}{"note-1", "https://example.com/note-1", "测试子账户", "测试计划", "搜索", "品牌词", 100.5, 10, 10.05, 9.5, 4.2, 0.8, 1.2}
	if err := workbook.SetSheetRow(SheetNotes, "A2", &row); err != nil {
		t.Fatal(err)
	}
	snapshot := parseTestWorkbook(t, workbook)
	if len(snapshot.Notes) != 1 {
		t.Fatalf("notes = %+v", snapshot.Notes)
	}
	note := snapshot.Notes[0]
	if note.Category != defaultNoteCategory || note.Subaccount != "测试子账户" || note.Placement != "搜索" || note.Spend != 100.5 || note.CTRPct != 1.2 {
		t.Fatalf("note = %+v", note)
	}
}

func TestParseNotesDefaultsBlankCategory(t *testing.T) {
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
	row := []interface{}{"note-1", "https://example.com/note-1", "", "测试子账户", "测试计划", "信息流", "", 20, 2, 10, "", 3.5, 0.7, 1.1}
	if err := workbook.SetSheetRow(SheetNotes, "A2", &row); err != nil {
		t.Fatal(err)
	}
	snapshot := parseTestWorkbook(t, workbook)
	if len(snapshot.Notes) != 1 || snapshot.Notes[0].Category != defaultNoteCategory {
		t.Fatalf("notes = %+v", snapshot.Notes)
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
