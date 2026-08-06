package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"paipai-red-campaign-manager/internal/dandelion"

	"github.com/jackc/pgx/v5"
)

const (
	dandelionExcelAppToken = "excel:dandelion"
	dandelionExcelTableID  = "dandelion_excel_upload"
)

var ErrDandelionExcelImportLocked = errors.New("another Dandelion Excel import is already running")

func (p *Postgres) ImportDandelionExcel(ctx context.Context, snapshot dandelion.Snapshot) (result dandelion.ImportResult, err error) {
	result = dandelion.ImportResult{
		FileName: snapshot.FileName, FileSHA256: snapshot.FileSHA256,
		SheetName: snapshot.SheetName, HeaderRow: snapshot.HeaderRow, Fetched: len(snapshot.Records),
	}
	if err := p.pool.QueryRow(ctx, `
		INSERT INTO sync_runs (app_token,table_id,status)
		VALUES ($1,$2,'running') RETURNING id
	`, dandelionExcelAppToken, dandelionExcelTableID).Scan(&result.RunID); err != nil {
		return result, fmt.Errorf("start Dandelion Excel import run: %w", err)
	}
	defer func() {
		finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		status, errorMessage := "succeeded", ""
		if err != nil {
			status, errorMessage = "failed", err.Error()
		}
		if status == "succeeded" {
			result.CompletedAt = time.Now().Format(time.RFC3339)
		}
		_, _ = p.pool.Exec(finishCtx, `
			UPDATE sync_runs SET status=$2,fetched_count=$3,upserted_count=$4,deleted_count=0,
				tables_count=1,completed_at=NOW(),error_message=NULLIF($5,'')
			WHERE id=$1
		`, result.RunID, status, result.Fetched, result.Inserted+result.Updated, errorMessage)
	}()

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, fmt.Errorf("begin Dandelion Excel import: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('dandelion-excel-import',0))`).Scan(&locked); err != nil {
		return result, fmt.Errorf("acquire Dandelion Excel import lock: %w", err)
	}
	if !locked {
		return result, ErrDandelionExcelImportLocked
	}
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE dandelion_excel_stage (
			record_id TEXT PRIMARY KEY,
			fields JSONB NOT NULL,
			data_updated_at TIMESTAMPTZ NOT NULL
		) ON COMMIT DROP
	`); err != nil {
		return result, fmt.Errorf("create Dandelion Excel stage: %w", err)
	}
	batch := &pgx.Batch{}
	for _, record := range snapshot.Records {
		batch.Queue(`
			INSERT INTO dandelion_excel_stage (record_id,fields,data_updated_at)
			VALUES ($1,$2::jsonb,$3)
		`, record.RecordID, string(record.Fields), record.DataUpdated)
	}
	results := tx.SendBatch(ctx, batch)
	for index := range snapshot.Records {
		if _, batchErr := results.Exec(); batchErr != nil {
			_ = results.Close()
			return result, fmt.Errorf("stage Dandelion Excel row %d: %w", snapshot.Records[index].SourceRow, batchErr)
		}
	}
	if err := results.Close(); err != nil {
		return result, fmt.Errorf("close Dandelion Excel stage batch: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE target.record_id IS NULL),
			COUNT(*) FILTER (WHERE target.record_id IS NOT NULL
				AND (target.fields IS DISTINCT FROM stage.fields OR target.deleted_at IS NOT NULL)),
			COUNT(*) FILTER (WHERE target.record_id IS NOT NULL
				AND target.fields=stage.fields AND target.deleted_at IS NULL)
		FROM dandelion_excel_stage stage
		LEFT JOIN lark_bitable_records target
		  ON target.app_token=$1 AND target.table_id=$2 AND target.record_id=stage.record_id
	`, dandelionExcelAppToken, dandelionExcelTableID).Scan(&result.Inserted, &result.Updated, &result.Unchanged); err != nil {
		return result, fmt.Errorf("compare Dandelion Excel records: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO lark_bitable_tables (app_token,table_id,name,revision,synced_at,deleted_at)
		VALUES ($1,$2,'蒲公英数据',0,NOW(),NULL)
		ON CONFLICT (app_token,table_id) DO UPDATE SET
			name=EXCLUDED.name,synced_at=NOW(),deleted_at=NULL
	`, dandelionExcelAppToken, dandelionExcelTableID); err != nil {
		return result, fmt.Errorf("upsert Dandelion Excel table: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO lark_bitable_records (
			app_token,table_id,record_id,fields,lark_created_at,lark_updated_at,synced_at,deleted_at
		)
		SELECT $1,$2,record_id,fields,NULL,data_updated_at,NOW(),NULL
		FROM dandelion_excel_stage
		ON CONFLICT (app_token,table_id,record_id) DO UPDATE SET
			fields=EXCLUDED.fields,
			lark_updated_at=EXCLUDED.lark_updated_at,
			synced_at=NOW(),
			deleted_at=NULL
	`, dandelionExcelAppToken, dandelionExcelTableID); err != nil {
		return result, fmt.Errorf("upsert Dandelion Excel records: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit Dandelion Excel import: %w", err)
	}
	return result, nil
}
