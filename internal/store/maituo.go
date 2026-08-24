package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/maituo"

	"github.com/jackc/pgx/v5"
)

var ErrMaituoImportLocked = errors.New("another Maituo customer daily import is already running")

func (p *Postgres) ImportMaituoCustomerDaily(ctx context.Context, snapshot maituo.Snapshot) (result maituo.ImportResult, err error) {
	result = maituo.ImportResult{FileName: snapshot.FileName, FileSHA256: snapshot.FileSHA256, ReportDate: snapshot.ReportDate.Format("2006-01-02"), PresentSheets: append([]string(nil), snapshot.PresentSheets...), MissingSheets: maituo.MissingSheets(snapshot.PresentSheets), Fetched: len(snapshot.KPIs) + len(snapshot.Notes) + len(snapshot.SPUs) + len(snapshot.Subaccounts) + len(snapshot.Trends)}
	if saved, found, findErr := p.maituoImportByHash(ctx, snapshot.FileSHA256); findErr != nil {
		return result, findErr
	} else if found {
		repairNeeded, repairErr := p.maituoNoteAttributionRepairNeeded(ctx, snapshot)
		if repairErr != nil {
			return result, repairErr
		}
		if !repairNeeded {
			saved.AlreadySaved = true
			return saved, nil
		}
	}
	if err := p.pool.QueryRow(ctx, `INSERT INTO maituo_customer_daily_import_runs (file_name,file_sha256,report_date,present_sheets,status) VALUES ($1,$2,$3,$4,'running') RETURNING id`, snapshot.FileName, snapshot.FileSHA256, snapshot.ReportDate, snapshot.PresentSheets).Scan(&result.RunID); err != nil {
		return result, fmt.Errorf("start Maituo import run: %w", err)
	}
	defer func() {
		finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.finishMaituoImportRun(finishCtx, result, err)
	}()
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, fmt.Errorf("begin Maituo import transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('maituo-customer-daily',0))`).Scan(&locked); err != nil {
		return result, fmt.Errorf("acquire Maituo import lock: %w", err)
	}
	if !locked {
		return result, ErrMaituoImportLocked
	}
	applyCurrentTables, err := shouldApplyMaituoCurrentTables(ctx, tx, snapshot.ReportDate)
	if err != nil {
		return result, err
	}
	if err := createMaituoStages(ctx, tx); err != nil {
		return result, err
	}
	if err := copyMaituoStages(ctx, tx, snapshot); err != nil {
		return result, err
	}

	specs := maituoReconcileSpecs(snapshot, applyCurrentTables)
	result.TableCount = len(specs)
	for _, spec := range specs {
		tableResult, reconcileErr := reconcileMaituoTable(ctx, tx, result.RunID, snapshot.ReportDate, spec)
		if reconcileErr != nil {
			return result, reconcileErr
		}
		result.Tables = append(result.Tables, tableResult)
		result.Inserted += tableResult.Inserted
		result.Updated += tableResult.Updated
		result.Unchanged += tableResult.Unchanged
		result.Deleted += tableResult.Deleted
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit Maituo import: %w", err)
	}
	return result, nil
}

// maituoNoteAttributionRepairNeeded permits one replay of workbooks imported
// before note account dimensions were restored. The parser requires these
// values, so a replay can replace only the explicitly unassigned rows.
func (p *Postgres) maituoNoteAttributionRepairNeeded(ctx context.Context, snapshot maituo.Snapshot) (bool, error) {
	if !snapshot.HasSheet(maituo.SheetNotes) || len(snapshot.Notes) == 0 {
		return false, nil
	}
	for _, note := range snapshot.Notes {
		if strings.TrimSpace(note.Subaccount) == "" || strings.TrimSpace(note.CampaignName) == "" {
			return false, nil
		}
	}
	var found bool
	if err := p.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM maituo_customer_daily_notes
			WHERE report_date=$1 AND deleted_at IS NULL AND BTRIM(subaccount)=''
		)
	`, snapshot.ReportDate).Scan(&found); err != nil {
		return false, fmt.Errorf("check Maituo note attribution repair: %w", err)
	}
	return found, nil
}

type maituoReconcileSpec struct{ key, name, target, stage, presence, join, deleteScope, upsert string }

func reconcileMaituoTable(ctx context.Context, tx pgx.Tx, runID int64, reportDate time.Time, spec maituoReconcileSpec) (result maituo.TableResult, err error) {
	result.Key, result.Name = spec.key, spec.name
	query := fmt.Sprintf(`SELECT COUNT(*), COUNT(*) FILTER (WHERE %s IS NULL), COUNT(*) FILTER (WHERE %s IS NOT NULL AND (t.content_hash IS DISTINCT FROM s.content_hash OR t.deleted_at IS NOT NULL)), COUNT(*) FILTER (WHERE %s IS NOT NULL AND t.content_hash=s.content_hash AND t.deleted_at IS NULL) FROM %s s LEFT JOIN %s t ON %s`, spec.presence, spec.presence, spec.presence, spec.stage, spec.target, spec.join)
	if err := tx.QueryRow(ctx, query).Scan(&result.Fetched, &result.Inserted, &result.Updated, &result.Unchanged); err != nil {
		return result, fmt.Errorf("compare Maituo table %s: %w", spec.name, err)
	}
	if _, err := tx.Exec(ctx, spec.upsert, runID); err != nil {
		return result, fmt.Errorf("upsert Maituo table %s: %w", spec.name, err)
	}
	deleteScope := ""
	args := []interface{}{runID}
	if spec.deleteScope != "" {
		deleteScope = " AND " + spec.deleteScope
		args = append(args, reportDate)
	}
	deleteSQL := fmt.Sprintf(`UPDATE %s t SET deleted_at=NOW(),updated_at=NOW(),import_run_id=$1 WHERE t.deleted_at IS NULL%s AND NOT EXISTS (SELECT 1 FROM %s s WHERE %s)`, spec.target, deleteScope, spec.stage, spec.join)
	deleted, err := tx.Exec(ctx, deleteSQL, args...)
	if err != nil {
		return result, fmt.Errorf("soft-delete Maituo table %s: %w", spec.name, err)
	}
	result.Deleted = deleted.RowsAffected()
	return result, nil
}

func createMaituoStages(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		CREATE TEMP TABLE maituo_stage_kpis (report_date DATE,metric TEXT,metric_value NUMERIC,data_basis TEXT,source_row_number INTEGER,content_hash TEXT,PRIMARY KEY(report_date,metric)) ON COMMIT DROP;
		CREATE TEMP TABLE maituo_stage_notes (report_date DATE,note_id TEXT,note_url TEXT,category TEXT,subaccount TEXT,campaign_name TEXT,placement TEXT,keyword_category_note TEXT,spend NUMERIC,search_users BIGINT,search_cost NUMERIC,estimated_postback_cost NUMERIC,search_rate_pct NUMERIC,cpc NUMERIC,ctr_pct NUMERIC,source_row_number INTEGER,content_hash TEXT,PRIMARY KEY(report_date,note_id,subaccount,campaign_name,placement)) ON COMMIT DROP;
		CREATE TEMP TABLE maituo_stage_spus (report_date DATE,spu TEXT,auction_spend NUMERIC,search_users BIGINT,search_cost NUMERIC,search_rate_pct NUMERIC,cpc NUMERIC,ctr_pct NUMERIC,note_count BIGINT,source_row_number INTEGER,content_hash TEXT,PRIMARY KEY(report_date,spu)) ON COMMIT DROP;
		CREATE TEMP TABLE maituo_stage_subaccounts (report_date DATE,spu TEXT,subaccount TEXT,placement TEXT,search_cost NUMERIC,estimated_postback_cost NUMERIC,spend NUMERIC,search_users BIGINT,search_rate_pct NUMERIC,cpc NUMERIC,ctr_pct NUMERIC,note_count BIGINT,source_row_number INTEGER,content_hash TEXT,PRIMARY KEY(report_date,spu,subaccount,placement)) ON COMMIT DROP;
		CREATE TEMP TABLE maituo_stage_trends (report_date DATE PRIMARY KEY,coenzyme_spend NUMERIC,coenzyme_search_uv BIGINT,coenzyme_order_uv BIGINT,coenzyme_search_cost NUMERIC,krill_oil_spend NUMERIC,krill_oil_search_uv BIGINT,krill_oil_order_uv BIGINT,krill_oil_search_cost NUMERIC,total_search_uv BIGINT,total_order_uv BIGINT,total_search_cost NUMERIC,total_spend NUMERIC,total_recall_search_cost NUMERIC,source_row_number INTEGER,content_hash TEXT) ON COMMIT DROP
	`)
	if err != nil {
		return fmt.Errorf("create Maituo staging tables: %w", err)
	}
	return nil
}

func copyMaituoStages(ctx context.Context, tx pgx.Tx, snapshot maituo.Snapshot) error {
	kpis := make([][]interface{}, len(snapshot.KPIs))
	for i, row := range snapshot.KPIs {
		kpis[i] = []interface{}{snapshot.ReportDate, row.Metric, row.Value, row.DataBasis, row.SourceRow, row.ContentHash}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"maituo_stage_kpis"}, []string{"report_date", "metric", "metric_value", "data_basis", "source_row_number", "content_hash"}, pgx.CopyFromRows(kpis)); err != nil {
		return fmt.Errorf("stage %s: %w", maituo.SheetKPI, err)
	}
	notes := make([][]interface{}, len(snapshot.Notes))
	for i, row := range snapshot.Notes {
		notes[i] = []interface{}{snapshot.ReportDate, row.NoteID, row.NoteURL, row.Category, row.Subaccount, row.CampaignName, row.Placement, nullableString(row.KeywordCategoryNote), row.Spend, row.SearchUsers, nullableFloat(row.SearchCost), nullableFloat(row.EstimatedPostbackCost), nullableFloat(row.SearchRatePct), row.CPC, row.CTRPct, row.SourceRow, row.ContentHash}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"maituo_stage_notes"}, []string{"report_date", "note_id", "note_url", "category", "subaccount", "campaign_name", "placement", "keyword_category_note", "spend", "search_users", "search_cost", "estimated_postback_cost", "search_rate_pct", "cpc", "ctr_pct", "source_row_number", "content_hash"}, pgx.CopyFromRows(notes)); err != nil {
		return fmt.Errorf("stage %s: %w", maituo.SheetNotes, err)
	}
	spus := make([][]interface{}, len(snapshot.SPUs))
	for i, row := range snapshot.SPUs {
		spus[i] = []interface{}{snapshot.ReportDate, row.SPU, row.AuctionSpend, row.SearchUsers, row.SearchCost, row.SearchRatePct, row.CPC, row.CTRPct, row.NoteCount, row.SourceRow, row.ContentHash}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"maituo_stage_spus"}, []string{"report_date", "spu", "auction_spend", "search_users", "search_cost", "search_rate_pct", "cpc", "ctr_pct", "note_count", "source_row_number", "content_hash"}, pgx.CopyFromRows(spus)); err != nil {
		return fmt.Errorf("stage %s: %w", maituo.SheetSPU, err)
	}
	subs := make([][]interface{}, len(snapshot.Subaccounts))
	for i, row := range snapshot.Subaccounts {
		subs[i] = []interface{}{snapshot.ReportDate, row.SPU, row.Subaccount, row.Placement, nullableFloat(row.SearchCost), nullableFloat(row.EstimatedPostbackCost), row.Spend, row.SearchUsers, nullableFloat(row.SearchRatePct), nullableFloat(row.CPC), nullableFloat(row.CTRPct), row.NoteCount, row.SourceRow, row.ContentHash}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"maituo_stage_subaccounts"}, []string{"report_date", "spu", "subaccount", "placement", "search_cost", "estimated_postback_cost", "spend", "search_users", "search_rate_pct", "cpc", "ctr_pct", "note_count", "source_row_number", "content_hash"}, pgx.CopyFromRows(subs)); err != nil {
		return fmt.Errorf("stage %s: %w", maituo.SheetSubaccount, err)
	}
	trends := make([][]interface{}, len(snapshot.Trends))
	for i, row := range snapshot.Trends {
		trends[i] = []interface{}{row.Date, nullableFloat(row.CoenzymeSpend), nullableInt(row.CoenzymeSearchUV), nullableInt(row.CoenzymeOrderUV), nullableFloat(row.CoenzymeSearchCost), nullableFloat(row.KrillOilSpend), nullableInt(row.KrillOilSearchUV), nullableInt(row.KrillOilOrderUV), nullableFloat(row.KrillOilSearchCost), nullableInt(row.TotalSearchUV), nullableInt(row.TotalOrderUV), nullableFloat(row.TotalSearchCost), nullableFloat(row.TotalSpend), nullableFloat(row.TotalRecallSearchCost), row.SourceRow, row.ContentHash}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"maituo_stage_trends"}, []string{"report_date", "coenzyme_spend", "coenzyme_search_uv", "coenzyme_order_uv", "coenzyme_search_cost", "krill_oil_spend", "krill_oil_search_uv", "krill_oil_order_uv", "krill_oil_search_cost", "total_search_uv", "total_order_uv", "total_search_cost", "total_spend", "total_recall_search_cost", "source_row_number", "content_hash"}, pgx.CopyFromRows(trends)); err != nil {
		return fmt.Errorf("stage %s: %w", maituo.SheetTrend, err)
	}
	return nil
}

func nullableFloat(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
func nullableInt(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
func nullableString(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func (p *Postgres) maituoImportByHash(ctx context.Context, fileSHA256 string) (maituo.ImportResult, bool, error) {
	var result maituo.ImportResult
	var tableStats []byte
	err := p.pool.QueryRow(ctx, `
		SELECT id,file_name,file_sha256,report_date::TEXT,present_sheets,fetched_count,inserted_count,updated_count,unchanged_count,deleted_count,table_stats
		FROM maituo_customer_daily_import_runs
		WHERE status='succeeded' AND file_sha256=$1
		ORDER BY completed_at DESC
		LIMIT 1
	`, fileSHA256).Scan(&result.RunID, &result.FileName, &result.FileSHA256, &result.ReportDate, &result.PresentSheets, &result.Fetched, &result.Inserted, &result.Updated, &result.Unchanged, &result.Deleted, &tableStats)
	if errors.Is(err, pgx.ErrNoRows) {
		return maituo.ImportResult{}, false, nil
	}
	if err != nil {
		return maituo.ImportResult{}, false, fmt.Errorf("find saved Maituo import: %w", err)
	}
	if err := json.Unmarshal(tableStats, &result.Tables); err != nil {
		return maituo.ImportResult{}, false, fmt.Errorf("decode saved Maituo import stats: %w", err)
	}
	result.TableCount = len(result.Tables)
	result.MissingSheets = maituo.MissingSheets(result.PresentSheets)
	return result, true, nil
}

func (p *Postgres) SavedMaituoImports(ctx context.Context) ([]maituo.SavedImport, error) {
	rows, err := p.pool.Query(ctx, `
			SELECT DISTINCT ON (runs.report_date) runs.id,runs.file_name,runs.file_sha256,
				runs.report_date::TEXT,runs.present_sheets,runs.fetched_count,
				(SELECT COUNT(*)::INTEGER FROM maituo_customer_daily_notes notes
				 WHERE notes.report_date=runs.report_date AND notes.deleted_at IS NULL),
				COALESCE(runs.completed_at,runs.started_at)
			FROM maituo_customer_daily_import_runs runs
			WHERE runs.status='succeeded'
			ORDER BY runs.report_date DESC,runs.completed_at DESC,runs.id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list saved Maituo imports: %w", err)
	}
	defer rows.Close()
	result := make([]maituo.SavedImport, 0)
	for rows.Next() {
		var item maituo.SavedImport
		if err := rows.Scan(&item.RunID, &item.FileName, &item.FileSHA256, &item.ReportDate, &item.PresentSheets, &item.Fetched, &item.MergedRows, &item.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan saved Maituo import: %w", err)
		}
		item.MissingSheets = maituo.MissingSheets(item.PresentSheets)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate saved Maituo imports: %w", err)
	}
	return result, nil
}

func (p *Postgres) MaituoDailyNotes(ctx context.Context, reportDate time.Time) (maituo.DailyNoteReport, error) {
	result := maituo.DailyNoteReport{
		ReportDate: reportDate.Format(time.DateOnly),
		Items:      []maituo.NoteDetail{},
	}
	rows, err := p.pool.Query(ctx, `
		SELECT note_id,note_url,category,subaccount,campaign_name,placement,keyword_category_note,
			spend::DOUBLE PRECISION,search_users,search_cost::DOUBLE PRECISION,
			estimated_postback_cost::DOUBLE PRECISION,search_rate_pct::DOUBLE PRECISION,
			cpc::DOUBLE PRECISION,ctr_pct::DOUBLE PRECISION
		FROM maituo_customer_daily_notes
		WHERE report_date=$1 AND deleted_at IS NULL
		ORDER BY spend DESC,note_id,subaccount,campaign_name,placement
	`, reportDate)
	if err != nil {
		return result, fmt.Errorf("query Maituo daily notes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item maituo.NoteDetail
		if err := rows.Scan(
			&item.NoteID, &item.NoteURL, &item.Category, &item.Subaccount, &item.CampaignName, &item.Placement, &item.KeywordCategoryNote,
			&item.Spend, &item.SearchUsers, &item.SearchCost, &item.EstimatedPostbackCost,
			&item.SearchRatePct, &item.CPC, &item.CTRPct,
		); err != nil {
			return result, fmt.Errorf("scan Maituo daily note: %w", err)
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate Maituo daily notes: %w", err)
	}
	result.Total = len(result.Items)
	return result, nil
}

func (p *Postgres) finishMaituoImportRun(ctx context.Context, result maituo.ImportResult, runErr error) error {
	status, errorMessage := "succeeded", ""
	if runErr != nil {
		status, errorMessage = "failed", runErr.Error()
	}
	stats, err := json.Marshal(result.Tables)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `UPDATE maituo_customer_daily_import_runs SET status=$2,table_stats=$3::jsonb,fetched_count=$4,inserted_count=$5,updated_count=$6,unchanged_count=$7,deleted_count=$8,completed_at=NOW(),error_message=NULLIF($9,'') WHERE id=$1`, result.RunID, status, string(stats), result.Fetched, result.Inserted, result.Updated, result.Unchanged, result.Deleted, errorMessage)
	return err
}

const maituoKPIUpsert = `INSERT INTO maituo_customer_daily_kpis (report_date,metric,metric_value,data_basis,source_row_number,content_hash,import_run_id) SELECT report_date,metric,metric_value,data_basis,source_row_number,content_hash,$1 FROM maituo_stage_kpis ON CONFLICT(report_date,metric) DO UPDATE SET metric_value=EXCLUDED.metric_value,data_basis=EXCLUDED.data_basis,source_row_number=EXCLUDED.source_row_number,content_hash=EXCLUDED.content_hash,import_run_id=EXCLUDED.import_run_id,updated_at=NOW(),deleted_at=NULL WHERE maituo_customer_daily_kpis.content_hash IS DISTINCT FROM EXCLUDED.content_hash OR maituo_customer_daily_kpis.deleted_at IS NOT NULL`
const maituoNoteUpsert = `INSERT INTO maituo_customer_daily_notes (report_date,note_id,note_url,category,subaccount,campaign_name,placement,keyword_category_note,spend,search_users,search_cost,estimated_postback_cost,search_rate_pct,cpc,ctr_pct,source_row_number,content_hash,import_run_id) SELECT report_date,note_id,note_url,category,subaccount,campaign_name,placement,keyword_category_note,spend,search_users,search_cost,ROUND(ROUND(search_cost,2)*0.63,2),search_rate_pct,cpc,ctr_pct,source_row_number,content_hash,$1 FROM maituo_stage_notes ON CONFLICT(report_date,note_id,subaccount,campaign_name,placement) DO UPDATE SET note_url=EXCLUDED.note_url,category=EXCLUDED.category,keyword_category_note=EXCLUDED.keyword_category_note,spend=EXCLUDED.spend,search_users=EXCLUDED.search_users,search_cost=EXCLUDED.search_cost,estimated_postback_cost=EXCLUDED.estimated_postback_cost,search_rate_pct=EXCLUDED.search_rate_pct,cpc=EXCLUDED.cpc,ctr_pct=EXCLUDED.ctr_pct,source_row_number=EXCLUDED.source_row_number,content_hash=EXCLUDED.content_hash,import_run_id=EXCLUDED.import_run_id,updated_at=NOW(),deleted_at=NULL WHERE maituo_customer_daily_notes.content_hash IS DISTINCT FROM EXCLUDED.content_hash OR maituo_customer_daily_notes.deleted_at IS NOT NULL`
const maituoSPUUpsert = `INSERT INTO maituo_customer_daily_spus (report_date,spu,auction_spend,search_users,search_cost,search_rate_pct,cpc,ctr_pct,note_count,source_row_number,content_hash,import_run_id) SELECT report_date,spu,auction_spend,search_users,search_cost,search_rate_pct,cpc,ctr_pct,note_count,source_row_number,content_hash,$1 FROM maituo_stage_spus ON CONFLICT(report_date,spu) DO UPDATE SET auction_spend=EXCLUDED.auction_spend,search_users=EXCLUDED.search_users,search_cost=EXCLUDED.search_cost,search_rate_pct=EXCLUDED.search_rate_pct,cpc=EXCLUDED.cpc,ctr_pct=EXCLUDED.ctr_pct,note_count=EXCLUDED.note_count,source_row_number=EXCLUDED.source_row_number,content_hash=EXCLUDED.content_hash,import_run_id=EXCLUDED.import_run_id,updated_at=NOW(),deleted_at=NULL WHERE maituo_customer_daily_spus.content_hash IS DISTINCT FROM EXCLUDED.content_hash OR maituo_customer_daily_spus.deleted_at IS NOT NULL`
const maituoSubaccountUpsert = `INSERT INTO maituo_customer_daily_subaccounts (report_date,spu,subaccount,placement,search_cost,estimated_postback_cost,spend,search_users,search_rate_pct,cpc,ctr_pct,note_count,source_row_number,content_hash,import_run_id) SELECT report_date,spu,subaccount,placement,search_cost,ROUND(ROUND(search_cost,2)*0.63,2),spend,search_users,search_rate_pct,cpc,ctr_pct,note_count,source_row_number,content_hash,$1 FROM maituo_stage_subaccounts ON CONFLICT(report_date,spu,subaccount,placement) DO UPDATE SET search_cost=EXCLUDED.search_cost,estimated_postback_cost=EXCLUDED.estimated_postback_cost,spend=EXCLUDED.spend,search_users=EXCLUDED.search_users,search_rate_pct=EXCLUDED.search_rate_pct,cpc=EXCLUDED.cpc,ctr_pct=EXCLUDED.ctr_pct,note_count=EXCLUDED.note_count,source_row_number=EXCLUDED.source_row_number,content_hash=EXCLUDED.content_hash,import_run_id=EXCLUDED.import_run_id,updated_at=NOW(),deleted_at=NULL WHERE maituo_customer_daily_subaccounts.content_hash IS DISTINCT FROM EXCLUDED.content_hash OR maituo_customer_daily_subaccounts.deleted_at IS NOT NULL`
const maituoTrendUpsert = `INSERT INTO maituo_customer_daily_trends (report_date,coenzyme_spend,coenzyme_search_uv,coenzyme_order_uv,coenzyme_search_cost,krill_oil_spend,krill_oil_search_uv,krill_oil_order_uv,krill_oil_search_cost,total_search_uv,total_order_uv,total_search_cost,total_spend,total_recall_search_cost,source_row_number,content_hash,import_run_id) SELECT report_date,coenzyme_spend,coenzyme_search_uv,coenzyme_order_uv,coenzyme_search_cost,krill_oil_spend,krill_oil_search_uv,krill_oil_order_uv,krill_oil_search_cost,total_search_uv,total_order_uv,total_search_cost,total_spend,total_recall_search_cost,source_row_number,content_hash,$1 FROM maituo_stage_trends ON CONFLICT(report_date) DO UPDATE SET coenzyme_spend=EXCLUDED.coenzyme_spend,coenzyme_search_uv=EXCLUDED.coenzyme_search_uv,coenzyme_order_uv=EXCLUDED.coenzyme_order_uv,coenzyme_search_cost=EXCLUDED.coenzyme_search_cost,krill_oil_spend=EXCLUDED.krill_oil_spend,krill_oil_search_uv=EXCLUDED.krill_oil_search_uv,krill_oil_order_uv=EXCLUDED.krill_oil_order_uv,krill_oil_search_cost=EXCLUDED.krill_oil_search_cost,total_search_uv=EXCLUDED.total_search_uv,total_order_uv=EXCLUDED.total_order_uv,total_search_cost=EXCLUDED.total_search_cost,total_spend=EXCLUDED.total_spend,total_recall_search_cost=EXCLUDED.total_recall_search_cost,source_row_number=EXCLUDED.source_row_number,content_hash=EXCLUDED.content_hash,import_run_id=EXCLUDED.import_run_id,updated_at=NOW(),deleted_at=NULL WHERE maituo_customer_daily_trends.content_hash IS DISTINCT FROM EXCLUDED.content_hash OR maituo_customer_daily_trends.deleted_at IS NOT NULL`
