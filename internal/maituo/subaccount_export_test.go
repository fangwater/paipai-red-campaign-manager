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
			{NoteID: "multi-account-note", Subaccount: "账户B、账户A，账户C", CampaignName: "共投计划"},
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
	if len(notes) != 3 || notes[1][0] != "shared-note" || notes[1][3] != "账户A" || notes[1][4] != "计划A" || notes[2][0] != "multi-account-note" || notes[2][3] != "账户A" || notes[2][4] != "共投计划" {
		t.Fatalf("notes = %v", notes)
	}
	subaccounts, _ := workbook.GetRows(SheetSubaccount)
	if len(subaccounts) != 2 || subaccounts[1][1] != "账户A" {
		t.Fatalf("subaccounts = %v", subaccounts)
	}
}

func TestNoteBelongsToSubaccountMatchesDelimitedMembers(t *testing.T) {
	for _, test := range []struct {
		source     string
		subaccount string
		want       bool
	}{
		{source: "账户A", subaccount: "账户A", want: true},
		{source: "账户B、 账户A，账户C;账户D；账户E", subaccount: "账户A", want: true},
		{source: "账户B、账户C", subaccount: "账户A", want: false},
		{source: "账户A-测试", subaccount: "账户A", want: false},
	} {
		if got := noteBelongsToSubaccount(test.source, test.subaccount); got != test.want {
			t.Fatalf("noteBelongsToSubaccount(%q, %q) = %t, want %t", test.source, test.subaccount, got, test.want)
		}
	}
}

func TestBuildSubaccountWorkbookRejectsMissingAccountData(t *testing.T) {
	_, err := BuildSubaccountWorkbook("账户A", Snapshot{Subaccounts: []SubaccountOverview{{Subaccount: "账户B"}}})
	if !errors.Is(err, ErrNoSubaccountData) {
		t.Fatalf("error = %v", err)
	}
}
