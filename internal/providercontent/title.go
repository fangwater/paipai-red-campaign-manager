package providercontent

import "strings"

const youyiyouerProviderCode = "youyiyouer"

// SourceTitle resolves the user-facing manuscript title for a provider note.
func SourceTitle(providerCode, content, linkTitle, documentTitle string) string {
	if providerCode == youyiyouerProviderCode {
		if title := manuscriptTitle(content); title != "" {
			return title
		}
	}
	for _, title := range []string{linkTitle, documentTitle} {
		if title = strings.TrimSpace(title); title != "" {
			return title
		}
	}
	return ""
}

func manuscriptTitle(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for index, line := range lines {
		value, ok := titleFieldValue(line)
		if !ok {
			continue
		}
		if value != "" {
			return value
		}
		for _, followingLine := range lines[index+1:] {
			followingLine = strings.TrimSpace(followingLine)
			if followingLine == "" {
				continue
			}
			if isContentSection(followingLine) {
				break
			}
			return followingLine
		}
	}
	return ""
}

func titleFieldValue(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "标题") {
		return "", false
	}
	remainder := strings.TrimSpace(strings.TrimPrefix(line, "标题"))
	if remainder == "" || (remainder[0] != ':' && !strings.HasPrefix(remainder, "：")) {
		return "", false
	}
	if remainder[0] == ':' {
		return strings.TrimSpace(remainder[1:]), true
	}
	return strings.TrimSpace(strings.TrimPrefix(remainder, "：")), true
}

func isContentSection(line string) bool {
	for _, prefix := range []string{"正文", "文案", "内容", "封面", "备注", "对标"} {
		remainder := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if remainder != line && (strings.HasPrefix(remainder, ":") || strings.HasPrefix(remainder, "：")) {
			return true
		}
	}
	return false
}
