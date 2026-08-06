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
		FileName:    "2026-08-05-Maituo-客户日报.xlsx",
		ReportDate:  time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC),
		Notes:       []NoteDetail{{NoteID: "note-a", Subaccount: "账户A"}, {NoteID: "note-b", Subaccount: "账户B"}},
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
	notes, _ := workbook.GetRows(SheetNotes)
	subaccounts, _ := workbook.GetRows(SheetSubaccount)
	if len(notes) != 2 || notes[1][3] != "账户A" {
		t.Fatalf("notes = %v", notes)
	}
	if len(subaccounts) != 2 || subaccounts[1][1] != "账户A" {
		t.Fatalf("subaccounts = %v", subaccounts)
	}
}

func TestBuildSubaccountWorkbookRejectsMissingAccountData(t *testing.T) {
	_, err := BuildSubaccountWorkbook("账户A", Snapshot{Notes: []NoteDetail{{Subaccount: "账户B"}}})
	if !errors.Is(err, ErrNoSubaccountData) {
		t.Fatalf("error = %v", err)
	}
}
