package maituo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

var ErrInvalidWorkbook = errors.New("invalid Maituo customer daily workbook")

var reportDatePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

const defaultNoteCategory = "信息流"

var expectedHeaders = map[string][]string{
	SheetKPI:        {"指标", "数值", "数据口径"},
	SheetNotes:      {"笔记ID", "笔记链接", "分类", "子账户", "计划名", "场域", "词类备注", "消耗", "回搜人数", "回搜成本", "预计回流后成本", "回搜率(%)", "CPC", "CTR(%)"},
	SheetSPU:        {"SPU", "竞价消耗", "回搜", "回搜成本", "回搜率(%)", "CPC", "CTR(%)", "笔记数"},
	SheetSubaccount: {"SPU", "子账户", "场域", "回搜成本", "预计回流后成本", "消耗", "回搜", "回搜率(%)", "CPC", "CTR(%)", "笔记数"},
	SheetTrend:      {"日期", "辅酶消耗(元)", "辅酶淘搜UV", "辅酶成交UV", "辅酶淘搜成本(元/人)", "磷虾油消耗(元)", "磷虾油淘搜UV", "磷虾油成交UV", "磷虾油淘搜成本(元/人)", "合计淘搜UV", "合计成交UV", "合计淘搜成本(元/人)", "合计消耗(元)", "合计回搜成本(元/人)"},
}

var noteHeadersWithoutCategory = []string{"笔记ID", "笔记链接", "子账户", "计划名", "场域", "词类备注", "消耗", "回搜人数", "回搜成本", "预计回流后成本", "回搜率(%)", "CPC", "CTR(%)"}

func Parse(reader io.Reader, fileName string) (Snapshot, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read Maituo workbook: %w", err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return Snapshot{}, invalid("无法读取 .xlsx 文件")
	}
	defer func() { _ = workbook.Close() }()
	actualSheets := workbook.GetSheetList()
	presentSheets := make([]string, 0, len(WorkbookSheets))
	for _, name := range WorkbookSheets {
		if contains(actualSheets, name) {
			presentSheets = append(presentSheets, name)
		}
	}
	if len(presentSheets) == 0 {
		return Snapshot{}, invalid("未找到可识别的数据表，实际工作表为 %s", strings.Join(actualSheets, "、"))
	}
	sum := sha256.Sum256(data)
	snapshot := Snapshot{FileName: fileName, FileSHA256: hex.EncodeToString(sum[:]), PresentSheets: presentSheets}
	for _, sheet := range presentSheets {
		switch sheet {
		case SheetKPI:
			snapshot.KPIs, err = parseKPIs(workbook)
		case SheetNotes:
			snapshot.Notes, err = parseNotes(workbook)
		case SheetSPU:
			snapshot.SPUs, err = parseSPUs(workbook)
		case SheetSubaccount:
			snapshot.Subaccounts, err = parseSubaccounts(workbook)
		case SheetTrend:
			snapshot.Trends, err = parseTrends(workbook)
		}
		if err != nil {
			return Snapshot{}, err
		}
	}
	if snapshot.ReportDate, err = resolveReportDate(fileName, snapshot.Trends); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func resolveReportDate(fileName string, trends []SearchTrend) (time.Time, error) {
	if value := reportDatePattern.FindString(fileName); value != "" {
		date, err := time.Parse("2006-01-02", value)
		if err != nil {
			return time.Time{}, invalid("文件名中的报表日期 %q 无效", value)
		}
		return date, nil
	}
	if len(trends) == 0 {
		return time.Time{}, invalid("文件名不含报表日期，且淘搜趋势没有可用日期")
	}
	latest := trends[0].Date
	for _, trend := range trends[1:] {
		if trend.Date.After(latest) {
			latest = trend.Date
		}
	}
	return latest, nil
}

func sheetRows(workbook *excelize.File, sheet string) ([][]string, error) {
	rows, err := workbook.GetRows(sheet)
	if err != nil {
		return nil, invalid("读取工作表 %q 失败", sheet)
	}
	if len(rows) == 0 {
		return nil, invalid("工作表 %q 为空", sheet)
	}
	if sheet == SheetNotes && headersMatch(rows[0], noteHeadersWithoutCategory) {
		return normalizeNoteRowsWithoutCategory(rows[1:]), nil
	}
	expected := expectedHeaders[sheet]
	if len(rows[0]) != len(expected) {
		return nil, invalid("工作表 %q 表头应有 %d 列，实际为 %d", sheet, len(expected), len(rows[0]))
	}
	for column, header := range expected {
		if strings.TrimSpace(rows[0][column]) != header {
			return nil, invalid("工作表 %q 第 %d 列应为 %q，实际为 %q", sheet, column+1, header, rows[0][column])
		}
	}
	return rows[1:], nil
}

func headersMatch(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index, header := range expected {
		if strings.TrimSpace(actual[index]) != header {
			return false
		}
	}
	return true
}

func normalizeNoteRowsWithoutCategory(rows [][]string) [][]string {
	result := make([][]string, len(rows))
	for index, row := range rows {
		if rowBlank(row) {
			result[index] = row
			continue
		}
		length := len(row) + 1
		if length < 3 {
			length = 3
		}
		normalized := make([]string, length)
		prefixLength := len(row)
		if prefixLength > 2 {
			prefixLength = 2
		}
		copy(normalized[:prefixLength], row[:prefixLength])
		normalized[2] = defaultNoteCategory
		if len(row) > 2 {
			copy(normalized[3:], row[2:])
		}
		result[index] = normalized
	}
	return result
}

func parseKPIs(workbook *excelize.File) ([]KPI, error) {
	rows, err := sheetRows(workbook, SheetKPI)
	if err != nil {
		return nil, err
	}
	result := make([]KPI, 0, len(rows))
	seen := map[string]int{}
	for index, row := range rows {
		if rowBlank(row) {
			continue
		}
		n := index + 2
		item := KPI{Metric: required(row, 0), DataBasis: required(row, 2), RowMetadata: RowMetadata{SourceRow: n}}
		if item.Metric == "" || item.DataBasis == "" {
			return nil, invalid("工作表 %q 第 %d 行必填字段为空", SheetKPI, n)
		}
		if item.Value, err = requiredFloat(row, 1, SheetKPI, n); err != nil {
			return nil, err
		}
		if first, ok := seen[item.Metric]; ok {
			return nil, invalid("工作表 %q 指标 %q 在第 %d、%d 行重复", SheetKPI, item.Metric, first, n)
		}
		seen[item.Metric] = n
		item.ContentHash = hash(item)
		result = append(result, item)
	}
	return result, nil
}

func parseNotes(workbook *excelize.File) ([]NoteDetail, error) {
	rows, err := sheetRows(workbook, SheetNotes)
	if err != nil {
		return nil, err
	}
	result := make([]NoteDetail, 0, len(rows))
	seen := map[string]int{}
	for index, row := range rows {
		if rowBlank(row) {
			continue
		}
		n := index + 2
		item := NoteDetail{NoteID: required(row, 0), NoteURL: required(row, 1), Category: required(row, 2), Subaccount: required(row, 3), CampaignName: required(row, 4), Placement: required(row, 5), KeywordCategoryNote: optionalString(row, 6), RowMetadata: RowMetadata{SourceRow: n}}
		if item.Category == "" {
			item.Category = defaultNoteCategory
		}
		if item.NoteID == "" || item.NoteURL == "" || item.Subaccount == "" || item.CampaignName == "" || item.Placement == "" {
			return nil, invalid("工作表 %q 第 %d 行关键字段为空", SheetNotes, n)
		}
		if item.Spend, err = requiredFloat(row, 7, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.SearchUsers, err = requiredInt(row, 8, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.SearchCost, err = optionalFloat(row, 9, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.EstimatedPostbackCost, err = optionalFloat(row, 10, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.SearchRatePct, err = optionalFloat(row, 11, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.CPC, err = requiredFloat(row, 12, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.CTRPct, err = requiredFloat(row, 13, SheetNotes, n); err != nil {
			return nil, err
		}
		key := strings.Join([]string{item.NoteID, item.Subaccount, item.CampaignName, item.Placement}, "\x00")
		if first, ok := seen[key]; ok {
			return nil, invalid("工作表 %q 第 %d、%d 行业务键重复", SheetNotes, first, n)
		}
		seen[key] = n
		item.ContentHash = hash(item)
		result = append(result, item)
	}
	return result, nil
}

func parseSPUs(workbook *excelize.File) ([]SPUOverview, error) {
	rows, err := sheetRows(workbook, SheetSPU)
	if err != nil {
		return nil, err
	}
	result := make([]SPUOverview, 0, len(rows))
	seen := map[string]int{}
	for index, row := range rows {
		if rowBlank(row) {
			continue
		}
		n := index + 2
		item := SPUOverview{SPU: required(row, 0), RowMetadata: RowMetadata{SourceRow: n}}
		if item.SPU == "" {
			return nil, invalid("工作表 %q 第 %d 行 SPU 为空", SheetSPU, n)
		}
		if item.AuctionSpend, err = requiredFloat(row, 1, SheetSPU, n); err != nil {
			return nil, err
		}
		if item.SearchUsers, err = requiredInt(row, 2, SheetSPU, n); err != nil {
			return nil, err
		}
		if item.SearchCost, err = requiredFloat(row, 3, SheetSPU, n); err != nil {
			return nil, err
		}
		if item.SearchRatePct, err = requiredFloat(row, 4, SheetSPU, n); err != nil {
			return nil, err
		}
		if item.CPC, err = requiredFloat(row, 5, SheetSPU, n); err != nil {
			return nil, err
		}
		if item.CTRPct, err = requiredFloat(row, 6, SheetSPU, n); err != nil {
			return nil, err
		}
		if item.NoteCount, err = requiredInt(row, 7, SheetSPU, n); err != nil {
			return nil, err
		}
		if first, ok := seen[item.SPU]; ok {
			return nil, invalid("工作表 %q SPU %q 在第 %d、%d 行重复", SheetSPU, item.SPU, first, n)
		}
		seen[item.SPU] = n
		item.ContentHash = hash(item)
		result = append(result, item)
	}
	return result, nil
}

func parseSubaccounts(workbook *excelize.File) ([]SubaccountOverview, error) {
	rows, err := sheetRows(workbook, SheetSubaccount)
	if err != nil {
		return nil, err
	}
	result := make([]SubaccountOverview, 0, len(rows))
	seen := map[string]int{}
	for index, row := range rows {
		if rowBlank(row) {
			continue
		}
		n := index + 2
		item := SubaccountOverview{SPU: required(row, 0), Subaccount: required(row, 1), Placement: required(row, 2), RowMetadata: RowMetadata{SourceRow: n}}
		if item.SPU == "" || item.Subaccount == "" || item.Placement == "" {
			return nil, invalid("工作表 %q 第 %d 行关键字段为空", SheetSubaccount, n)
		}
		if item.SearchCost, err = optionalFloat(row, 3, SheetSubaccount, n); err != nil {
			return nil, err
		}
		if item.EstimatedPostbackCost, err = optionalFloat(row, 4, SheetSubaccount, n); err != nil {
			return nil, err
		}
		if item.Spend, err = requiredFloat(row, 5, SheetSubaccount, n); err != nil {
			return nil, err
		}
		if item.SearchUsers, err = requiredInt(row, 6, SheetSubaccount, n); err != nil {
			return nil, err
		}
		if item.SearchRatePct, err = optionalFloat(row, 7, SheetSubaccount, n); err != nil {
			return nil, err
		}
		if item.CPC, err = optionalFloat(row, 8, SheetSubaccount, n); err != nil {
			return nil, err
		}
		if item.CTRPct, err = optionalFloat(row, 9, SheetSubaccount, n); err != nil {
			return nil, err
		}
		if item.NoteCount, err = requiredInt(row, 10, SheetSubaccount, n); err != nil {
			return nil, err
		}
		key := strings.Join([]string{item.SPU, item.Subaccount, item.Placement}, "\x00")
		if first, ok := seen[key]; ok {
			return nil, invalid("工作表 %q 第 %d、%d 行业务键重复", SheetSubaccount, first, n)
		}
		seen[key] = n
		item.ContentHash = hash(item)
		result = append(result, item)
	}
	return result, nil
}

func parseTrends(workbook *excelize.File) ([]SearchTrend, error) {
	rows, err := sheetRows(workbook, SheetTrend)
	if err != nil {
		return nil, err
	}
	result := make([]SearchTrend, 0, len(rows))
	seen := map[string]int{}
	for index, row := range rows {
		if rowBlank(row) {
			continue
		}
		n := index + 2
		dateValue := required(row, 0)
		date, parseErr := time.Parse("2006-01-02", dateValue)
		if parseErr != nil {
			return nil, invalid("工作表 %q 第 %d 行日期 %q 格式错误", SheetTrend, n, dateValue)
		}
		item := SearchTrend{Date: date, RowMetadata: RowMetadata{SourceRow: n}}
		floats := []**float64{&item.CoenzymeSpend, &item.CoenzymeSearchCost, &item.KrillOilSpend, &item.KrillOilSearchCost, &item.TotalSearchCost, &item.TotalSpend, &item.TotalRecallSearchCost}
		floatColumns := []int{1, 4, 5, 8, 11, 12, 13}
		for i, column := range floatColumns {
			if *floats[i], err = optionalFloat(row, column, SheetTrend, n); err != nil {
				return nil, err
			}
		}
		ints := []**int64{&item.CoenzymeSearchUV, &item.CoenzymeOrderUV, &item.KrillOilSearchUV, &item.KrillOilOrderUV, &item.TotalSearchUV, &item.TotalOrderUV}
		intColumns := []int{2, 3, 6, 7, 9, 10}
		for i, column := range intColumns {
			if *ints[i], err = optionalInt(row, column, SheetTrend, n); err != nil {
				return nil, err
			}
		}
		if first, ok := seen[dateValue]; ok {
			return nil, invalid("工作表 %q 日期 %q 在第 %d、%d 行重复", SheetTrend, dateValue, first, n)
		}
		seen[dateValue] = n
		item.ContentHash = hash(item)
		result = append(result, item)
	}
	return result, nil
}

func required(row []string, column int) string {
	if column >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[column])
}
func optionalString(row []string, column int) *string {
	value := required(row, column)
	if value == "" {
		return nil
	}
	return &value
}
func requiredFloat(row []string, column int, sheet string, line int) (float64, error) {
	value := required(row, column)
	if value == "" {
		return 0, invalid("工作表 %q 第 %d 行第 %d 列不能为空", sheet, line, column+1)
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, invalid("工作表 %q 第 %d 行第 %d 列 %q 不是数字", sheet, line, column+1, value)
	}
	return number, nil
}
func optionalFloat(row []string, column int, sheet string, line int) (*float64, error) {
	value := required(row, column)
	if value == "" {
		return nil, nil
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, invalid("工作表 %q 第 %d 行第 %d 列 %q 不是数字", sheet, line, column+1, value)
	}
	return &number, nil
}
func requiredInt(row []string, column int, sheet string, line int) (int64, error) {
	value, err := optionalInt(row, column, sheet, line)
	if err != nil {
		return 0, err
	}
	if value == nil {
		return 0, invalid("工作表 %q 第 %d 行第 %d 列不能为空", sheet, line, column+1)
	}
	return *value, nil
}
func optionalInt(row []string, column int, sheet string, line int) (*int64, error) {
	value := required(row, column)
	if value == "" {
		return nil, nil
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.Trunc(number) != number {
		return nil, invalid("工作表 %q 第 %d 行第 %d 列 %q 不是整数", sheet, line, column+1, value)
	}
	result := int64(number)
	return &result, nil
}
func rowBlank(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
func hash(value interface{}) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func invalid(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrInvalidWorkbook, fmt.Sprintf(format, args...))
}
