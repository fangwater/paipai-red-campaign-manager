package lark

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/model"

	larksdk "github.com/larksuite/oapi-sdk-go/v3"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"
)

const (
	pageSize           = 500
	tablePageSize      = 100
	docxRequestDelay   = 220 * time.Millisecond
	documentSucceeded  = "succeeded"
	documentFailed     = "failed"
	documentAuthNeeded = "auth_required"
)

type Client struct {
	client   *larksdk.Client
	appToken string
}

type SingleTableSource struct {
	client  *Client
	tableID string
}

func NewSingleTableSource(appID, appSecret, appToken, tableID string) *SingleTableSource {
	return &SingleTableSource{
		client:  NewClient(appID, appSecret, appToken),
		tableID: tableID,
	}
}

func (source *SingleTableSource) Snapshot(ctx context.Context) (model.Snapshot, error) {
	return source.client.snapshotTable(ctx, source.tableID)
}

func (source *SingleTableSource) FetchDocuments(ctx context.Context, refs []model.DocumentRef) ([]model.Document, error) {
	return source.client.FetchDocuments(ctx, refs)
}

func NewClient(appID, appSecret, appToken string) *Client {
	return &Client{
		client:   larksdk.NewClient(appID, appSecret),
		appToken: appToken,
	}
}

func (c *Client) Snapshot(ctx context.Context) (model.Snapshot, error) {
	tables, err := c.listTables(ctx)
	if err != nil {
		return model.Snapshot{}, err
	}

	snapshot := model.Snapshot{Tables: make([]model.Table, 0, len(tables))}
	for _, table := range tables {
		records, refs, err := c.listRecords(ctx, table.ID)
		if err != nil {
			return model.Snapshot{}, fmt.Errorf("sync table %s (%s): %w", table.Name, table.ID, err)
		}
		table.Records = records
		snapshot.Tables = append(snapshot.Tables, table)
		snapshot.DocumentRefs = append(snapshot.DocumentRefs, refs...)
	}
	return snapshot, nil
}

func (c *Client) snapshotTable(ctx context.Context, tableID string) (model.Snapshot, error) {
	tables, err := c.listTables(ctx)
	if err != nil {
		return model.Snapshot{}, err
	}
	table, err := selectTable(tables, tableID)
	if err != nil {
		return model.Snapshot{}, err
	}
	records, refs, err := c.listRecords(ctx, table.ID)
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("sync table %s (%s): %w", table.Name, table.ID, err)
	}
	table.Records = records
	return model.Snapshot{Tables: []model.Table{table}, DocumentRefs: refs}, nil
}

func selectTable(tables []model.Table, tableID string) (model.Table, error) {
	for _, table := range tables {
		if table.ID == tableID {
			return table, nil
		}
	}
	return model.Table{}, fmt.Errorf("Bitable table %q was not found", tableID)
}

func (c *Client) FetchDocuments(ctx context.Context, refs []model.DocumentRef) ([]model.Document, error) {
	documents := make([]model.Document, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		key := ref.Provider + "\x00" + ref.ResourceKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if ref.Provider == "feishu" {
			documents = append(documents, c.fetchFeishuDocument(ctx, ref))
			continue
		}
		documents = append(documents, model.Document{
			Provider:     ref.Provider,
			ResourceKey:  ref.ResourceKey,
			SourceURL:    ref.SourceURL,
			DocumentType: "sheet",
			Content:      "nan",
			Status:       documentAuthNeeded,
			ErrorMessage: "Tencent document content fetch is disabled",
		})
	}
	return documents, nil
}

func (c *Client) listTables(ctx context.Context) ([]model.Table, error) {
	tables := make([]model.Table, 0, 20)
	pageToken := ""
	for {
		builder := larkbitable.NewListAppTableReqBuilder().
			AppToken(c.appToken).
			PageSize(tablePageSize)
		if pageToken != "" {
			builder.PageToken(pageToken)
		}

		resp, err := c.client.Bitable.AppTable.List(ctx, builder.Build())
		if err != nil {
			return nil, fmt.Errorf("list Bitable tables: %w", err)
		}
		if !resp.Success() {
			return nil, fmt.Errorf("list Bitable tables: code=%d message=%s", resp.Code, resp.Msg)
		}
		if resp.Data == nil {
			return nil, errors.New("Bitable returned an empty table response")
		}
		for _, item := range resp.Data.Items {
			if item == nil || item.TableId == nil || item.Name == nil {
				continue
			}
			revision := 0
			if item.Revision != nil {
				revision = *item.Revision
			}
			tables = append(tables, model.Table{ID: *item.TableId, Name: *item.Name, Revision: revision})
		}

		hasMore := resp.Data.HasMore != nil && *resp.Data.HasMore
		if !hasMore {
			return tables, nil
		}
		if resp.Data.PageToken == nil || *resp.Data.PageToken == "" {
			return nil, errors.New("Bitable table response has_more is true but page_token is empty")
		}
		pageToken = *resp.Data.PageToken
	}
}

func (c *Client) listRecords(ctx context.Context, tableID string) ([]model.Record, []model.DocumentRef, error) {
	body := larkbitable.NewSearchAppTableRecordReqBodyBuilder().
		AutomaticFields(true).
		Build()

	records := make([]model.Record, 0, pageSize)
	refs := make([]model.DocumentRef, 0)
	pageToken := ""
	for {
		builder := larkbitable.NewSearchAppTableRecordReqBuilder().
			AppToken(c.appToken).
			TableId(tableID).
			PageSize(pageSize).
			Body(body)
		if pageToken != "" {
			builder.PageToken(pageToken)
		}

		resp, err := c.client.Bitable.AppTableRecord.Search(ctx, builder.Build())
		if err != nil {
			return nil, nil, fmt.Errorf("search Bitable records: %w", err)
		}
		if !resp.Success() {
			return nil, nil, fmt.Errorf("search Bitable records: code=%d message=%s", resp.Code, resp.Msg)
		}
		if resp.Data == nil {
			return nil, nil, errors.New("Bitable returned an empty record response")
		}

		for _, item := range resp.Data.Items {
			if item == nil || item.RecordId == nil || *item.RecordId == "" {
				return nil, nil, errors.New("Bitable returned a record without record_id")
			}
			fields, err := json.Marshal(item.Fields)
			if err != nil {
				return nil, nil, fmt.Errorf("encode fields for record %s: %w", *item.RecordId, err)
			}
			records = append(records, model.Record{
				ID:        *item.RecordId,
				Fields:    fields,
				CreatedAt: millisecondsToTime(item.CreatedTime),
				UpdatedAt: millisecondsToTime(item.LastModifiedTime),
			})
			refs = append(refs, extractDocumentRefs(tableID, *item.RecordId, item.Fields)...)
		}

		hasMore := resp.Data.HasMore != nil && *resp.Data.HasMore
		if !hasMore {
			return records, refs, nil
		}
		if resp.Data.PageToken == nil || *resp.Data.PageToken == "" {
			return nil, nil, errors.New("Bitable record response has_more is true but page_token is empty")
		}
		pageToken = *resp.Data.PageToken
	}
}

func (c *Client) fetchFeishuDocument(ctx context.Context, ref model.DocumentRef) model.Document {
	document := model.Document{
		Provider:    ref.Provider,
		ResourceKey: ref.ResourceKey,
		SourceURL:   ref.SourceURL,
		Status:      documentFailed,
	}
	kind, token, ok := strings.Cut(ref.ResourceKey, ":")
	if !ok || token == "" {
		document.ErrorMessage = "invalid Feishu document resource key"
		return document
	}

	documentToken := token
	document.DocumentType = kind
	if kind == "wiki" {
		resp, err := c.client.Wiki.Space.GetNode(ctx, larkwiki.NewGetNodeSpaceReqBuilder().Token(token).Build())
		if err != nil {
			document.ErrorMessage = "resolve Wiki node: " + err.Error()
			return document
		}
		if !resp.Success() {
			document.ErrorMessage = fmt.Sprintf("resolve Wiki node: code=%d message=%s", resp.Code, resp.Msg)
			return document
		}
		if resp.Data == nil || resp.Data.Node == nil || resp.Data.Node.ObjToken == nil || resp.Data.Node.ObjType == nil {
			document.ErrorMessage = "resolve Wiki node: empty node response"
			return document
		}
		documentToken = *resp.Data.Node.ObjToken
		document.DocumentType = *resp.Data.Node.ObjType
		if resp.Data.Node.Title != nil {
			document.Title = *resp.Data.Node.Title
		}
	}
	if document.DocumentType != "docx" {
		document.ErrorMessage = "unsupported Feishu document type: " + document.DocumentType
		return document
	}

	if err := waitForDocxRequest(ctx); err != nil {
		document.ErrorMessage = err.Error()
		return document
	}
	meta, err := c.client.Docx.Document.Get(ctx, larkdocx.NewGetDocumentReqBuilder().DocumentId(documentToken).Build())
	if err != nil {
		document.ErrorMessage = "get Docx metadata: " + err.Error()
		return document
	}
	if !meta.Success() {
		document.ErrorMessage = fmt.Sprintf("get Docx metadata: code=%d message=%s", meta.Code, meta.Msg)
		return document
	}
	if meta.Data != nil && meta.Data.Document != nil {
		if meta.Data.Document.Title != nil {
			document.Title = *meta.Data.Document.Title
		}
		if meta.Data.Document.RevisionId != nil {
			document.RevisionID = *meta.Data.Document.RevisionId
		}
	}

	if err := waitForDocxRequest(ctx); err != nil {
		document.ErrorMessage = err.Error()
		return document
	}
	content, err := c.client.Docx.Document.RawContent(ctx, larkdocx.NewRawContentDocumentReqBuilder().DocumentId(documentToken).Build())
	if err != nil {
		document.ErrorMessage = "get Docx content: " + err.Error()
		return document
	}
	if !content.Success() {
		document.ErrorMessage = fmt.Sprintf("get Docx content: code=%d message=%s", content.Code, content.Msg)
		return document
	}
	if content.Data == nil || content.Data.Content == nil {
		document.ErrorMessage = "get Docx content: empty content response"
		return document
	}
	document.Content = *content.Data.Content
	document.Status = documentSucceeded
	document.ErrorMessage = ""
	return document
}

func extractDocumentRefs(tableID, recordID string, fields map[string]interface{}) []model.DocumentRef {
	refs := make([]model.DocumentRef, 0)
	seen := make(map[string]struct{})
	for fieldName, value := range fields {
		links := make([]string, 0)
		collectLinks(value, &links)
		for _, rawLink := range links {
			provider, resourceKey, ok := parseDocumentLink(rawLink)
			if !ok {
				continue
			}
			key := fieldName + "\x00" + provider + "\x00" + resourceKey
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			refs = append(refs, model.DocumentRef{
				TableID:     tableID,
				RecordID:    recordID,
				FieldName:   fieldName,
				Provider:    provider,
				ResourceKey: resourceKey,
				SourceURL:   rawLink,
			})
		}
	}
	return refs
}

func collectLinks(value interface{}, links *[]string) {
	switch typed := value.(type) {
	case string:
		if strings.HasPrefix(typed, "http://") || strings.HasPrefix(typed, "https://") {
			*links = append(*links, typed)
		}
	case []interface{}:
		for _, item := range typed {
			collectLinks(item, links)
		}
	case map[string]interface{}:
		for _, item := range typed {
			collectLinks(item, links)
		}
	}
}

func parseDocumentLink(rawLink string) (string, string, bool) {
	parsed, err := url.Parse(rawLink)
	if err != nil {
		return "", "", false
	}
	parsed.Fragment = ""
	host := strings.ToLower(parsed.Hostname())
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if strings.HasSuffix(host, ".feishu.cn") && len(parts) >= 2 && (parts[0] == "wiki" || parts[0] == "docx") {
		return "feishu", parts[0] + ":" + parts[1], true
	}
	if len(parts) >= 2 && (host == "doc.weixin.qq.com" || host == "docs.qq.com") {
		normalized := parsed.String()
		sum := sha256.Sum256([]byte(normalized))
		provider := "tencent"
		if host == "doc.weixin.qq.com" {
			provider = "weixin"
		}
		return provider, hex.EncodeToString(sum[:]), true
	}
	return "", "", false
}

func waitForDocxRequest(ctx context.Context) error {
	timer := time.NewTimer(docxRequestDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func millisecondsToTime(value *int64) *time.Time {
	if value == nil || *value <= 0 {
		return nil
	}
	t := time.UnixMilli(*value)
	return &t
}
