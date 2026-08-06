package maituo

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

var ErrNoSubaccountData = errors.New("该子账户在指定日期暂无数据")

type SubaccountWorkbook struct {
	FileName string
	Data     []byte
}

type subaccountRows struct {
	notes       []NoteDetail
	subaccounts []SubaccountOverview
}

// BuildSubaccountWorkbook filters every row again so a generated file only
// contains the requested subaccount, even if the data source returns extra rows.
func BuildSubaccountWorkbook(subaccount string, snapshot Snapshot) (SubaccountWorkbook, error) {
	subaccount = strings.TrimSpace(subaccount)
	if subaccount == "" || subaccount == "总体" {
		return SubaccountWorkbook{}, ErrNoSubaccountData
	}
	rows := &subaccountRows{}
	for _, row := range snapshot.Notes {
		if strings.TrimSpace(row.Subaccount) == subaccount {
			rows.notes = append(rows.notes, row)
		}
	}
	for _, row := range snapshot.Subaccounts {
		if strings.TrimSpace(row.Subaccount) == subaccount {
			rows.subaccounts = append(rows.subaccounts, row)
		}
	}
	if len(rows.notes) == 0 && len(rows.subaccounts) == 0 {
		return SubaccountWorkbook{}, ErrNoSubaccountData
	}
	data, err := buildSubaccountWorkbook(rows)
	if err != nil {
		return SubaccountWorkbook{}, fmt.Errorf("build subaccount workbook: %w", err)
	}
	return SubaccountWorkbook{
		FileName: exportBaseName(snapshot) + "-" + safeExportPart(subaccount) + ".xlsx",
		Data:     data,
	}, nil
}

func buildSubaccountWorkbook(rows *subaccountRows) ([]byte, error) {
	workbook := excelize.NewFile()
	defer func() { _ = workbook.Close() }()
	if err := workbook.SetSheetName("Sheet1", SheetNotes); err != nil {
		return nil, err
	}
	if _, err := workbook.NewSheet(SheetSubaccount); err != nil {
		return nil, err
	}
	noteRows := make([][]interface{}, len(rows.notes))
	for index, row := range rows.notes {
		noteRows[index] = []interface{}{row.NoteID, row.NoteURL, row.Category, row.Subaccount, row.CampaignName, row.Placement, optionalExportValue(row.KeywordCategoryNote), row.Spend, row.SearchUsers, optionalExportValue(row.SearchCost), optionalExportValue(row.EstimatedPostbackCost), optionalExportValue(row.SearchRatePct), row.CPC, row.CTRPct}
	}
	subaccountRows := make([][]interface{}, len(rows.subaccounts))
	for index, row := range rows.subaccounts {
		subaccountRows[index] = []interface{}{row.SPU, row.Subaccount, row.Placement, optionalExportValue(row.SearchCost), optionalExportValue(row.EstimatedPostbackCost), row.Spend, row.SearchUsers, optionalExportValue(row.SearchRatePct), optionalExportValue(row.CPC), optionalExportValue(row.CTRPct), row.NoteCount}
	}
	if err := writeExportSheet(workbook, SheetNotes, expectedHeaders[SheetNotes], noteRows, []float64{22, 34, 14, 18, 24, 12, 16, 14, 12, 14, 18, 14, 10, 10}); err != nil {
		return nil, err
	}
	if err := writeExportSheet(workbook, SheetSubaccount, expectedHeaders[SheetSubaccount], subaccountRows, []float64{14, 18, 12, 14, 18, 14, 12, 14, 10, 10, 10}); err != nil {
		return nil, err
	}
	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeExportSheet(workbook *excelize.File, sheet string, headers []string, rows [][]interface{}, widths []float64) error {
	headerValues := make([]interface{}, len(headers))
	for index, header := range headers {
		headerValues[index] = header
	}
	if err := workbook.SetSheetRow(sheet, "A1", &headerValues); err != nil {
		return err
	}
	for index, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := workbook.SetSheetRow(sheet, cell, &row); err != nil {
			return err
		}
	}
	headerStyle, err := workbook.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"2B6F5B"}}, Alignment: &excelize.Alignment{Vertical: "center"}})
	if err != nil {
		return err
	}
	lastColumn, _ := excelize.ColumnNumberToName(len(headers))
	if err := workbook.SetCellStyle(sheet, "A1", lastColumn+"1", headerStyle); err != nil {
		return err
	}
	if err := workbook.AutoFilter(sheet, fmt.Sprintf("A1:%s%d", lastColumn, len(rows)+1), nil); err != nil {
		return err
	}
	if err := workbook.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	for index, width := range widths {
		column, _ := excelize.ColumnNumberToName(index + 1)
		if err := workbook.SetColWidth(sheet, column, column, width); err != nil {
			return err
		}
	}
	return nil
}

func optionalExportValue[T any](value *T) interface{} {
	if value == nil {
		return ""
	}
	return *value
}

func exportBaseName(snapshot Snapshot) string {
	baseName := strings.TrimSuffix(filepath.Base(snapshot.FileName), filepath.Ext(snapshot.FileName))
	if baseName == "" || baseName == "." {
		baseName = snapshot.ReportDate.Format("2006-01-02") + "-Maituo-客户日报"
	}
	return safeExportPart(baseName)
}

func safeExportPart(value string) string {
	value = strings.Map(func(character rune) rune {
		if character < 32 || strings.ContainsRune(`<>:"/\\|?*`, character) {
			return '_'
		}
		return character
	}, strings.TrimSpace(value))
	value = strings.TrimRight(value, ". ")
	if value == "" {
		return "未命名子账户"
	}
	for utf8.RuneCountInString(value) > 80 {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}
