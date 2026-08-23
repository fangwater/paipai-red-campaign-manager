package maituo

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestBuildProviderWorkbookContainsOnlyNoteSheet(t *testing.T) {
	result, err := BuildProviderWorkbook("曼杰", Snapshot{
		FileName:   "2026-08-23-Maituo-客户日报.xlsx",
		ReportDate: time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC),
		Notes: []NoteDetail{{
			NoteID: "68a123456789abcdef123456", NoteURL: "https://example.com/note",
			Category: "信息流", Placement: "搜索", Spend: 120.5, SearchUsers: 4,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FileName != "2026-08-23-Maituo-客户日报-曼杰.xlsx" {
		t.Fatalf("file name = %q", result.FileName)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer workbook.Close()
	if sheets := workbook.GetSheetList(); len(sheets) != 1 || sheets[0] != SheetNotes {
		t.Fatalf("sheets = %v", sheets)
	}
	rows, err := workbook.GetRows(SheetNotes)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][0] != "68a123456789abcdef123456" || rows[1][3] != "搜索" {
		t.Fatalf("rows = %v", rows)
	}
}

func TestBuildProviderWorkbookRejectsEmptyNotes(t *testing.T) {
	_, err := BuildProviderWorkbook("曼杰", Snapshot{})
	if !errors.Is(err, ErrNoProviderData) {
		t.Fatalf("error = %v", err)
	}
}
