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
	"math/big"
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
	SheetNotes:      {"笔记ID", "笔记链接", "分类", "场域", "词类备注", "消耗", "回搜人数", "回搜成本", "预计回流后成本", "回搜率(%)", "CPC", "CTR(%)"},
	SheetSPU:        {"SPU", "竞价消耗", "回搜", "回搜成本", "回搜率(%)", "CPC", "CTR(%)", "笔记数"},
	SheetSubaccount: {"SPU", "子账户", "场域", "回搜成本", "预计回流后成本", "消耗", "回搜", "回搜率(%)", "CPC", "CTR(%)", "笔记数"},
	SheetTrend:      {"日期", "辅酶消耗(元)", "辅酶淘搜UV", "辅酶成交UV", "辅酶淘搜成本(元/人)", "磷虾油消耗(元)", "磷虾油淘搜UV", "磷虾油成交UV", "磷虾油淘搜成本(元/人)", "合计淘搜UV", "合计成交UV", "合计淘搜成本(元/人)", "合计消耗(元)", "合计回搜成本(元/人)"},
}

var noteHeadersWithoutCategory = []string{"笔记ID", "笔记链接", "场域", "词类备注", "消耗", "回搜人数", "回搜成本", "预计回流后成本", "回搜率(%)", "CPC", "CTR(%)"}

var legacyNoteHeaders = []string{"笔记ID", "笔记链接", "分类", "子账户", "计划名", "场域", "词类备注", "消耗", "回搜人数", "回搜成本", "预计回流后成本", "回搜率(%)", "CPC", "CTR(%)"}

var legacyNoteHeadersWithoutCategory = []string{"笔记ID", "笔记链接", "子账户", "计划名", "场域", "词类备注", "消耗", "回搜人数", "回搜成本", "预计回流后成本", "回搜率(%)", "CPC", "CTR(%)"}

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
	presentSheets := make([]string, 0, len(RecognizedWorkbookSheets))
	for _, name := range RecognizedWorkbookSheets {
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
	if sheet == SheetNotes {
		switch {
		case headersMatch(rows[0], expectedHeaders[SheetNotes]):
			return rows[1:], nil
		case headersMatch(rows[0], noteHeadersWithoutCategory):
			return normalizeNoteRowsWithoutCategory(rows[1:]), nil
		case headersMatch(rows[0], legacyNoteHeaders):
			return normalizeLegacyNoteRows(rows[1:], true), nil
		case headersMatch(rows[0], legacyNoteHeadersWithoutCategory):
			return normalizeLegacyNoteRows(rows[1:], false), nil
		}
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
		normalized := make([]string, len(expectedHeaders[SheetNotes]))
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

func normalizeLegacyNoteRows(rows [][]string, hasCategory bool) [][]string {
	result := make([][]string, len(rows))
	for index, row := range rows {
		if rowBlank(row) {
			result[index] = row
			continue
		}
		normalized := make([]string, len(expectedHeaders[SheetNotes]))
		if len(row) > 0 {
			normalized[0] = row[0]
		}
		if len(row) > 1 {
			normalized[1] = row[1]
		}
		legacyMetricStart := 4
		if hasCategory {
			if len(row) > 2 {
				normalized[2] = row[2]
			}
			legacyMetricStart = 5
		} else {
			normalized[2] = defaultNoteCategory
		}
		if len(row) > legacyMetricStart {
			copy(normalized[3:], row[legacyMetricStart:])
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
	type noteAggregate struct {
		item           NoteDetail
		componentCount int
		clicks         float64
		impressions    float64
	}
	aggregates := make(map[string]*noteAggregate, len(rows))
	keys := make([]string, 0, len(rows))
	for index, row := range rows {
		if rowBlank(row) {
			continue
		}
		n := index + 2
		item := NoteDetail{NoteID: required(row, 0), NoteURL: required(row, 1), Category: required(row, 2), Placement: required(row, 3), KeywordCategoryNote: optionalString(row, 4), RowMetadata: RowMetadata{SourceRow: n}}
		if item.Category == "" {
			item.Category = defaultNoteCategory
		}
		if item.NoteID == "" || item.NoteURL == "" || item.Placement == "" {
			return nil, invalid("工作表 %q 第 %d 行关键字段为空", SheetNotes, n)
		}
		if item.Spend, err = requiredFloat(row, 5, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.SearchUsers, err = requiredInt(row, 6, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.SearchCost, err = optionalFloat(row, 7, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.EstimatedPostbackCost, err = optionalFloat(row, 8, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.SearchRatePct, err = optionalFloat(row, 9, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.CPC, err = requiredFloat(row, 10, SheetNotes, n); err != nil {
			return nil, err
		}
		if item.CTRPct, err = requiredFloat(row, 11, SheetNotes, n); err != nil {
			return nil, err
		}
		key := strings.Join([]string{item.NoteID, item.Placement}, "\x00")
		aggregate, ok := aggregates[key]
		if !ok {
			aggregate = &noteAggregate{item: item}
			aggregates[key] = aggregate
			keys = append(keys, key)
		} else if aggregate.item.NoteURL != item.NoteURL || aggregate.item.Category != item.Category || !equalOptionalString(aggregate.item.KeywordCategoryNote, item.KeywordCategoryNote) {
			return nil, invalid("工作表 %q 笔记 %q 场域 %q 的基础信息在第 %d、%d 行不一致", SheetNotes, item.NoteID, item.Placement, aggregate.item.SourceRow, n)
		} else {
			aggregate.item.Spend += item.Spend
			aggregate.item.SearchUsers += item.SearchUsers
		}
		aggregate.componentCount++
		if item.CPC > 0 {
			clicks := math.Round(item.Spend / item.CPC)
			aggregate.clicks += clicks
			if item.CTRPct > 0 {
				aggregate.impressions += math.Round(clicks / (item.CTRPct / 100))
			}
		}
	}
	result := make([]NoteDetail, 0, len(keys))
	for _, key := range keys {
		aggregate := aggregates[key]
		item := aggregate.item
		if aggregate.componentCount > 1 {
			item.SearchCost = nil
			item.EstimatedPostbackCost = nil
			item.SearchRatePct = nil
			item.CPC = 0
			item.CTRPct = 0
			if item.SearchUsers > 0 {
				searchCost := item.Spend / float64(item.SearchUsers)
				item.SearchCost = floatPointer(roundNoteMetric(searchCost, 2))
			}
			if aggregate.clicks > 0 {
				item.CPC = roundNoteMetric(item.Spend/aggregate.clicks, 4)
				if item.SearchUsers > 0 {
					item.SearchRatePct = floatPointer(roundNoteMetric(float64(item.SearchUsers)/aggregate.clicks*100, 4))
				}
			}
			if aggregate.impressions > 0 {
				item.CTRPct = roundNoteMetric(aggregate.clicks/aggregate.impressions*100, 4)
			}
		}
		item.EstimatedPostbackCost = estimatedPostbackCost(item.SearchCost)
		item.ContentHash = hash(item)
		result = append(result, item)
	}
	return result, nil
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func floatPointer(value float64) *float64 {
	return &value
}

func roundNoteMetric(value float64, places int) float64 {
	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}

func estimatedPostbackCost(searchCost *float64) *float64 {
	if searchCost == nil {
		return nil
	}
	decimalCost, ok := new(big.Rat).SetString(strconv.FormatFloat(*searchCost, 'f', -1, 64))
	if !ok {
		return nil
	}
	searchCostCents := roundDecimalRat(new(big.Rat).Mul(decimalCost, big.NewRat(100, 1)))
	postbackCents := roundDecimalRat(new(big.Rat).SetFrac(
		new(big.Int).Mul(searchCostCents, big.NewInt(63)),
		big.NewInt(100),
	))
	result, _ := new(big.Rat).SetFrac(postbackCents, big.NewInt(100)).Float64()
	return floatPointer(result)
}

func roundDecimalRat(value *big.Rat) *big.Int {
	quotient := new(big.Int)
	remainder := new(big.Int)
	quotient.QuoRem(value.Num(), value.Denom(), remainder)
	doubledRemainder := new(big.Int).Lsh(new(big.Int).Abs(remainder), 1)
	if doubledRemainder.Cmp(value.Denom()) >= 0 {
		if value.Sign() < 0 {
			quotient.Sub(quotient, big.NewInt(1))
		} else {
			quotient.Add(quotient, big.NewInt(1))
		}
	}
	return quotient
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
		item.EstimatedPostbackCost = estimatedPostbackCost(item.SearchCost)
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
