package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
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
	"稿件",
	"封面类型",
	"商业强度",
	"对话人群",
	"用户场景",
	"进度",
}

var providerColumnAliases = map[string]string{
	"发布时间": "提交日期",
	"笔记类型": "内容类型",
	"人群标签": "对话人群",
	"对应场景": "用户场景",
	"笔记进度": "进度",
	"审核状态": "进度",
}

var providerNoteIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

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
	rawValues, err := c.readRawSheetRange(ctx, spreadsheetToken, dataRange)
	if err != nil {
		return model.ProviderContentSnapshot{}, fmt.Errorf("read provider %s raw records: %w", table.ProviderName, err)
	}

	records := make([]model.ProviderNoteExecution, 0, len(values))
	noteRefs := make([]model.DocumentRef, 0, len(values))
	seenNoteIDs := make(map[string]struct{}, len(values))
	noteErrors := 0
	for index, row := range values {
		sourceRow := headerRow + index + 2
		record := providerRecord(row, columns, minColumn, sourceRow)
		if providerRecordEmpty(record) || record.NoteID == "" {
			continue
		}
		records = append(records, record)
		if _, ok := seenNoteIDs[record.NoteID]; ok {
			continue
		}
		seenNoteIDs[record.NoteID] = struct{}{}
		if index >= len(rawValues) {
			noteErrors++
			continue
		}
		ref, ok := providerNoteDocumentRef(rawValues[index], columns, minColumn, record.NoteID)
		if !ok {
			noteErrors++
			continue
		}
		noteRefs = append(noteRefs, ref)
	}
	return model.ProviderContentSnapshot{
		Table: table, Records: records, NoteRefs: noteRefs, NoteErrors: noteErrors,
	}, nil
}

func (c *Client) FetchProviderNotes(ctx context.Context, refs []model.DocumentRef) ([]model.ProviderNote, int, error) {
	documents := make([]model.Document, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		key := ref.Provider + "\x00" + ref.ResourceKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if ref.Provider == "feishu" {
			documents = append(documents, c.fetchFeishuManuscript(ctx, ref))
			continue
		}
		documents = append(documents, model.Document{
			Provider: ref.Provider, ResourceKey: ref.ResourceKey, SourceURL: ref.SourceURL,
			DocumentType: "sheet", Content: "nan", Status: documentAuthNeeded,
			ErrorMessage: "non-Feishu manuscript content fetch is disabled",
		})
	}
	notes, fetchErrors := providerNotes(refs, documents)
	return notes, fetchErrors, nil
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
	return decodeSheetValues(resp.RawBody)
}

func (c *Client) readRawSheetRange(ctx context.Context, spreadsheetToken, readRange string) ([][]interface{}, error) {
	resp, err := c.client.Do(ctx, &larkcore.ApiReq{
		HttpMethod: http.MethodGet,
		ApiPath:    fmt.Sprintf("/open-apis/sheets/v2/spreadsheets/%s/values/%s", spreadsheetToken, readRange),
		SupportedAccessTokenTypes: []larkcore.AccessTokenType{
			larkcore.AccessTokenTypeTenant,
		},
	})
	if err != nil {
		return nil, err
	}
	return decodeSheetValues(resp.RawBody)
}

func decodeSheetValues(raw []byte) ([][]interface{}, error) {
	var values sheetValuesResponse
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("decode Sheets values: %w", err)
	}
	if values.Code != 0 {
		return nil, fmt.Errorf("code=%d message=%s", values.Code, values.Msg)
	}
	return values.Data.ValueRange.Values, nil
}

func findProviderColumns(rows [][]interface{}) (int, map[string]int, error) {
	wanted := make(map[string]string, len(providerColumnNames)+len(providerColumnAliases))
	for _, name := range providerColumnNames {
		wanted[normalizeHeader(name)] = name
	}
	for alias, canonical := range providerColumnAliases {
		wanted[normalizeHeader(alias)] = canonical
	}
	bestRow := -1
	best := map[string]int{}
	for rowIndex, row := range rows {
		found := make(map[string]int)
		for columnIndex, cell := range row {
			normalized := normalizeHeader(cellString(cell))
			canonical, ok := wanted[normalized]
			if ok {
				_, alreadyFound := found[canonical]
				if !alreadyFound || normalized == normalizeHeader(canonical) {
					found[canonical] = columnIndex
				}
			}
		}
		if len(found) > len(best) {
			bestRow, best = rowIndex, found
		}
		if len(found) == len(providerColumnNames) {
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
		NoteID:              normalizeProviderNoteID(value("笔记id")),
		CoverType:           value("封面类型"),
		CommercialIntensity: value("商业强度"),
		Audience:            value("对话人群"),
		UserScenario:        value("用户场景"),
		NoteType:            value("内容类型"),
		Progress:            value("进度"),
	}
}

func providerRecordEmpty(record model.ProviderNoteExecution) bool {
	return record.SubmissionDate == "" && record.NoteID == "" && record.CoverType == "" &&
		record.CommercialIntensity == "" && record.Audience == "" &&
		record.UserScenario == "" && record.NoteType == "" && record.Progress == ""
}

func normalizeProviderNoteID(value string) string {
	value = strings.TrimSpace(value)
	if !providerNoteIDPattern.MatchString(value) {
		return ""
	}
	return value
}

type providerDocumentLink struct {
	URL   string
	Label string
}

func providerNoteDocumentRef(row []interface{}, columns map[string]int, offset int, noteID string) (model.DocumentRef, bool) {
	column, ok := columns["稿件"]
	if !ok {
		return model.DocumentRef{}, false
	}
	index := column - offset
	if index < 0 || index >= len(row) {
		return model.DocumentRef{}, false
	}
	links := make([]providerDocumentLink, 0, 2)
	collectProviderDocumentLinks(row[index], "", &links)
	candidates := make([]model.DocumentRef, 0, len(links))
	seen := make(map[string]struct{}, len(links))
	for _, link := range links {
		provider, resourceKey, ok := parseDocumentLink(link.URL)
		if !ok || provider != "feishu" {
			continue
		}
		key := provider + "\x00" + resourceKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		candidates = append(candidates, model.DocumentRef{
			RecordID: noteID, FieldName: "稿件", Label: link.Label, Provider: provider,
			ResourceKey: resourceKey, SourceURL: link.URL,
		})
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	for index := len(candidates) - 1; index >= 0; index-- {
		if isFinalLinkLabel(candidates[index].Label) {
			return candidates[index], true
		}
	}
	return model.DocumentRef{}, false
}

func collectProviderDocumentLinks(value interface{}, inheritedLabel string, links *[]providerDocumentLink) {
	switch typed := value.(type) {
	case string:
		if strings.HasPrefix(typed, "http://") || strings.HasPrefix(typed, "https://") {
			*links = append(*links, providerDocumentLink{URL: typed, Label: inheritedLabel})
		}
	case []interface{}:
		for _, item := range typed {
			collectProviderDocumentLinks(item, inheritedLabel, links)
		}
	case map[string]interface{}:
		label := inheritedLabel
		if text, ok := typed["text"]; ok {
			if candidate := strings.TrimSpace(cellString(text)); candidate != "" {
				label = candidate
			}
		}
		for _, key := range []string{"link", "url"} {
			if rawLink, ok := typed[key]; ok {
				collectProviderDocumentLinks(rawLink, label, links)
			}
		}
		for key, item := range typed {
			if key == "link" || key == "url" || key == "text" {
				continue
			}
			collectProviderDocumentLinks(item, label, links)
		}
	}
}

func isFinalLinkLabel(value string) bool {
	normalized := normalizeHeader(value)
	return strings.Contains(normalized, "定稿") || strings.Contains(normalized, "终稿") ||
		strings.Contains(normalized, "最终稿")
}

func providerNotes(refs []model.DocumentRef, documents []model.Document) ([]model.ProviderNote, int) {
	contents := make(map[string]model.Document, len(documents))
	for _, document := range documents {
		if document.Status != documentSucceeded || (document.Content == "" && len(document.Blocks) == 0) {
			continue
		}
		contents[document.Provider+"\x00"+document.ResourceKey] = document
	}

	notes := make([]model.ProviderNote, 0, len(refs))
	errorsCount := 0
	for _, ref := range refs {
		document, ok := contents[ref.Provider+"\x00"+ref.ResourceKey]
		if !ok {
			errorsCount++
			continue
		}
		notes = append(notes, model.ProviderNote{
			NoteID: ref.RecordID, NoteContent: document.Content,
			ContentBlocks: document.Blocks, ReferenceNoteIDs: excludeReferenceNoteID(document.ReferenceNoteIDs, ref.RecordID),
			Assets: document.Assets, SourceURL: ref.SourceURL, SourceResourceKey: ref.ResourceKey,
			SourceRevision: document.RevisionID, ExtractorVersion: model.ManuscriptExtractorVersion,
		})
	}
	return notes, errorsCount
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
