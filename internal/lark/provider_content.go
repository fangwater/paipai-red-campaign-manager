package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"paipai-red-campaign-manager/internal/model"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larksheets "github.com/larksuite/oapi-sdk-go/v3/service/sheets/v3"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"
)

const (
	headerScanRows    = 20
	headerScanColumns = 100
)

var providerColumnNames = []string{
	"提交日期",
	"笔记id",
	"内容类型",
	"封面类型",
	"商业强度",
	"对话人群",
	"用户场景",
	"笔记类型",
	"进度",
	"审核反馈",
}

type sheetValuesResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ValueRange struct {
			Range  string          `json:"range"`
			Values [][]interface{} `json:"values"`
		} `json:"valueRange"`
	} `json:"data"`
}

func (c *Client) FetchProviderContent(ctx context.Context, table model.ProviderContentTable) (model.ProviderContentSnapshot, error) {
	spreadsheetToken, err := c.resolveSpreadsheetToken(ctx, table.WikiToken)
	if err != nil {
		return model.ProviderContentSnapshot{}, fmt.Errorf("resolve provider %s spreadsheet: %w", table.ProviderName, err)
	}
	table.SpreadsheetToken = spreadsheetToken

	sheetID, sheetName, rowCount, err := c.resolveSheet(ctx, spreadsheetToken, table.SheetID, table.SheetName)
	if err != nil {
		return model.ProviderContentSnapshot{}, fmt.Errorf("resolve provider %s worksheet: %w", table.ProviderName, err)
	}
	table.SheetID = sheetID
	table.SheetName = sheetName

	headerRange := fmt.Sprintf("%s!A1:%s%d", sheetID, columnName(headerScanColumns-1), headerScanRows)
	headerValues, err := c.readSheetRange(ctx, spreadsheetToken, headerRange)
	if err != nil {
		return model.ProviderContentSnapshot{}, fmt.Errorf("read provider %s headers: %w", table.ProviderName, err)
	}
	headerRow, columns, err := findProviderColumns(headerValues)
	if err != nil {
		return model.ProviderContentSnapshot{}, fmt.Errorf("map provider %s headers: %w", table.ProviderName, err)
	}

	minColumn, maxColumn := providerColumnBounds(columns)
	if rowCount < headerRow+2 {
		rowCount = headerRow + 2
	}
	dataRange := fmt.Sprintf("%s!%s%d:%s%d", sheetID, columnName(minColumn), headerRow+2, columnName(maxColumn), rowCount)
	values, err := c.readSheetRange(ctx, spreadsheetToken, dataRange)
	if err != nil {
		return model.ProviderContentSnapshot{}, fmt.Errorf("read provider %s records: %w", table.ProviderName, err)
	}

	records := make([]model.ProviderNoteExecution, 0, len(values))
	for index, row := range values {
		sourceRow := headerRow + index + 2
		record := providerRecord(row, columns, minColumn, sourceRow)
		if providerRecordEmpty(record) {
			continue
		}
		records = append(records, record)
	}
	return model.ProviderContentSnapshot{Table: table, Records: records}, nil
}

func (c *Client) resolveSpreadsheetToken(ctx context.Context, wikiToken string) (string, error) {
	if wikiToken == "" {
		return "", errors.New("wiki token is empty")
	}
	resp, err := c.client.Wiki.Space.GetNode(ctx, larkwiki.NewGetNodeSpaceReqBuilder().Token(wikiToken).Build())
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("code=%d message=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.Node == nil || resp.Data.Node.ObjToken == nil || *resp.Data.Node.ObjToken == "" {
		return "", errors.New("Wiki returned an empty object token")
	}
	if resp.Data.Node.ObjType == nil || *resp.Data.Node.ObjType != "sheet" {
		return "", fmt.Errorf("Wiki object type is %q, want sheet", stringValue(resp.Data.Node.ObjType))
	}
	return *resp.Data.Node.ObjToken, nil
}

func (c *Client) resolveSheet(ctx context.Context, spreadsheetToken, preferredID, preferredName string) (string, string, int, error) {
	resp, err := c.client.Sheets.V3.SpreadsheetSheet.Query(ctx,
		larksheets.NewQuerySpreadsheetSheetReqBuilder().SpreadsheetToken(spreadsheetToken).Build())
	if err != nil {
		return "", "", 0, err
	}
	if !resp.Success() {
		return "", "", 0, fmt.Errorf("code=%d message=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil {
		return "", "", 0, errors.New("Sheets returned an empty worksheet response")
	}

	var idMatch *larksheets.Sheet
	for _, sheet := range resp.Data.Sheets {
		if sheet == nil || sheet.SheetId == nil || sheet.Title == nil {
			continue
		}
		if preferredName != "" && *sheet.Title == preferredName {
			return resolvedSheet(sheet)
		}
		if preferredID != "" && *sheet.SheetId == preferredID {
			idMatch = sheet
		}
	}
	if idMatch != nil {
		return resolvedSheet(idMatch)
	}
	return "", "", 0, fmt.Errorf("worksheet name=%q id=%q was not found", preferredName, preferredID)
}

func resolvedSheet(sheet *larksheets.Sheet) (string, string, int, error) {
	rowCount := 1000
	if sheet.GridProperties != nil && sheet.GridProperties.RowCount != nil && *sheet.GridProperties.RowCount > 0 {
		rowCount = *sheet.GridProperties.RowCount
	}
	return *sheet.SheetId, *sheet.Title, rowCount, nil
}

func (c *Client) readSheetRange(ctx context.Context, spreadsheetToken, readRange string) ([][]interface{}, error) {
	resp, err := c.client.Do(ctx, &larkcore.ApiReq{
		HttpMethod: http.MethodGet,
		ApiPath:    fmt.Sprintf("/open-apis/sheets/v2/spreadsheets/%s/values/%s", spreadsheetToken, readRange),
		QueryParams: larkcore.QueryParams{
			"valueRenderOption":    []string{"ToString"},
			"dateTimeRenderOption": []string{"FormattedString"},
		},
		SupportedAccessTokenTypes: []larkcore.AccessTokenType{larkcore.AccessTokenTypeTenant},
	})
	if err != nil {
		return nil, err
	}
	var values sheetValuesResponse
	if err := json.Unmarshal(resp.RawBody, &values); err != nil {
		return nil, fmt.Errorf("decode Sheets values: %w", err)
	}
	if values.Code != 0 {
		return nil, fmt.Errorf("code=%d message=%s", values.Code, values.Msg)
	}
	return values.Data.ValueRange.Values, nil
}

func findProviderColumns(rows [][]interface{}) (int, map[string]int, error) {
	wanted := make(map[string]string, len(providerColumnNames))
	for _, name := range providerColumnNames {
		wanted[normalizeHeader(name)] = name
	}
	bestRow := -1
	best := map[string]int{}
	for rowIndex, row := range rows {
		found := make(map[string]int)
		for columnIndex, cell := range row {
			canonical, ok := wanted[normalizeHeader(cellString(cell))]
			if ok {
				found[canonical] = columnIndex
			}
		}
		if len(found) > len(best) {
			bestRow, best = rowIndex, found
		}
		if len(found) == len(wanted) {
			return rowIndex, found, nil
		}
	}
	missing := make([]string, 0)
	for _, name := range providerColumnNames {
		if _, ok := best[name]; !ok {
			missing = append(missing, name)
		}
	}
	return bestRow, best, fmt.Errorf("required columns not found: %s", strings.Join(missing, ", "))
}

func providerColumnBounds(columns map[string]int) (int, int) {
	minColumn, maxColumn := headerScanColumns, 0
	for _, column := range columns {
		if column < minColumn {
			minColumn = column
		}
		if column > maxColumn {
			maxColumn = column
		}
	}
	return minColumn, maxColumn
}

func providerRecord(row []interface{}, columns map[string]int, offset, sourceRow int) model.ProviderNoteExecution {
	value := func(name string) string {
		index := columns[name] - offset
		if index < 0 || index >= len(row) {
			return ""
		}
		return cellString(row[index])
	}
	return model.ProviderNoteExecution{
		RecordKey:           "row:" + strconv.Itoa(sourceRow),
		SourceRowNumber:     sourceRow,
		SubmissionDate:      value("提交日期"),
		NoteID:              value("笔记id"),
		ContentType:         value("内容类型"),
		CoverType:           value("封面类型"),
		CommercialIntensity: value("商业强度"),
		Audience:            value("对话人群"),
		UserScenario:        value("用户场景"),
		NoteType:            value("笔记类型"),
		Progress:            value("进度"),
		ReviewFeedback:      value("审核反馈"),
	}
}

func providerRecordEmpty(record model.ProviderNoteExecution) bool {
	return record.SubmissionDate == "" && record.NoteID == "" && record.ContentType == "" &&
		record.CoverType == "" && record.CommercialIntensity == "" && record.Audience == "" &&
		record.UserScenario == "" && record.NoteType == "" && record.Progress == "" && record.ReviewFeedback == ""
}

func normalizeHeader(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(value))
}

func cellString(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	case []interface{}:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := cellString(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	case map[string]interface{}:
		for _, key := range []string{"text", "name", "value", "link"} {
			if item, ok := typed[key]; ok {
				if text := cellString(item); text != "" {
					return text
				}
			}
		}
	}
	encoded, _ := json.Marshal(value)
	return strings.TrimSpace(string(encoded))
}

func columnName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+index%26)) + name
		index = index/26 - 1
	}
	return name
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
