package lark

import (
	"strings"
	"testing"

	"paipai-red-campaign-manager/internal/model"
)

func TestFindProviderColumnsAndRecord(t *testing.T) {
	headers := []interface{}{
		"无关字段", " 提交日期 ", "笔记 ID", "笔记类型", "稿件", "封面类型", "商业强度",
		"对话人群", "用户场景", "进度",
	}
	headerRow, columns, err := findProviderColumns([][]interface{}{
		{"曼杰达人笔记执行表"},
		headers,
	})
	if err != nil {
		t.Fatalf("findProviderColumns() error = %v", err)
	}
	if headerRow != 1 {
		t.Fatalf("header row = %d, want 1", headerRow)
	}
	minColumn, maxColumn := providerColumnBounds(columns)
	if minColumn != 1 || maxColumn != 9 {
		t.Fatalf("column bounds = (%d, %d), want (1, 9)", minColumn, maxColumn)
	}

	record := providerRecord([]interface{}{
		"2026-07-19", "6a59e5d700000000010016cf", "测评", "稿件标题", "单图", "弱", "新客", "通勤",
		"已提交",
	}, columns, minColumn, 3)
	if record.RecordKey != "row:3" || record.NoteID != "6a59e5d700000000010016cf" || record.NoteType != "测评" {
		t.Fatalf("providerRecord() = %+v", record)
	}
}

func TestFindProviderColumnsReportsMissingFields(t *testing.T) {
	_, _, err := findProviderColumns([][]interface{}{{"提交日期", "笔记ID"}})
	if err == nil || !strings.Contains(err.Error(), "内容类型") {
		t.Fatalf("findProviderColumns() error = %v, want missing 内容类型", err)
	}
}

func TestProviderRecordMapsContentTypeToNoteType(t *testing.T) {
	headers := []interface{}{
		"提交日期", "笔记ID", "内容类型", "稿件", "封面类型", "商业强度",
		"对话人群", "用户场景", "进度",
	}
	headerRow, columns, err := findProviderColumns([][]interface{}{headers})
	if err != nil {
		t.Fatalf("findProviderColumns() error = %v", err)
	}
	if headerRow != 0 {
		t.Fatalf("header row = %d, want 0", headerRow)
	}

	record := providerRecord([]interface{}{
		"2026-07-19", "6a59e5d700000000010016cf", "测评", "稿件标题", "单图", "弱", "新客", "通勤", "已提交",
	}, columns, 0, 2)
	if record.NoteType != "测评" {
		t.Fatalf("providerRecord() note type = %q, want %q", record.NoteType, "测评")
	}
}

func TestFindProviderColumnsSupportsYouyiyouerAliases(t *testing.T) {
	headers := []interface{}{
		"内容类型", "封面类型", "商业强度", "人群标签", "对应场景",
		"稿件", "笔记进度", "发布时间", "笔记id",
	}
	headerRow, columns, err := findProviderColumns([][]interface{}{headers})
	if err != nil {
		t.Fatalf("findProviderColumns() error = %v", err)
	}
	if headerRow != 0 {
		t.Fatalf("header row = %d, want 0", headerRow)
	}
	record := providerRecord([]interface{}{
		"科普", "信息图", "软广", "辅酶选购", "辅酶价值",
		"稿件标题", "已发布", "2026.7.13", "6a50dc74000000000f01599a",
	}, columns, 0, 2)
	if record.SubmissionDate != "2026.7.13" || record.NoteID != "6a50dc74000000000f01599a" ||
		record.NoteType != "科普" || record.Audience != "辅酶选购" ||
		record.UserScenario != "辅酶价值" || record.Progress != "已发布" {
		t.Fatalf("providerRecord() = %+v", record)
	}
}

func TestFindProviderColumnsSupportsZhiyuanAliases(t *testing.T) {
	headers := []interface{}{
		"人群标签", "对应场景", "封面类型", "商业强度", "内容类型",
		"稿件", "审核状态", "发布时间", "笔记id",
	}
	headerRow, columns, err := findProviderColumns([][]interface{}{headers})
	if err != nil {
		t.Fatalf("findProviderColumns() error = %v", err)
	}
	if headerRow != 0 {
		t.Fatalf("header row = %d, want 0", headerRow)
	}
	record := providerRecord([]interface{}{
		"职场人", "精力疲惫", "大字报", "软广", "经验分享",
		"稿件标题", "已通过", "2026.7.21", "6a59e5d700000000010016cf",
	}, columns, 0, 2)
	if record.SubmissionDate != "2026.7.21" || record.NoteID != "6a59e5d700000000010016cf" ||
		record.NoteType != "经验分享" || record.Audience != "职场人" ||
		record.UserScenario != "精力疲惫" || record.Progress != "已通过" {
		t.Fatalf("providerRecord() = %+v", record)
	}
}

func TestNormalizeProviderNoteIDRejectsUnconfirmedValues(t *testing.T) {
	for _, value := range []string{"", " 未发布 ", "待确认", "6a59e5d700000000010016c"} {
		if got := normalizeProviderNoteID(value); got != "" {
			t.Fatalf("normalizeProviderNoteID(%q) = %q, want empty", value, got)
		}
	}
	if got := normalizeProviderNoteID("6a59e5d700000000010016cf"); got != "6a59e5d700000000010016cf" {
		t.Fatalf("normalizeProviderNoteID() = %q", got)
	}
}

func TestProviderNoteDocumentRefAndContent(t *testing.T) {
	row := []interface{}{
		"6a59e5d700000000010016cf",
		[]interface{}{map[string]interface{}{
			"text": "稿件标题",
			"link": "https://example.feishu.cn/docx/doc-token",
		}},
	}
	ref, ok := providerNoteDocumentRef(row, map[string]int{"稿件": 1}, 0, "6a59e5d700000000010016cf")
	if !ok {
		t.Fatal("providerNoteDocumentRef() did not find the Feishu document link")
	}
	if ref.Provider != "feishu" || ref.ResourceKey != "docx:doc-token" {
		t.Fatalf("providerNoteDocumentRef() = %+v", ref)
	}

	notes, errorsCount := providerNotes([]model.DocumentRef{ref}, []model.Document{{
		Provider: ref.Provider, ResourceKey: ref.ResourceKey, Content: "笔记正文", Status: documentSucceeded,
	}})
	if errorsCount != 0 || len(notes) != 1 {
		t.Fatalf("providerNotes() notes=%+v errors=%d", notes, errorsCount)
	}
	if notes[0].NoteID != "6a59e5d700000000010016cf" || notes[0].NoteContent != "笔记正文" {
		t.Fatalf("providerNotes() = %+v", notes)
	}
}

func TestCellStringRichText(t *testing.T) {
	value := []interface{}{
		map[string]interface{}{"text": "达人"},
		map[string]interface{}{"text": "笔记"},
	}
	if got := cellString(value); got != "达人笔记" {
		t.Fatalf("cellString() = %q, want %q", got, "达人笔记")
	}
}

func TestColumnName(t *testing.T) {
	for index, want := range map[int]string{0: "A", 25: "Z", 26: "AA", 99: "CV"} {
		if got := columnName(index); got != want {
			t.Fatalf("columnName(%d) = %q, want %q", index, got, want)
		}
	}
}
