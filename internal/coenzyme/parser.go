package coenzyme

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var ErrInvalidDailySheet = errors.New("invalid coenzyme Q10 daily sheet")

type dailyColumn struct {
	name    string
	aliases []string
}

var dailyColumns = []dailyColumn{
	{name: "时间"},
	{name: "消耗", aliases: []string{"消费"}},
	{name: "展现量"},
	{name: "点击量"},
	{name: "CTR"},
	{name: "CPC"},
	{name: "CPM"},
	{name: "全成交GMV"},
	{name: "全店ROI"},
	{name: "退后GMV"},
	{name: "退后ROI"},
	{name: "辅酶成交GMV"},
	{name: "辅酶成交ROI"},
	{name: "当日GMV"},
	{name: "当日ROI"},
	{name: "搜索消耗"},
	{name: "搜GMV"},
	{name: "搜ROI"},
	{name: "搜索投放占比"},
}

func ParseDailyValues(rows [][]interface{}) ([]DailyRecord, error) {
	headerRow, columns, err := findDailyColumns(rows)
	if err != nil {
		return nil, err
	}

	records := make([]DailyRecord, 0, len(rows)-headerRow-1)
	seen := make(map[string]int)
	for rowIndex := headerRow + 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		dateText := columnText(row, columns["时间"])
		date, ok := parseDailyDate(dateText)
		if !ok {
			if looksLikeDailyDate(dateText) {
				return nil, invalid("第 %d 行日期 %q 格式错误", rowIndex+1, dateText)
			}
			continue
		}
		dateKey := date.Format("2006-01-02")
		if firstRow, duplicate := seen[dateKey]; duplicate {
			return nil, invalid("日期 %s 在第 %d、%d 行重复", dateKey, firstRow, rowIndex+1)
		}

		record := DailyRecord{ReportDate: date, SourceRowNumber: rowIndex + 1}
		if record.Spend, err = optionalFloat(row, columns["消耗"], rowIndex+1, "消耗"); err != nil {
			return nil, err
		}
		if record.Impressions, err = optionalInt(row, columns["展现量"], rowIndex+1, "展现量"); err != nil {
			return nil, err
		}
		if record.Clicks, err = optionalInt(row, columns["点击量"], rowIndex+1, "点击量"); err != nil {
			return nil, err
		}
		floatTargets := []**float64{
			&record.CTR, &record.CPC, &record.CPM, &record.AllTransactionGMV,
			&record.AllStoreROI, &record.PostRefundGMV, &record.PostRefundROI,
			&record.CoenzymeGMV, &record.CoenzymeROI, &record.SameDayGMV,
			&record.SameDayROI, &record.SearchSpend, &record.SearchGMV,
			&record.SearchROI, &record.SearchSpendRatio,
		}
		floatNames := []string{
			"CTR", "CPC", "CPM", "全成交GMV", "全店ROI", "退后GMV", "退后ROI",
			"辅酶成交GMV", "辅酶成交ROI", "当日GMV", "当日ROI", "搜索消耗", "搜GMV",
			"搜ROI", "搜索投放占比",
		}
		for index, name := range floatNames {
			if *floatTargets[index], err = optionalFloat(row, columns[name], rowIndex+1, name); err != nil {
				return nil, err
			}
		}
		record.ContentHash = dailyRecordHash(record)
		seen[dateKey] = rowIndex + 1
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil, invalid("没有找到可同步的日期明细")
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ReportDate.Before(records[j].ReportDate) })
	return records, nil
}

func findDailyColumns(rows [][]interface{}) (int, map[string]int, error) {
	aliases := make(map[string]string)
	for _, column := range dailyColumns {
		aliases[normalize(column.name)] = column.name
		for _, alias := range column.aliases {
			aliases[normalize(alias)] = column.name
		}
	}
	bestRow := -1
	best := map[string]int{}
	for rowIndex, row := range rows {
		found := make(map[string]int)
		for columnIndex, cell := range row {
			canonical, ok := aliases[normalize(cellText(cell))]
			if !ok {
				continue
			}
			if _, exists := found[canonical]; !exists {
				found[canonical] = columnIndex
			}
		}
		if len(found) > len(best) {
			bestRow, best = rowIndex, found
		}
	}
	missing := make([]string, 0)
	for _, column := range dailyColumns {
		if _, ok := best[column.name]; !ok {
			missing = append(missing, column.name)
		}
	}
	if len(missing) > 0 {
		return bestRow, best, invalid("缺少必需列：%s", strings.Join(missing, "、"))
	}
	return bestRow, best, nil
}

func parseDailyDate(value string) (time.Time, bool) {
	for _, layout := range []string{"2006/1/2", "2006-1-2", "2006.1.2"} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func looksLikeDailyDate(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 6 || value[0] < '0' || value[0] > '9' {
		return false
	}
	return strings.ContainsAny(value, "/-.")
}

func optionalFloat(row []interface{}, column, rowNumber int, name string) (*float64, error) {
	value := columnText(row, column)
	if value == "" || value == "-" || value == "--" {
		return nil, nil
	}
	percentage := strings.HasSuffix(value, "%")
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	value = strings.NewReplacer(",", "", "￥", "", "¥", "").Replace(value)
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return nil, invalid("第 %d 行列 %q 的值 %q 不是数字", rowNumber, name, columnText(row, column))
	}
	if percentage {
		number /= 100
	}
	if number == 0 {
		number = 0
	}
	return &number, nil
}

func optionalInt(row []interface{}, column, rowNumber int, name string) (*int64, error) {
	value, err := optionalFloat(row, column, rowNumber, name)
	if err != nil || value == nil {
		return nil, err
	}
	if math.Trunc(*value) != *value {
		return nil, invalid("第 %d 行列 %q 的值 %q 不是整数", rowNumber, name, columnText(row, column))
	}
	result := int64(*value)
	return &result, nil
}

func columnText(row []interface{}, column int) string {
	if column < 0 || column >= len(row) {
		return ""
	}
	return cellText(row[column])
}

func cellText(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	}
	encoded, _ := json.Marshal(value)
	return strings.TrimSpace(string(encoded))
}

func normalize(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(value))
}

func dailyRecordHash(record DailyRecord) string {
	values := []string{
		record.ReportDate.Format("2006-01-02"), floatToken(record.Spend), intToken(record.Impressions),
		intToken(record.Clicks), floatToken(record.CTR), floatToken(record.CPC), floatToken(record.CPM),
		floatToken(record.AllTransactionGMV), floatToken(record.AllStoreROI), floatToken(record.PostRefundGMV),
		floatToken(record.PostRefundROI), floatToken(record.CoenzymeGMV), floatToken(record.CoenzymeROI),
		floatToken(record.SameDayGMV), floatToken(record.SameDayROI), floatToken(record.SearchSpend),
		floatToken(record.SearchGMV), floatToken(record.SearchROI), floatToken(record.SearchSpendRatio),
	}
	sum := sha256.Sum256([]byte(strings.Join(values, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func floatToken(value *float64) string {
	if value == nil {
		return "null"
	}
	if *value == 0 {
		return "0"
	}
	return strconv.FormatFloat(*value, 'g', -1, 64)
}

func intToken(value *int64) string {
	if value == nil {
		return "null"
	}
	return strconv.FormatInt(*value, 10)
}

func invalid(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrInvalidDailySheet, fmt.Sprintf(format, args...))
}
