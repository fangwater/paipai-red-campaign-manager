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
