package lark

import (
	"bytes"
	"image"
	"image/png"
	"net/url"
	"slices"
	"testing"

	"paipai-red-campaign-manager/internal/model"

	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

func TestSelectFinalManuscriptBlocksKeepsOnlyFinalSectionInDocumentOrder(t *testing.T) {
	rootID := "root"
	root := &larkdocx.Block{
		BlockId: &rootID,
		Page:    manuscriptTestText("文档标题"),
		Children: []string{
			"draft-heading", "draft-text", "final-heading", "final-text", "final-image", "notes-heading", "notes-text",
		},
	}
	items := []*larkdocx.Block{
		root,
		manuscriptTestBlock("final-image", &rootID, func(block *larkdocx.Block) {
			token, width, height := "image-token", 1080, 1440
			block.Image = &larkdocx.Image{Token: &token, Width: &width, Height: &height}
		}),
		manuscriptTestBlock("draft-heading", &rootID, func(block *larkdocx.Block) { block.Heading1 = manuscriptTestText("初稿") }),
		manuscriptTestBlock("draft-text", &rootID, func(block *larkdocx.Block) { block.Text = manuscriptTestText("不应保留的初稿") }),
		manuscriptTestBlock("final-heading", &rootID, func(block *larkdocx.Block) { block.Heading1 = manuscriptTestText("二、定稿（修改后）") }),
		manuscriptTestBlock("final-text", &rootID, func(block *larkdocx.Block) { block.Text = manuscriptTestText("最终正文") }),
		manuscriptTestBlock("notes-heading", &rootID, func(block *larkdocx.Block) { block.Heading1 = manuscriptTestText("备注") }),
		manuscriptTestBlock("notes-text", &rootID, func(block *larkdocx.Block) { block.Text = manuscriptTestText("不属于定稿") }),
	}

	blocks := selectFinalManuscriptBlocks(parseManuscriptBlocks(items))
	if len(blocks) != 2 {
		t.Fatalf("selected blocks = %+v", blocks)
	}
	if blocks[0].Type != "paragraph" || blocks[0].Text != "最终正文" {
		t.Fatalf("first selected block = %+v", blocks[0])
	}
	if blocks[1].Type != "image" || blocks[1].SourceToken != "image-token" || blocks[1].Width != 1080 || blocks[1].Height != 1440 {
		t.Fatalf("image block = %+v", blocks[1])
	}
	if got := manuscriptPlainText(blocks); got != "最终正文" {
		t.Fatalf("plain text = %q", got)
	}
}

func TestSelectFinalManuscriptBlocksKeepsUnversionedDocument(t *testing.T) {
	blocks := []model.ManuscriptBlock{{Type: "paragraph", Text: "唯一稿件"}, {Type: "image", SourceToken: "token"}}
	selected := selectFinalManuscriptBlocks(blocks)
	if len(selected) != 2 || selected[0].Text != "唯一稿件" || selected[1].SourceToken != "token" {
		t.Fatalf("selected blocks = %+v", selected)
	}
}

func TestSelectFinalManuscriptBlocksRecognizesNumberedParagraphMarker(t *testing.T) {
	blocks := []model.ManuscriptBlock{
		{Type: "paragraph", Text: "初稿正文"},
		{Type: "paragraph", Text: "二、定稿（修改后）"},
		{Type: "paragraph", Text: "确认后的正文"},
		{Type: "image", SourceToken: "final-image"},
	}
	selected := selectFinalManuscriptBlocks(blocks)
	if len(selected) != 2 || selected[0].Text != "确认后的正文" || selected[1].SourceToken != "final-image" {
		t.Fatalf("selected blocks = %+v", selected)
	}
	if isFinalSectionBlock(model.ManuscriptBlock{Type: "paragraph", Text: "这是最终稿"}) {
		t.Fatal("ordinary manuscript text must not be treated as a final-section marker")
	}
}

func TestManuscriptTextReferenceNoteIDsRecognizesCompleteDetailLinks(t *testing.T) {
	firstID := "69b1039d00000000080316ae"
	secondID := "6761604b000000001301beca"
	firstURL := "https://www.xiaohongshu.com/explore/" + firstID + "?xsec_source=pc_search"
	secondURL := "https://www.xiaohongshu.com/discovery/item/" + secondID + "?source=webshare"
	encodedSecondURL := url.QueryEscape(secondURL)
	encodedSearchURL := url.QueryEscape("https://www.xiaohongshu.com/search_result?keyword=test")
	visible := "参考：" + firstURL + "，"
	linkedLabel := "参考案例"
	searchLabel := "话题标签"
	malformed := "https://www.xiaohongshu.com/discovery/item/6a3babd10000000 006021ff7"
	text := &larkdocx.Text{Elements: []*larkdocx.TextElement{
		{TextRun: &larkdocx.TextRun{Content: &visible}},
		{TextRun: &larkdocx.TextRun{Content: &linkedLabel, TextElementStyle: &larkdocx.TextElementStyle{
			Link: &larkdocx.Link{Url: &encodedSecondURL},
		}}},
		{TextRun: &larkdocx.TextRun{Content: &searchLabel, TextElementStyle: &larkdocx.TextElementStyle{
			Link: &larkdocx.Link{Url: &encodedSearchURL},
		}}},
		{TextRun: &larkdocx.TextRun{Content: &malformed}},
	}}

	got := manuscriptTextReferenceNoteIDs(text)
	want := []string{firstID, secondID}
	if !slices.Equal(got, want) {
		t.Fatalf("reference note IDs = %v, want %v", got, want)
	}
}

func TestManuscriptReferenceNoteIDsOnlyUsesSelectedFinalSection(t *testing.T) {
	draftID := "69b1039d00000000080316ae"
	finalID := "6761604b000000001301beca"
	notesID := "697b195d000000000b0125b6"
	blocks := []model.ManuscriptBlock{
		{Type: "heading", Text: "初稿", Level: 1},
		{Type: "paragraph", Text: "初稿参考", ReferenceNoteIDs: []string{draftID}},
		{Type: "heading", Text: "定稿", Level: 1},
		{Type: "paragraph", Text: "最终参考", ReferenceNoteIDs: []string{finalID, finalID}},
		{Type: "heading", Text: "备注", Level: 1},
		{Type: "paragraph", Text: "备注参考", ReferenceNoteIDs: []string{notesID}},
	}

	got := manuscriptReferenceNoteIDs(selectFinalManuscriptBlocks(blocks))
	if !slices.Equal(got, []string{finalID}) {
		t.Fatalf("reference note IDs = %v, want [%s]", got, finalID)
	}
}

func TestExcludeReferenceNoteIDRemovesCurrentManuscript(t *testing.T) {
	currentID := "6a33d5aa000000001c024f8a"
	referenceID := "69b1039d00000000080316ae"
	got := excludeReferenceNoteID([]string{currentID, referenceID}, currentID)
	if !slices.Equal(got, []string{referenceID}) {
		t.Fatalf("reference note IDs = %v", got)
	}
}

func TestSupportedManuscriptImageTypeRejectsActiveContent(t *testing.T) {
	for _, contentType := range []string{"image/jpeg", "image/png", "image/webp", "image/gif"} {
		if !supportedManuscriptImageType(contentType) {
			t.Fatalf("content type %q should be supported", contentType)
		}
	}
	if supportedManuscriptImageType("image/svg+xml") {
		t.Fatal("SVG must not be served as an inline manuscript image")
	}
}

func TestNormalizeManuscriptImageDownscalesLargePNG(t *testing.T) {
	var source bytes.Buffer
	if err := png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 3000, 120))); err != nil {
		t.Fatal(err)
	}
	optimized, contentType, err := normalizeManuscriptImage(source.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/jpeg" || bytes.Equal(optimized, source.Bytes()) {
		t.Fatalf("content type=%q optimized=%v", contentType, !bytes.Equal(optimized, source.Bytes()))
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(optimized))
	if err != nil {
		t.Fatal(err)
	}
	if format != "jpeg" || config.Width != manuscriptImageMaxDimension || config.Height != 102 {
		t.Fatalf("optimized format=%s dimensions=%dx%d", format, config.Width, config.Height)
	}
}

func TestNormalizeManuscriptImagePreservesSmallPNG(t *testing.T) {
	var source bytes.Buffer
	if err := png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 20, 20))); err != nil {
		t.Fatal(err)
	}
	content, contentType, err := normalizeManuscriptImage(source.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/png" || !bytes.Equal(content, source.Bytes()) {
		t.Fatalf("small image was modified: content type=%q", contentType)
	}
}

func manuscriptTestBlock(id string, parentID *string, fill func(*larkdocx.Block)) *larkdocx.Block {
	block := &larkdocx.Block{BlockId: &id, ParentId: parentID}
	fill(block)
	return block
}

func manuscriptTestText(content string) *larkdocx.Text {
	return &larkdocx.Text{Elements: []*larkdocx.TextElement{{
		TextRun: &larkdocx.TextRun{Content: &content},
	}}}
}
