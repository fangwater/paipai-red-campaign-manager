package lark

import (
	"testing"

	"paipai-red-campaign-manager/internal/model"
)

func TestMillisecondsToTime(t *testing.T) {
	value := int64(1_700_000_000_123)
	got := millisecondsToTime(&value)
	if got == nil || got.UnixMilli() != value {
		t.Fatalf("millisecondsToTime() = %v, want %d", got, value)
	}
}

func TestMillisecondsToTimeEmpty(t *testing.T) {
	zero := int64(0)
	if got := millisecondsToTime(nil); got != nil {
		t.Fatalf("millisecondsToTime(nil) = %v, want nil", got)
	}
	if got := millisecondsToTime(&zero); got != nil {
		t.Fatalf("millisecondsToTime(0) = %v, want nil", got)
	}
}

func TestParseDocumentLink(t *testing.T) {
	tests := []struct {
		name         string
		link         string
		wantProvider string
		wantKey      string
		wantOK       bool
	}{
		{
			name:         "Docx",
			link:         "https://example.feishu.cn/docx/doxcn123?from=copy",
			wantProvider: "feishu",
			wantKey:      "docx:doxcn123",
			wantOK:       true,
		},
		{
			name:         "Wiki",
			link:         "https://example.feishu.cn/wiki/wikcn123",
			wantProvider: "feishu",
			wantKey:      "wiki:wikcn123",
			wantOK:       true,
		},
		{
			name:   "Xiaohongshu is not a document",
			link:   "https://www.xiaohongshu.com/explore/123",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, key, ok := parseDocumentLink(tt.link)
			if provider != tt.wantProvider || key != tt.wantKey || ok != tt.wantOK {
				t.Fatalf("parseDocumentLink() = (%q, %q, %v), want (%q, %q, %v)",
					provider, key, ok, tt.wantProvider, tt.wantKey, tt.wantOK)
			}
		})
	}
}

func TestParseTencentDocumentLink(t *testing.T) {
	tests := []struct {
		link     string
		provider string
	}{
		{link: "https://doc.weixin.qq.com/sheet/e3_example?tab=1", provider: "weixin"},
		{link: "https://doc.weixin.qq.com/doc/e3_example", provider: "weixin"},
		{link: "https://docs.qq.com/sheet/example", provider: "tencent"},
		{link: "https://docs.qq.com/doc/example", provider: "tencent"},
	}
	for _, tt := range tests {
		t.Run(tt.provider+"_"+tt.link, func(t *testing.T) {
			provider, key, ok := parseDocumentLink(tt.link)
			if !ok || provider != tt.provider || len(key) != 64 {
				t.Fatalf("parseDocumentLink() = (%q, %q, %v), want %s SHA-256 key",
					provider, key, ok, tt.provider)
			}
		})
	}
}

func TestFetchTencentDocumentDoesNotFetchContent(t *testing.T) {
	client := &Client{}
	documents, err := client.FetchDocuments(t.Context(), []model.DocumentRef{{
		Provider: "weixin", ResourceKey: "key", SourceURL: "https://doc.weixin.qq.com/sheet/example",
	}})
	if err != nil {
		t.Fatalf("FetchDocuments() error = %v", err)
	}
	if len(documents) != 1 || documents[0].Content != "nan" || documents[0].Status != documentAuthNeeded {
		t.Fatalf("FetchDocuments() = %#v, want nan content with auth_required status", documents)
	}
}
