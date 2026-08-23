package maituo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

var ErrNoProviderData = errors.New("该服务商在指定日期暂无笔记数据")

type ProviderWorkbook struct {
	FileName string
	Data     []byte
}

// BuildProviderWorkbook emits the current note-grained report contract. The
// store has already selected notes belonging to the requested provider.
func BuildProviderWorkbook(providerName string, snapshot Snapshot) (ProviderWorkbook, error) {
	providerName = strings.TrimSpace(providerName)
	if providerName == "" || len(snapshot.Notes) == 0 {
		return ProviderWorkbook{}, ErrNoProviderData
	}

	workbook := excelize.NewFile()
	defer func() { _ = workbook.Close() }()
	if err := workbook.SetSheetName("Sheet1", SheetNotes); err != nil {
		return ProviderWorkbook{}, fmt.Errorf("name provider workbook sheet: %w", err)
	}
	rows := make([][]interface{}, len(snapshot.Notes))
	for index, row := range snapshot.Notes {
		rows[index] = []interface{}{
			row.NoteID, row.NoteURL, row.Category, row.Placement,
			optionalExportValue(row.KeywordCategoryNote), row.Spend, row.SearchUsers,
			optionalExportValue(row.SearchCost), optionalExportValue(row.EstimatedPostbackCost),
			optionalExportValue(row.SearchRatePct), row.CPC, row.CTRPct,
		}
	}
	if err := writeExportSheet(
		workbook, SheetNotes, expectedHeaders[SheetNotes], rows,
		[]float64{28, 44, 14, 12, 18, 14, 12, 14, 18, 14, 10, 10},
	); err != nil {
		return ProviderWorkbook{}, fmt.Errorf("build provider workbook: %w", err)
	}
	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		return ProviderWorkbook{}, fmt.Errorf("serialize provider workbook: %w", err)
	}
	return ProviderWorkbook{
		FileName: exportBaseName(snapshot) + "-" + safeExportPart(providerName) + ".xlsx",
		Data:     buffer.Bytes(),
	}, nil
}
