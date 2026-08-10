package lark

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"paipai-red-campaign-manager/internal/model"

	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"
	xdraw "golang.org/x/image/draw"
)

const (
	manuscriptBlockPageSize            = 500
	maxManuscriptDownloadBytes         = 50 * 1024 * 1024
	maxManuscriptImageBytes            = 10 * 1024 * 1024
	maxManuscriptAssetBytes            = 100 * 1024 * 1024
	manuscriptImageOptimizeBytes       = 2 * 1024 * 1024
	manuscriptImageMaxDimension        = 2560
	manuscriptImageMaxPixels     int64 = 64_000_000
	manuscriptImageJPEGQuality         = 85
)

var xiaohongshuNoteURLPattern = regexp.MustCompile(`(?i)xiaohongshu[.]com/(?:explore/|discovery/item/)([0-9a-f]{24})(?:[^0-9a-f]|$)`)

func (c *Client) fetchFeishuManuscript(ctx context.Context, ref model.DocumentRef) model.Document {
	document := model.Document{
		Provider: ref.Provider, ResourceKey: ref.ResourceKey, SourceURL: ref.SourceURL,
		Status: documentFailed,
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

	items, err := c.listManuscriptBlocks(ctx, documentToken, document.RevisionID)
	if err != nil {
		document.ErrorMessage = "list Docx blocks: " + err.Error()
		return document
	}
	blocks := selectFinalManuscriptBlocks(parseManuscriptBlocks(items))
	if len(blocks) == 0 {
		document.ErrorMessage = "Docx final manuscript section is empty"
		return document
	}
	blocks, assets, err := c.downloadManuscriptAssets(ctx, blocks)
	if err != nil {
		document.ErrorMessage = err.Error()
		return document
	}
	document.Blocks = blocks
	document.Assets = assets
	document.Content = manuscriptPlainText(blocks)
	document.ReferenceNoteIDs = manuscriptReferenceNoteIDs(blocks)
	document.Status = documentSucceeded
	document.ErrorMessage = ""
	return document
}

func (c *Client) listManuscriptBlocks(ctx context.Context, documentToken string, revisionID int) ([]*larkdocx.Block, error) {
	items := make([]*larkdocx.Block, 0, manuscriptBlockPageSize)
	pageToken := ""
	for {
		if err := waitForDocxRequest(ctx); err != nil {
			return nil, err
		}
		builder := larkdocx.NewListDocumentBlockReqBuilder().
			DocumentId(documentToken).
			PageSize(manuscriptBlockPageSize)
		if revisionID > 0 {
			builder.DocumentRevisionId(revisionID)
		}
		if pageToken != "" {
			builder.PageToken(pageToken)
		}
		resp, err := c.client.Docx.DocumentBlock.List(ctx, builder.Build())
		if err != nil {
			return nil, err
		}
		if !resp.Success() {
			return nil, fmt.Errorf("code=%d message=%s", resp.Code, resp.Msg)
		}
		if resp.Data == nil {
			return nil, errors.New("empty block response")
		}
		items = append(items, resp.Data.Items...)
		if resp.Data.HasMore == nil || !*resp.Data.HasMore {
			return items, nil
		}
		if resp.Data.PageToken == nil || *resp.Data.PageToken == "" {
			return nil, errors.New("block response has_more is true but page_token is empty")
		}
		pageToken = *resp.Data.PageToken
	}
}

func parseManuscriptBlocks(items []*larkdocx.Block) []model.ManuscriptBlock {
	ordered := orderedDocumentBlocks(items)
	blocks := make([]model.ManuscriptBlock, 0, len(ordered))
	for _, item := range ordered {
		if item == nil {
			continue
		}
		if text, blockType, level := manuscriptTextBlock(item); text != nil {
			blocks = append(blocks, model.ManuscriptBlock{
				Type: blockType, Text: manuscriptText(text), Level: level,
				ReferenceNoteIDs: manuscriptTextReferenceNoteIDs(text),
			})
			continue
		}
		if item.Image != nil && item.Image.Token != nil && *item.Image.Token != "" {
			block := model.ManuscriptBlock{Type: "image", SourceToken: *item.Image.Token}
			if item.Image.Width != nil && *item.Image.Width > 0 {
				block.Width = *item.Image.Width
			}
			if item.Image.Height != nil && *item.Image.Height > 0 {
				block.Height = *item.Image.Height
			}
			if item.Image.Caption != nil && item.Image.Caption.Content != nil {
				block.Caption = strings.TrimSpace(*item.Image.Caption.Content)
			}
			blocks = append(blocks, block)
			continue
		}
		if item.Divider != nil {
			blocks = append(blocks, model.ManuscriptBlock{Type: "divider"})
		}
	}
	return blocks
}

func orderedDocumentBlocks(items []*larkdocx.Block) []*larkdocx.Block {
	byID := make(map[string]*larkdocx.Block, len(items))
	for _, item := range items {
		if item != nil && item.BlockId != nil && *item.BlockId != "" {
			byID[*item.BlockId] = item
		}
	}
	ordered := make([]*larkdocx.Block, 0, len(items))
	visited := make(map[string]struct{}, len(items))
	var walk func(string)
	walk = func(blockID string) {
		if _, ok := visited[blockID]; ok {
			return
		}
		item, ok := byID[blockID]
		if !ok {
			return
		}
		visited[blockID] = struct{}{}
		if item.Page == nil {
			ordered = append(ordered, item)
		}
		for _, childID := range item.Children {
			walk(childID)
		}
	}
	for _, item := range items {
		if item != nil && item.Page != nil {
			for _, childID := range item.Children {
				walk(childID)
			}
		}
	}
	for _, item := range items {
		if item == nil || item.BlockId == nil || *item.BlockId == "" {
			continue
		}
		walk(*item.BlockId)
	}
	return ordered
}

func manuscriptTextBlock(block *larkdocx.Block) (*larkdocx.Text, string, int) {
	switch {
	case block.Text != nil:
		return block.Text, "paragraph", 0
	case block.Heading1 != nil:
		return block.Heading1, "heading", 1
	case block.Heading2 != nil:
		return block.Heading2, "heading", 2
	case block.Heading3 != nil:
		return block.Heading3, "heading", 3
	case block.Heading4 != nil:
		return block.Heading4, "heading", 4
	case block.Heading5 != nil:
		return block.Heading5, "heading", 5
	case block.Heading6 != nil:
		return block.Heading6, "heading", 6
	case block.Heading7 != nil:
		return block.Heading7, "heading", 7
	case block.Heading8 != nil:
		return block.Heading8, "heading", 8
	case block.Heading9 != nil:
		return block.Heading9, "heading", 9
	case block.Bullet != nil:
		return block.Bullet, "bullet", 0
	case block.Ordered != nil:
		return block.Ordered, "ordered", 0
	case block.Quote != nil:
		return block.Quote, "quote", 0
	case block.Code != nil:
		return block.Code, "code", 0
	case block.Todo != nil:
		return block.Todo, "todo", 0
	case block.Equation != nil:
		return block.Equation, "equation", 0
	default:
		return nil, "", 0
	}
}

func manuscriptText(text *larkdocx.Text) string {
	if text == nil {
		return ""
	}
	var content strings.Builder
	for _, element := range text.Elements {
		if element != nil && element.TextRun != nil && element.TextRun.Content != nil {
			content.WriteString(*element.TextRun.Content)
		}
	}
	return strings.TrimSpace(content.String())
}

func manuscriptTextReferenceNoteIDs(text *larkdocx.Text) []string {
	if text == nil {
		return nil
	}
	values := []string{manuscriptText(text)}
	for _, element := range text.Elements {
		if element == nil || element.TextRun == nil || element.TextRun.TextElementStyle == nil ||
			element.TextRun.TextElementStyle.Link == nil || element.TextRun.TextElementStyle.Link.Url == nil {
			continue
		}
		values = append(values, *element.TextRun.TextElementStyle.Link.Url)
	}
	return extractReferenceNoteIDs(values...)
}

func manuscriptReferenceNoteIDs(blocks []model.ManuscriptBlock) []string {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, block := range blocks {
		for _, id := range block.ReferenceNoteIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}

func excludeReferenceNoteID(ids []string, noteID string) []string {
	excluded := strings.ToLower(strings.TrimSpace(noteID))
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if strings.ToLower(id) == excluded {
			continue
		}
		filtered = append(filtered, id)
	}
	return filtered
}

func extractReferenceNoteIDs(values ...string) []string {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, value := range values {
		value = decodeReferenceURL(value)
		for _, match := range xiaohongshuNoteURLPattern.FindAllStringSubmatch(value, -1) {
			if len(match) < 2 {
				continue
			}
			id := strings.ToLower(match[1])
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}

func decodeReferenceURL(value string) string {
	for range 3 {
		decoded, err := url.QueryUnescape(value)
		if err != nil || decoded == value {
			break
		}
		value = decoded
	}
	return value
}

func selectFinalManuscriptBlocks(blocks []model.ManuscriptBlock) []model.ManuscriptBlock {
	markerIndex := -1
	markerLevel := 0
	for index, block := range blocks {
		if isFinalSectionBlock(block) {
			markerIndex = index
			markerLevel = block.Level
		}
	}
	if markerIndex < 0 {
		return blocks
	}
	end := len(blocks)
	if markerLevel > 0 {
		for index := markerIndex + 1; index < len(blocks); index++ {
			if blocks[index].Type == "heading" && blocks[index].Level > 0 && blocks[index].Level <= markerLevel {
				end = index
				break
			}
		}
	}
	selected := append([]model.ManuscriptBlock(nil), blocks[markerIndex+1:end]...)
	for len(selected) > 0 && manuscriptBlockEmpty(selected[0]) {
		selected = selected[1:]
	}
	for len(selected) > 0 && manuscriptBlockEmpty(selected[len(selected)-1]) {
		selected = selected[:len(selected)-1]
	}
	return selected
}

func normalizeManuscriptSection(value string) string {
	return strings.Map(func(character rune) rune {
		if strings.ContainsRune(" \t\r\n:：#【】[]()（）_-—,，.。、;；", character) {
			return -1
		}
		return character
	}, strings.TrimSpace(value))
}

func isFinalSectionBlock(block model.ManuscriptBlock) bool {
	if isFinalSectionMarker(block.Text) {
		return true
	}
	normalized := trimManuscriptSectionPrefix(normalizeManuscriptSection(block.Text))
	if len([]rune(normalized)) > 24 {
		return false
	}
	if block.Type == "heading" {
		return strings.Contains(normalized, "定稿") || strings.Contains(normalized, "终稿")
	}
	if block.Type != "paragraph" {
		return false
	}
	return strings.HasPrefix(normalized, "定稿") || strings.HasPrefix(normalized, "终稿") ||
		strings.HasPrefix(normalized, "最终稿")
}

func trimManuscriptSectionPrefix(value string) string {
	value = strings.TrimPrefix(value, "版本")
	value = strings.TrimPrefix(value, "第")
	value = strings.TrimLeft(value, "0123456789一二三四五六七八九十")
	for _, prefix := range []string{"部分", "章节", "章", "节"} {
		value = strings.TrimPrefix(value, prefix)
	}
	return value
}

func isFinalSectionMarker(value string) bool {
	normalized := normalizeManuscriptSection(value)
	switch normalized {
	case "定稿", "定稿正文", "定稿内容", "定稿版本", "终稿", "终稿正文", "终稿内容", "最终稿", "最终稿正文", "最终稿内容", "最终版本":
		return true
	default:
		return false
	}
}

func manuscriptBlockEmpty(block model.ManuscriptBlock) bool {
	return block.Type != "image" && block.Type != "divider" && strings.TrimSpace(block.Text) == ""
}

func (c *Client) downloadManuscriptAssets(ctx context.Context, blocks []model.ManuscriptBlock) ([]model.ManuscriptBlock, []model.ManuscriptAsset, error) {
	assets := make([]model.ManuscriptAsset, 0)
	assetByToken := make(map[string]model.ManuscriptAsset)
	var totalBytes int64
	for index := range blocks {
		if blocks[index].Type != "image" {
			continue
		}
		token := blocks[index].SourceToken
		if token == "" {
			return nil, nil, fmt.Errorf("download Docx image %d: image token is empty", index+1)
		}
		asset, ok := assetByToken[token]
		if !ok {
			var err error
			asset, err = c.downloadManuscriptAsset(ctx, token, blocks[index].Width, blocks[index].Height)
			if err != nil {
				return nil, nil, fmt.Errorf("download Docx image %d: %w", index+1, err)
			}
			totalBytes += asset.ByteSize
			if totalBytes > maxManuscriptAssetBytes {
				return nil, nil, fmt.Errorf("Docx images exceed %d MB", maxManuscriptAssetBytes/1024/1024)
			}
			assetByToken[token] = asset
			assets = append(assets, asset)
		}
		blocks[index].AssetID = asset.AssetID
		blocks[index].SourceToken = ""
	}
	return blocks, assets, nil
}

func (c *Client) downloadManuscriptAsset(ctx context.Context, token string, width, height int) (model.ManuscriptAsset, error) {
	if err := waitForDocxRequest(ctx); err != nil {
		return model.ManuscriptAsset{}, err
	}
	resp, err := c.client.Drive.Media.Download(ctx, larkdrive.NewDownloadMediaReqBuilder().FileToken(token).Build())
	if err != nil {
		return model.ManuscriptAsset{}, err
	}
	if !resp.Success() {
		return model.ManuscriptAsset{}, fmt.Errorf("code=%d message=%s", resp.Code, resp.Msg)
	}
	if resp.File == nil {
		return model.ManuscriptAsset{}, errors.New("empty media response")
	}
	content, err := io.ReadAll(io.LimitReader(resp.File, maxManuscriptDownloadBytes+1))
	if err != nil {
		return model.ManuscriptAsset{}, err
	}
	if len(content) == 0 {
		return model.ManuscriptAsset{}, errors.New("image content is empty")
	}
	if len(content) > maxManuscriptDownloadBytes {
		return model.ManuscriptAsset{}, fmt.Errorf("source image exceeds %d MB", maxManuscriptDownloadBytes/1024/1024)
	}
	contentType := http.DetectContentType(content)
	if !supportedManuscriptImageType(contentType) {
		return model.ManuscriptAsset{}, fmt.Errorf("unsupported image content type %q", contentType)
	}
	content, contentType, err = normalizeManuscriptImage(content, contentType)
	if err != nil {
		return model.ManuscriptAsset{}, err
	}
	digest := sha256.Sum256(content)
	return model.ManuscriptAsset{
		AssetID: hex.EncodeToString(digest[:]), ContentType: contentType,
		ByteSize: int64(len(content)), Width: width, Height: height, Content: content,
	}, nil
}

func normalizeManuscriptImage(content []byte, contentType string) ([]byte, string, error) {
	if contentType != "image/jpeg" && contentType != "image/png" {
		if len(content) > maxManuscriptImageBytes {
			return nil, "", fmt.Errorf("%s image exceeds stored limit of %d MB", contentType, maxManuscriptImageBytes/1024/1024)
		}
		return content, contentType, nil
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return nil, "", fmt.Errorf("decode image dimensions: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, "", errors.New("image dimensions are invalid")
	}
	if int64(config.Width)*int64(config.Height) > manuscriptImageMaxPixels {
		return nil, "", fmt.Errorf("image dimensions exceed %d megapixels", manuscriptImageMaxPixels/1_000_000)
	}
	if len(content) <= manuscriptImageOptimizeBytes &&
		config.Width <= manuscriptImageMaxDimension && config.Height <= manuscriptImageMaxDimension {
		return content, contentType, nil
	}
	source, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}
	targetWidth, targetHeight := config.Width, config.Height
	if targetWidth > manuscriptImageMaxDimension || targetHeight > manuscriptImageMaxDimension {
		if targetWidth >= targetHeight {
			targetHeight = max(1, targetHeight*manuscriptImageMaxDimension/targetWidth)
			targetWidth = manuscriptImageMaxDimension
		} else {
			targetWidth = max(1, targetWidth*manuscriptImageMaxDimension/targetHeight)
			targetHeight = manuscriptImageMaxDimension
		}
	}
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	stddraw.Draw(target, target.Bounds(), image.NewUniform(color.White), image.Point{}, stddraw.Src)
	xdraw.CatmullRom.Scale(target, target.Bounds(), source, source.Bounds(), xdraw.Over, nil)
	var optimized bytes.Buffer
	if err := jpeg.Encode(&optimized, target, &jpeg.Options{Quality: manuscriptImageJPEGQuality}); err != nil {
		return nil, "", fmt.Errorf("encode optimized image: %w", err)
	}
	if optimized.Len() > maxManuscriptImageBytes {
		return nil, "", fmt.Errorf("optimized image exceeds stored limit of %d MB", maxManuscriptImageBytes/1024/1024)
	}
	return optimized.Bytes(), "image/jpeg", nil
}

func supportedManuscriptImageType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func manuscriptPlainText(blocks []model.ManuscriptBlock) string {
	lines := make([]string, 0, len(blocks))
	for _, block := range blocks {
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		switch block.Type {
		case "bullet", "todo":
			text = "- " + text
		case "ordered":
			text = "1. " + text
		case "quote":
			text = "> " + text
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}
