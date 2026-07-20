package lark

import (
	"strings"
	"testing"
)

func TestFindProviderColumnsAndRecord(t *testing.T) {
	headers := []interface{}{
		"无关字段", " 提交日期 ", "笔记 ID", "内容类型", "封面类型", "商业强度",
		"对话人群", "用户场景", "笔记类型", "进度", "审核反馈",
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
	if minColumn != 1 || maxColumn != 10 {
		t.Fatalf("column bounds = (%d, %d), want (1, 10)", minColumn, maxColumn)
	}

	record := providerRecord([]interface{}{
		"2026-07-19", "note-1", "测评", "单图", "弱", "新客", "通勤",
		"达人笔记", "已提交", "通过",
	}, columns, minColumn, 3)
	if record.RecordKey != "row:3" || record.NoteID != "note-1" || record.ReviewFeedback != "通过" {
		t.Fatalf("providerRecord() = %+v", record)
	}
}

func TestFindProviderColumnsReportsMissingFields(t *testing.T) {
	_, _, err := findProviderColumns([][]interface{}{{"提交日期", "笔记ID"}})
	if err == nil || !strings.Contains(err.Error(), "内容类型") {
		t.Fatalf("findProviderColumns() error = %v, want missing 内容类型", err)
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
