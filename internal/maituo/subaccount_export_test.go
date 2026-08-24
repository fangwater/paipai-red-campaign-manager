package maituo

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestBuildSubaccountWorkbookFiltersOtherAccounts(t *testing.T) {
	snapshot := Snapshot{
		FileName:   "2026-08-05-Maituo-客户日报.xlsx",
		ReportDate: time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC),
		Notes: []NoteDetail{
			{NoteID: "shared-note", Subaccount: "账户A", CampaignName: "计划A"},
			{NoteID: "shared-note", Subaccount: "账户B", CampaignName: "计划B"},
		},
		Subaccounts: []SubaccountOverview{{SPU: "辅酶", Subaccount: "账户A"}, {SPU: "辅酶", Subaccount: "账户B"}},
	}
	result, err := BuildSubaccountWorkbook("账户A", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if result.FileName != "2026-08-05-Maituo-客户日报-账户A.xlsx" {
		t.Fatalf("file name = %q", result.FileName)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer workbook.Close()
	if sheets := workbook.GetSheetList(); len(sheets) != 2 || sheets[0] != SheetNotes || sheets[1] != SheetSubaccount {
		t.Fatalf("sheets = %v", sheets)
	}
	notes, _ := workbook.GetRows(SheetNotes)
	if len(notes) != 2 || notes[1][0] != "shared-note" || notes[1][3] != "账户A" || notes[1][4] != "计划A" {
		t.Fatalf("notes = %v", notes)
	}
	subaccounts, _ := workbook.GetRows(SheetSubaccount)
	if len(subaccounts) != 2 || subaccounts[1][1] != "账户A" {
		t.Fatalf("subaccounts = %v", subaccounts)
	}
}

func TestBuildSubaccountWorkbookRejectsMissingAccountData(t *testing.T) {
	_, err := BuildSubaccountWorkbook("账户A", Snapshot{Subaccounts: []SubaccountOverview{{Subaccount: "账户B"}}})
	if !errors.Is(err, ErrNoSubaccountData) {
		t.Fatalf("error = %v", err)
	}
}
