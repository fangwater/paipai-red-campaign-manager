package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/dandelion"
)

func TestImportDandelionExcelIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "dandelion-excel-test")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := time.Now().Format("20060102150405.000000000")
	firstID := "excel_integration_" + suffix + "_1"
	secondID := "excel_integration_" + suffix + "_2"
	runIDs := []int64{}
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM lark_bitable_records WHERE app_token=$1 AND table_id=$2 AND record_id=ANY($3)", dandelionExcelAppToken, dandelionExcelTableID, []string{firstID, secondID})
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM sync_runs WHERE id=ANY($1)", runIDs)
	}()
	date := time.Date(2099, 1, 1, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	first := dandelion.Snapshot{
		FileName: "first.xlsx", FileSHA256: suffix + "-first", SheetName: "蒲公英", HeaderRow: 1,
		Records: []dandelion.Record{{
			RecordID: firstID, SourceRow: 2, NoteID: suffix, DataUpdated: date,
			Fields: []byte(fmt.Sprintf(`{"笔记ID":[{"text":%q}],"数据更新日期":%d,"笔记标题":[{"text":"第一版"}]}`, suffix, date.UnixMilli())),
		}},
	}
	firstResult, err := postgres.ImportDandelionExcel(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	runIDs = append(runIDs, firstResult.RunID)
	if firstResult.Inserted != 1 || firstResult.Updated != 0 || firstResult.Unchanged != 0 || firstResult.Deleted != 0 {
		t.Fatalf("first result = %+v", firstResult)
	}
	unchanged, err := postgres.ImportDandelionExcel(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	runIDs = append(runIDs, unchanged.RunID)
	if unchanged.Inserted != 0 || unchanged.Updated != 0 || unchanged.Unchanged != 1 {
		t.Fatalf("unchanged result = %+v", unchanged)
	}
	changed := first
	changed.FileName = "changed.xlsx"
	changed.FileSHA256 = suffix + "-changed"
	changed.Records[0].Fields = []byte(fmt.Sprintf(`{"笔记ID":[{"text":%q}],"数据更新日期":%d,"笔记标题":[{"text":"第二版"}]}`, suffix, date.UnixMilli()))
	changed.Records = append(changed.Records, dandelion.Record{
		RecordID: secondID, SourceRow: 3, NoteID: suffix + "-2", DataUpdated: date,
		Fields: []byte(fmt.Sprintf(`{"笔记ID":[{"text":%q}],"数据更新日期":%d}`, suffix+"-2", date.UnixMilli())),
	})
	changedResult, err := postgres.ImportDandelionExcel(ctx, changed)
	if err != nil {
		t.Fatal(err)
	}
	runIDs = append(runIDs, changedResult.RunID)
	if changedResult.Inserted != 1 || changedResult.Updated != 1 || changedResult.Unchanged != 0 || changedResult.Deleted != 0 {
		t.Fatalf("changed result = %+v", changedResult)
	}
	var title string
	if err := postgres.pool.QueryRow(ctx, `
		SELECT fields #>> '{笔记标题,0,text}' FROM lark_bitable_records
		WHERE app_token=$1 AND table_id=$2 AND record_id=$3 AND deleted_at IS NULL
	`, dandelionExcelAppToken, dandelionExcelTableID, firstID).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "第二版" {
		t.Fatalf("title = %q", title)
	}
}
