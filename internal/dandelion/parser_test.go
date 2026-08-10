package dandelion

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

var testHeaders = []interface{}{
	"笔记ID", "笔记标题", "笔记链接", "博主昵称", "笔记发布日期", "下单账号", "SPU名称", "数据更新日期",
	"笔记类型", "内容标签", "蒲公英金额", "站外活跃成本（15天设备归因）", "曝光量", "阅读量", "互动量", "阅读单价", "互动单价", "点赞量", "备注",
}

func testWorkbook(t *testing.T, headers []interface{}, rows ...[]interface{}) []byte {
	t.Helper()
	workbook := excelize.NewFile()
	sheet := workbook.GetSheetName(0)
	workbook.SetSheetName(sheet, "蒲公英导出")
	sheet = "蒲公英导出"
	if err := workbook.SetSheetRow(sheet, "A1", &[]interface{}{"导出数据"}); err != nil {
		t.Fatal(err)
	}
	if err := workbook.SetSheetRow(sheet, "A2", &headers); err != nil {
		t.Fatal(err)
	}
	for index, row := range rows {
		axis, _ := excelize.CoordinatesToCellName(1, index+3)
		if err := workbook.SetSheetRow(sheet, axis, &row); err != nil {
			t.Fatal(err)
		}
	}
	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	_ = workbook.Close()
	return buffer.Bytes()
}

func TestParseMapsExportHeadersToLarkFieldShape(t *testing.T) {
	data := testWorkbook(t, testHeaders, []interface{}{
		"0123456789abcdef01234567", "测试笔记", "https://example.com/note",
		"测试博主", "2026-08-01", "杭州智元文化传播有限公司", "辅酶", "2026-08-05",
		"科普", "成分", 3000, 18.5, 10000, 3200, 188, 0.94, 15.96, 99, "保留字段",
	})
	snapshot, err := Parse(bytes.NewReader(data), "蒲公英.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SheetName != "蒲公英导出" || snapshot.HeaderRow != 2 || len(snapshot.Records) != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if got := snapshot.ReportDate.Format("2006-01-02"); got != "2026-08-05" {
		t.Fatalf("report date = %q", got)
	}
	record := snapshot.Records[0]
	if !strings.HasPrefix(record.RecordID, "excel_") || record.NoteID != "0123456789abcdef01234567" {
		t.Fatalf("record = %+v", record)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(record.Fields, &fields); err != nil {
		t.Fatal(err)
	}
	author, ok := fields[FieldAuthor].([]interface{})
	if !ok || len(author) != 1 || author[0].(map[string]interface{})["text"] != "测试博主" {
		t.Fatalf("author field = %#v", fields[FieldAuthor])
	}
	if fields[FieldPublishedAt].(float64) != 1785513600000 ||
		fields[FieldDataUpdatedAt].(float64) != 1785859200000 {
		t.Fatalf("date fields = %#v / %#v", fields[FieldPublishedAt], fields[FieldDataUpdatedAt])
	}
	if fields[FieldOffsiteActiveCost].(float64) != 18.5 || fields[FieldImpressions].(float64) != 10000 {
		t.Fatalf("metric fields = %#v", fields)
	}
	if fields["点赞量"].(float64) != 99 || fields["备注"].(string) != "保留字段" {
		t.Fatalf("extra fields = %#v", fields)
	}
}

func TestParseReadsCoreFieldFromVerticalMergedHeader(t *testing.T) {
	workbook := excelize.NewFile()
	sheet := workbook.GetSheetName(0)
	workbook.SetSheetName(sheet, "笔记批量数据")
	sheet = "笔记批量数据"
	if err := workbook.SetCellValue(sheet, "A1", "数据更新日期"); err != nil {
		t.Fatal(err)
	}
	if err := workbook.MergeCell(sheet, "A1", "A3"); err != nil {
		t.Fatal(err)
	}
	headers := []interface{}{"笔记ID", "笔记标题", "笔记链接", "博主昵称", "笔记发布日期", "下单账号", "SPU名称"}
	if err := workbook.SetSheetRow(sheet, "B3", &headers); err != nil {
		t.Fatal(err)
	}
	row := []interface{}{
		"2026/08/04", "0123456789abcdef01234567", "测试笔记", "https://example.com/note",
		"测试博主", "2026-08-01", "杭州智元文化传播有限公司", "辅酶",
	}
	if err := workbook.SetSheetRow(sheet, "A4", &row); err != nil {
		t.Fatal(err)
	}
	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	_ = workbook.Close()

	snapshot, err := Parse(bytes.NewReader(buffer.Bytes()), "蒲公英.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.HeaderRow != 3 || len(snapshot.Records) != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(snapshot.Records[0].Fields, &fields); err != nil {
		t.Fatal(err)
	}
	if fields[FieldDataUpdatedAt].(float64) != 1785772800000 {
		t.Fatalf("data updated field = %#v", fields[FieldDataUpdatedAt])
	}
}

func TestParseRetainsRepeatedExtraHeaders(t *testing.T) {
	headers := append([]interface{}(nil), testHeaders...)
	headers = append(headers, "top1", "top1")
	row := []interface{}{
		"0123456789abcdef01234567", "测试笔记", "https://example.com/note",
		"测试博主", "2026-08-01", "杭州智元文化传播有限公司", "辅酶", "2026-08-05",
		"科普", "成分", 3000, 18.5, 10000, 3200, 188, 0.94, 15.96, 99, "保留字段", "北京", "美妆",
	}
	snapshot, err := Parse(bytes.NewReader(testWorkbook(t, headers, row)), "蒲公英.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(snapshot.Records[0].Fields, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["top1"] != "北京" || fields["top1（1）"] != "美妆" {
		t.Fatalf("repeated extra fields = %#v", fields)
	}
}

func TestParseRejectsMissingCoreHeader(t *testing.T) {
	headers := append([]interface{}(nil), testHeaders...)
	headers[3] = "未识别博主字段"
	_, err := Parse(bytes.NewReader(testWorkbook(t, headers, []interface{}{
		"0123456789abcdef01234567", "测试笔记", "https://example.com/note",
		"测试博主", "2026-08-01", "杭州智元文化传播有限公司", "辅酶", "2026-08-05",
	})), "蒲公英.xlsx")
	if err == nil || !strings.Contains(err.Error(), FieldAuthor) {
		t.Fatalf("error = %v", err)
	}
}

func TestParseRejectsDuplicateIncrementalKey(t *testing.T) {
	row := []interface{}{
		"0123456789abcdef01234567", "测试笔记", "https://example.com/note",
		"测试博主", "2026-08-01", "杭州智元文化传播有限公司", "辅酶", "2026-08-05",
	}
	_, err := Parse(bytes.NewReader(testWorkbook(t, testHeaders, row, row)), "蒲公英.xlsx")
	if err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("error = %v", err)
	}
}
