package dandelion

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
)

var ErrInvalidWorkbook = errors.New("invalid Dandelion workbook")

type fieldKind uint8

const (
	kindString fieldKind = iota
	kindRichText
	kindLink
	kindDate
	kindNumber
)

type fieldSpec struct {
	name    string
	aliases []string
	kind    fieldKind
	core    bool
}

var fieldSpecs = []fieldSpec{
	{name: FieldNoteID, aliases: []string{"笔记id", "笔记 Id", "note id"}, kind: kindRichText, core: true},
	{name: FieldTitle, aliases: []string{"标题"}, kind: kindRichText, core: true},
	{name: FieldNoteURL, aliases: []string{"链接", "笔记URL", "笔记地址"}, kind: kindLink, core: true},
	{name: FieldAuthor, aliases: []string{"博主昵称", "达人发布账号", "达人账号", "发布账号", "达人"}, kind: kindRichText, core: true},
	{name: FieldPublishedAt, aliases: []string{"笔记发布日期", "发布日期"}, kind: kindDate, core: true},
	{name: FieldOrderingAccount, aliases: []string{"订单账号"}, kind: kindString, core: true},
	{name: FieldSPUName, aliases: []string{"SPU名称", "SPU", "spu"}, kind: kindString, core: true},
	{name: FieldDataUpdatedAt, aliases: []string{"更新日期", "数据更新时间"}, kind: kindDate, core: true},
	{name: FieldNoteType, aliases: []string{"内容类型"}, kind: kindString},
	{name: FieldContentTag, aliases: []string{"笔记标签"}, kind: kindString},
	{name: FieldDandelionAmount, aliases: []string{"蒲公英合作金额", "合作金额"}, kind: kindNumber},
	{name: FieldOffsiteActiveCost, aliases: []string{"站外活跃成本(15天设备归因)", "站外活跃成本"}, kind: kindNumber},
	{name: FieldImpressions, aliases: []string{"曝光数"}, kind: kindNumber},
	{name: FieldReads, aliases: []string{"阅读数"}, kind: kindNumber},
	{name: FieldInteractions, aliases: []string{"互动数"}, kind: kindNumber},
	{name: FieldReadCost, aliases: []string{"阅读成本"}, kind: kindNumber},
	{name: FieldInteractionCost, aliases: []string{"互动成本"}, kind: kindNumber},
}

type sheetCandidate struct {
	name      string
	rows      [][]string
	headerRow int
	columns   map[string]int
	headers   []string
	nonEmpty  int
}

func Parse(reader io.Reader, fileName string) (Snapshot, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read Dandelion workbook: %w", err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return Snapshot{}, invalid("无法读取 .xlsx 文件")
	}
	defer func() { _ = workbook.Close() }()

	candidate, err := selectSheet(workbook)
	if err != nil {
		return Snapshot{}, err
	}
	missing := make([]string, 0)
	for _, spec := range fieldSpecs {
		if _, ok := candidate.columns[spec.name]; spec.core && !ok {
			missing = append(missing, spec.name)
		}
	}
	if len(missing) > 0 {
		return Snapshot{}, invalid("工作表 %q 缺少核心字段：%s", candidate.name, strings.Join(missing, "、"))
	}

	sum := sha256.Sum256(data)
	snapshot := Snapshot{
		FileName: fileName, FileSHA256: hex.EncodeToString(sum[:]),
		SheetName: candidate.name, HeaderRow: candidate.headerRow + 1,
		MatchedFields: make([]string, 0, len(candidate.columns)),
	}
	for _, spec := range fieldSpecs {
		if _, ok := candidate.columns[spec.name]; ok {
			snapshot.MatchedFields = append(snapshot.MatchedFields, spec.name)
		}
	}
	seen := make(map[string]int)
	for rowIndex := candidate.headerRow + 1; rowIndex < len(candidate.rows); rowIndex++ {
		sourceRow := rowIndex + 1
		if recognizedRowBlank(candidate.rows[rowIndex], candidate.columns) {
			continue
		}
		record, err := parseRecord(workbook, candidate, rowIndex)
		if err != nil {
			return Snapshot{}, err
		}
		if first, exists := seen[record.RecordID]; exists {
			return Snapshot{}, invalid("工作表 %q 第 %d、%d 行的笔记 ID 与数据更新日期重复", candidate.name, first, sourceRow)
		}
		seen[record.RecordID] = sourceRow
		snapshot.Records = append(snapshot.Records, record)
	}
	if len(snapshot.Records) == 0 {
		return Snapshot{}, invalid("工作表 %q 没有可导入的数据行", candidate.name)
	}
	return snapshot, nil
}

func selectSheet(workbook *excelize.File) (sheetCandidate, error) {
	var best sheetCandidate
	for _, sheetName := range workbook.GetSheetList() {
		rows, err := workbook.GetRows(sheetName)
		if err != nil {
			return sheetCandidate{}, invalid("读取工作表 %q 失败", sheetName)
		}
		limit := min(len(rows), 30)
		for rowIndex := 0; rowIndex < limit; rowIndex++ {
			columns, nonEmpty := inspectHeader(rows[rowIndex])
			if len(columns) > len(best.columns) || (len(columns) == len(best.columns) && nonEmpty > best.nonEmpty) {
				best = sheetCandidate{name: sheetName, rows: rows, headerRow: rowIndex, columns: columns, headers: uniqueHeaders(rows[rowIndex]), nonEmpty: nonEmpty}
			}
		}
	}
	if len(best.columns) == 0 {
		return sheetCandidate{}, invalid("未找到可识别的蒲公英数据表")
	}
	if err := applyMergedHeaders(workbook, &best); err != nil {
		return sheetCandidate{}, err
	}
	return best, nil
}

func applyMergedHeaders(workbook *excelize.File, candidate *sheetCandidate) error {
	mergedCells, err := workbook.GetMergeCells(candidate.name)
	if err != nil {
		return invalid("读取工作表 %q 的合并表头失败", candidate.name)
	}
	targetRow := candidate.headerRow + 1
	for _, merged := range mergedCells {
		startColumn, startRow, startErr := excelize.CellNameToCoordinates(merged.GetStartAxis())
		endColumn, endRow, endErr := excelize.CellNameToCoordinates(merged.GetEndAxis())
		if startErr != nil || endErr != nil || startColumn != endColumn || targetRow < startRow || targetRow > endRow {
			continue
		}
		header := strings.TrimSpace(merged.GetCellValue())
		spec, found := specForHeader(header)
		if !found {
			continue
		}
		column := startColumn - 1
		if _, exists := candidate.columns[spec.name]; exists {
			continue
		}
		candidate.columns[spec.name] = column
		if len(candidate.headers) <= column {
			candidate.headers = append(candidate.headers, make([]string, column-len(candidate.headers)+1)...)
		}
		candidate.headers[column] = header
	}
	return nil
}

func inspectHeader(row []string) (map[string]int, int) {
	columns := make(map[string]int)
	nonEmpty := 0
	for column, value := range row {
		header := strings.TrimSpace(value)
		if header == "" {
			continue
		}
		nonEmpty++
		spec, found := specForHeader(header)
		if !found {
			continue
		}
		previous, exists := columns[spec.name]
		if !exists || (header == spec.name && strings.TrimSpace(row[previous]) != spec.name) {
			columns[spec.name] = column
		}
	}
	return columns, nonEmpty
}

func uniqueHeaders(row []string) []string {
	headers := make([]string, len(row))
	counts := make(map[string]int)
	for column, value := range row {
		header := strings.TrimSpace(value)
		if header == "" {
			continue
		}
		count := counts[header]
		counts[header] = count + 1
		if count > 0 {
			header += fmt.Sprintf("（%d）", count)
		}
		headers[column] = header
	}
	return headers
}

func specForHeader(header string) (fieldSpec, bool) {
	normalized := normalizeHeader(header)
	for _, spec := range fieldSpecs {
		if normalizeHeader(spec.name) == normalized {
			return spec, true
		}
		for _, alias := range spec.aliases {
			if normalizeHeader(alias) == normalized {
				return spec, true
			}
		}
	}
	return fieldSpec{}, false
}

func normalizeHeader(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("_/\\-（）()", r) {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(value))
}

func recognizedRowBlank(row []string, columns map[string]int) bool {
	for _, column := range columns {
		if column < len(row) && strings.TrimSpace(row[column]) != "" {
			return false
		}
	}
	return true
}

func parseRecord(workbook *excelize.File, candidate sheetCandidate, rowIndex int) (Record, error) {
	sourceRow := rowIndex + 1
	fields := make(map[string]interface{}, len(candidate.headers))
	matchedColumns := make(map[int]struct{}, len(candidate.columns))
	for _, column := range candidate.columns {
		matchedColumns[column] = struct{}{}
	}
	for column, header := range candidate.headers {
		if header == "" {
			continue
		}
		if _, matched := matchedColumns[column]; matched {
			continue
		}
		raw := cellValue(workbook, candidate, rowIndex, column, false)
		if raw != "" {
			fields[header] = genericCellValue(workbook, candidate, rowIndex, column, raw)
		}
	}
	var noteID string
	var dataUpdated time.Time
	for _, spec := range fieldSpecs {
		column, found := candidate.columns[spec.name]
		if !found {
			continue
		}
		raw := cellValue(workbook, candidate, rowIndex, column, spec.kind == kindLink)
		if raw == "" {
			continue
		}
		switch spec.kind {
		case kindRichText:
			fields[spec.name] = []map[string]string{{"text": raw, "type": "text"}}
			if spec.name == FieldNoteID {
				noteID = strings.ToLower(strings.TrimSpace(raw))
			}
		case kindLink:
			fields[spec.name] = []map[string]string{{"link": raw, "text": raw}}
		case kindDate:
			date, err := parseDate(raw)
			if err != nil {
				return Record{}, invalid("工作表 %q 第 %d 行字段 %q：%v", candidate.name, sourceRow, spec.name, err)
			}
			fields[spec.name] = date.UnixMilli()
			if spec.name == FieldDataUpdatedAt {
				dataUpdated = date
			}
		case kindNumber:
			number, present, err := parseNumber(raw)
			if err != nil {
				return Record{}, invalid("工作表 %q 第 %d 行字段 %q：%v", candidate.name, sourceRow, spec.name, err)
			}
			if present {
				fields[spec.name] = number
			}
		default:
			fields[spec.name] = raw
		}
	}
	if noteID == "" {
		return Record{}, invalid("工作表 %q 第 %d 行笔记ID为空", candidate.name, sourceRow)
	}
	if dataUpdated.IsZero() {
		return Record{}, invalid("工作表 %q 第 %d 行数据更新日期为空", candidate.name, sourceRow)
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return Record{}, fmt.Errorf("encode Dandelion row %d: %w", sourceRow, err)
	}
	key := sha256.Sum256([]byte(noteID + "\x00" + strconv.FormatInt(dataUpdated.UnixMilli(), 10)))
	return Record{
		RecordID: "excel_" + hex.EncodeToString(key[:]), SourceRow: sourceRow,
		NoteID: noteID, DataUpdated: dataUpdated, Fields: encoded,
	}, nil
}

func cellValue(workbook *excelize.File, candidate sheetCandidate, rowIndex, column int, preferLink bool) string {
	axis, err := excelize.CoordinatesToCellName(column+1, rowIndex+1)
	if err != nil {
		return ""
	}
	if preferLink {
		if linked, target, linkErr := workbook.GetCellHyperLink(candidate.name, axis); linkErr == nil && linked && strings.TrimSpace(target) != "" {
			return strings.TrimSpace(target)
		}
	}
	value, err := workbook.GetCellValue(candidate.name, axis)
	if err == nil {
		return strings.TrimSpace(value)
	}
	if rowIndex < len(candidate.rows) && column < len(candidate.rows[rowIndex]) {
		return strings.TrimSpace(candidate.rows[rowIndex][column])
	}
	return ""
}

func genericCellValue(workbook *excelize.File, candidate sheetCandidate, rowIndex, column int, formatted string) interface{} {
	axis, err := excelize.CoordinatesToCellName(column+1, rowIndex+1)
	if err != nil {
		return formatted
	}
	cellType, err := workbook.GetCellType(candidate.name, axis)
	if err != nil {
		return formatted
	}
	switch cellType {
	case excelize.CellTypeBool:
		if value, parseErr := strconv.ParseBool(strings.ToLower(formatted)); parseErr == nil {
			return value
		}
	case excelize.CellTypeDate:
		if value, parseErr := parseDate(formatted); parseErr == nil {
			return value.UnixMilli()
		}
	case excelize.CellTypeNumber, excelize.CellTypeUnset:
		raw, valueErr := workbook.GetCellValue(candidate.name, axis, excelize.Options{RawCellValue: true})
		if valueErr == nil {
			if value, parseErr := strconv.ParseFloat(raw, 64); parseErr == nil && !math.IsNaN(value) && !math.IsInf(value, 0) {
				return value
			}
		}
	}
	return formatted
}

func parseDate(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, layout := range []string{
		"2006-01-02", "2006/01/02", "2006.01.02", "2006年1月2日",
		"2006-01-02 15:04:05", "2006/01/02 15:04:05", time.RFC3339,
	} {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return parsed, nil
		}
	}
	number, err := strconv.ParseFloat(value, 64)
	if err == nil {
		switch {
		case number >= 1e12:
			return time.UnixMilli(int64(number)).In(location), nil
		case number >= 1e9:
			return time.Unix(int64(number), 0).In(location), nil
		case number > 0:
			parsed, dateErr := excelize.ExcelDateToTime(number, false)
			if dateErr == nil {
				return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, location), nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("无法识别日期 %q", raw)
}

func parseNumber(raw string) (float64, bool, error) {
	value := strings.TrimSpace(raw)
	switch strings.ToLower(value) {
	case "", "-", "--", "n/a", "null":
		return 0, false, nil
	}
	value = strings.NewReplacer(",", "", "¥", "", "￥", "").Replace(value)
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false, fmt.Errorf("无法识别数值 %q", raw)
	}
	return number, true, nil
}

func invalid(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrInvalidWorkbook, fmt.Sprintf(format, args...))
}
