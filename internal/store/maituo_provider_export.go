package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"paipai-red-campaign-manager/internal/maituo"

	"github.com/jackc/pgx/v5"
)

var ErrMaituoProviderReportNotFound = errors.New("未找到该服务商的指定日报")

func (p *Postgres) MaituoProviderDirectories(ctx context.Context) ([]maituo.ProviderDirectory, error) {
	rows, err := p.pool.Query(ctx, `
		WITH provider_notes AS (
			SELECT DISTINCT provider_code, LOWER(BTRIM(note_id)) AS note_id
			FROM service_provider_note_executions
			WHERE deleted_at IS NULL AND NULLIF(BTRIM(note_id), '') IS NOT NULL
		)
		SELECT tables.provider_code, tables.provider_name,
			COUNT(DISTINCT daily.report_date)::INTEGER,
			COUNT(DISTINCT LOWER(BTRIM(daily.note_id)))::INTEGER,
			COALESCE(MIN(daily.report_date)::TEXT, ''), COALESCE(MAX(daily.report_date)::TEXT, '')
		FROM service_provider_content_tables tables
		LEFT JOIN provider_notes ON provider_notes.provider_code=tables.provider_code
		LEFT JOIN maituo_customer_daily_notes daily
		  ON LOWER(BTRIM(daily.note_id))=provider_notes.note_id AND daily.deleted_at IS NULL
		WHERE tables.enabled
		GROUP BY tables.provider_code, tables.provider_name
		ORDER BY tables.provider_name, tables.provider_code
	`)
	if err != nil {
		return nil, fmt.Errorf("query Maituo provider directories: %w", err)
	}
	defer rows.Close()
	items := make([]maituo.ProviderDirectory, 0)
	for rows.Next() {
		var item maituo.ProviderDirectory
		if err := rows.Scan(
			&item.ProviderCode, &item.ProviderName, &item.ReportCount, &item.NoteCount,
			&item.EarliestReportDate, &item.LatestReportDate,
		); err != nil {
			return nil, fmt.Errorf("scan Maituo provider directory: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Maituo provider directories: %w", err)
	}
	return items, nil
}

func (p *Postgres) MaituoProviderReports(ctx context.Context, providerCode string) (string, []maituo.ProviderReport, error) {
	providerName, err := p.maituoProviderName(ctx, providerCode)
	if err != nil {
		return "", nil, err
	}
	rows, err := p.pool.Query(ctx, `
		SELECT daily.report_date::TEXT,
			daily.report_date::TEXT || '-Maituo-客户日报-' || $2 || '.xlsx',
			COUNT(DISTINCT LOWER(BTRIM(daily.note_id)))::INTEGER
		FROM maituo_customer_daily_notes daily
		WHERE daily.deleted_at IS NULL AND EXISTS (
			SELECT 1 FROM service_provider_note_executions execution
			WHERE execution.provider_code=$1 AND execution.deleted_at IS NULL
			  AND LOWER(BTRIM(execution.note_id))=LOWER(BTRIM(daily.note_id))
		)
		GROUP BY daily.report_date
		ORDER BY daily.report_date DESC
	`, providerCode, providerName)
	if err != nil {
		return "", nil, fmt.Errorf("query Maituo provider reports: %w", err)
	}
	defer rows.Close()
	items := make([]maituo.ProviderReport, 0)
	for rows.Next() {
		var item maituo.ProviderReport
		if err := rows.Scan(&item.ReportDate, &item.FileName, &item.NoteCount); err != nil {
			return "", nil, fmt.Errorf("scan Maituo provider report: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return "", nil, fmt.Errorf("iterate Maituo provider reports: %w", err)
	}
	return providerName, items, nil
}

func (p *Postgres) MaituoProviderSnapshot(ctx context.Context, providerCode string, reportDate time.Time) (maituo.ProviderSnapshot, error) {
	providerName, err := p.maituoProviderName(ctx, providerCode)
	if err != nil {
		return maituo.ProviderSnapshot{}, err
	}
	snapshot := maituo.Snapshot{
		ReportDate:    reportDate,
		FileName:      reportDate.Format(time.DateOnly) + "-Maituo-客户日报.xlsx",
		PresentSheets: []string{maituo.SheetNotes},
	}
	if err := p.pool.QueryRow(ctx, `
		SELECT file_name FROM maituo_customer_daily_import_runs
		WHERE status='succeeded' AND report_date=$1
		ORDER BY completed_at DESC NULLS LAST,id DESC LIMIT 1
	`, reportDate).Scan(&snapshot.FileName); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return maituo.ProviderSnapshot{}, fmt.Errorf("find Maituo report for provider export: %w", err)
	}

	rows, err := p.pool.Query(ctx, `
		SELECT daily.note_id,daily.note_url,daily.category,daily.subaccount,daily.campaign_name,daily.placement,daily.keyword_category_note,
			daily.spend::DOUBLE PRECISION,daily.search_users,daily.search_cost::DOUBLE PRECISION,
			daily.estimated_postback_cost::DOUBLE PRECISION,daily.search_rate_pct::DOUBLE PRECISION,
			daily.cpc::DOUBLE PRECISION,daily.ctr_pct::DOUBLE PRECISION,
			daily.source_row_number,daily.content_hash
		FROM maituo_customer_daily_notes daily
		WHERE daily.report_date=$1 AND daily.deleted_at IS NULL AND EXISTS (
			SELECT 1 FROM service_provider_note_executions execution
			WHERE execution.provider_code=$2 AND execution.deleted_at IS NULL
			  AND LOWER(BTRIM(execution.note_id))=LOWER(BTRIM(daily.note_id))
		)
		ORDER BY daily.spend DESC,daily.note_id,daily.subaccount,daily.campaign_name,daily.placement
	`, reportDate, providerCode)
	if err != nil {
		return maituo.ProviderSnapshot{}, fmt.Errorf("query Maituo provider notes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row maituo.NoteDetail
		if err := rows.Scan(
			&row.NoteID, &row.NoteURL, &row.Category, &row.Subaccount, &row.CampaignName, &row.Placement, &row.KeywordCategoryNote,
			&row.Spend, &row.SearchUsers, &row.SearchCost, &row.EstimatedPostbackCost,
			&row.SearchRatePct, &row.CPC, &row.CTRPct, &row.SourceRow, &row.ContentHash,
		); err != nil {
			return maituo.ProviderSnapshot{}, fmt.Errorf("scan Maituo provider note: %w", err)
		}
		snapshot.Notes = append(snapshot.Notes, row)
	}
	if err := rows.Err(); err != nil {
		return maituo.ProviderSnapshot{}, fmt.Errorf("iterate Maituo provider notes: %w", err)
	}
	if len(snapshot.Notes) == 0 {
		return maituo.ProviderSnapshot{}, ErrMaituoProviderReportNotFound
	}
	return maituo.ProviderSnapshot{ProviderCode: providerCode, ProviderName: providerName, Snapshot: snapshot}, nil
}

func (p *Postgres) maituoProviderName(ctx context.Context, providerCode string) (string, error) {
	var providerName string
	err := p.pool.QueryRow(ctx, `
		SELECT provider_name FROM service_provider_content_tables
		WHERE provider_code=$1 AND enabled
	`, providerCode).Scan(&providerName)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrMaituoProviderReportNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find Maituo provider: %w", err)
	}
	return providerName, nil
}
