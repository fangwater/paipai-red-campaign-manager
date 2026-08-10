package providercontent

import "testing"

func TestSourceTitle(t *testing.T) {
	tests := []struct {
		name          string
		providerCode  string
		content       string
		linkTitle     string
		documentTitle string
		want          string
	}{
		{
			name:         "youyiyouer full-width colon",
			providerCode: "youyiyouer",
			content:      "对标：https://example.com/note\n标题： 吃了两个月，说点大实话...\n正文：测试正文",
			linkTitle:    "脉拓辅酶q10-职场人-Chris-0706",
			want:         "吃了两个月，说点大实话...",
		},
		{
			name:         "youyiyouer ASCII colon",
			providerCode: "youyiyouer",
			content:      "标题:高强度打工人如何保持运动习惯？\n文案:测试正文",
			linkTitle:    "内部稿件名",
			want:         "高强度打工人如何保持运动习惯？",
		},
		{
			name:         "youyiyouer title on following line",
			providerCode: "youyiyouer",
			content:      "标题：\n\n全世界熬夜心慌的打工人都给我吻上来!!!!!\n正文：测试正文",
			linkTitle:    "内部稿件名",
			want:         "全世界熬夜心慌的打工人都给我吻上来!!!!!",
		},
		{
			name:          "youyiyouer missing title falls back",
			providerCode:  "youyiyouer",
			content:       "标题：\n正文：测试正文",
			linkTitle:     "内部稿件名",
			documentTitle: "飞书文档标题",
			want:          "内部稿件名",
		},
		{
			name:          "other provider keeps link title",
			providerCode:  "manjie",
			content:       "标题：正文里的标题",
			linkTitle:     "链接标题",
			documentTitle: "飞书文档标题",
			want:          "链接标题",
		},
		{
			name:          "document title is final fallback",
			providerCode:  "manjie",
			content:       "正文",
			linkTitle:     " ",
			documentTitle: " 飞书文档标题 ",
			want:          "飞书文档标题",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SourceTitle(test.providerCode, test.content, test.linkTitle, test.documentTitle); got != test.want {
				t.Fatalf("SourceTitle() = %q, want %q", got, test.want)
			}
		})
	}
}
