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
		Notes:       []NoteDetail{{NoteID: "note-a"}, {NoteID: "note-b"}},
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
	if sheets := workbook.GetSheetList(); len(sheets) != 1 || sheets[0] != SheetSubaccount {
		t.Fatalf("sheets = %v", sheets)
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
