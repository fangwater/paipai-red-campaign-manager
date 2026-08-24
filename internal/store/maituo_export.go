package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"paipai-red-campaign-manager/internal/maituo"

	"github.com/jackc/pgx/v5"
)

var ErrMaituoSubaccountReportNotFound = errors.New("未找到该子账户的指定日报")

func (p *Postgres) MaituoSubaccountDirectories(ctx context.Context) ([]maituo.SubaccountDirectory, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT subaccount,COUNT(DISTINCT report_date),MIN(report_date)::TEXT,MAX(report_date)::TEXT
		FROM maituo_customer_daily_subaccounts
		WHERE deleted_at IS NULL AND BTRIM(subaccount) NOT IN ('','总体')
		GROUP BY subaccount
		ORDER BY subaccount
	`)
	if err != nil {
		return nil, fmt.Errorf("query Maituo subaccount directories: %w", err)
	}
	defer rows.Close()
	items := make([]maituo.SubaccountDirectory, 0)
	for rows.Next() {
		var item maituo.SubaccountDirectory
		if err := rows.Scan(&item.Subaccount, &item.ReportCount, &item.EarliestReportDate, &item.LatestReportDate); err != nil {
			return nil, fmt.Errorf("scan Maituo subaccount directory: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Maituo subaccount directories: %w", err)
	}
	return items, nil
}

func (p *Postgres) MaituoSubaccountReports(ctx context.Context, subaccount string) ([]maituo.SubaccountReport, error) {
	rows, err := p.pool.Query(ctx, `
		WITH account_dates AS (
			SELECT DISTINCT report_date
			FROM maituo_customer_daily_subaccounts
			WHERE deleted_at IS NULL AND subaccount=$1
		)
		SELECT dates.report_date::TEXT,COALESCE(run.file_name,dates.report_date::TEXT || '-Maituo-客户日报.xlsx')
		FROM account_dates dates
		LEFT JOIN LATERAL (
			SELECT file_name FROM maituo_customer_daily_import_runs
			WHERE status='succeeded' AND report_date=dates.report_date
			ORDER BY completed_at DESC NULLS LAST,id DESC LIMIT 1
		) run ON TRUE
		ORDER BY dates.report_date DESC
	`, subaccount)
	if err != nil {
		return nil, fmt.Errorf("query Maituo subaccount reports: %w", err)
	}
	defer rows.Close()
	items := make([]maituo.SubaccountReport, 0)
	for rows.Next() {
		var item maituo.SubaccountReport
		if err := rows.Scan(&item.ReportDate, &item.FileName); err != nil {
			return nil, fmt.Errorf("scan Maituo subaccount report: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Maituo subaccount reports: %w", err)
	}
	return items, nil
}

func (p *Postgres) MaituoSubaccountSnapshot(ctx context.Context, subaccount string, reportDate time.Time) (maituo.Snapshot, error) {
	snapshot := maituo.Snapshot{ReportDate: reportDate, PresentSheets: []string{maituo.SheetNotes, maituo.SheetSubaccount}}
	if err := p.pool.QueryRow(ctx, `
		SELECT file_name FROM maituo_customer_daily_import_runs
		WHERE status='succeeded' AND report_date=$1
		ORDER BY completed_at DESC NULLS LAST,id DESC LIMIT 1
	`, reportDate).Scan(&snapshot.FileName); errors.Is(err, pgx.ErrNoRows) {
		return maituo.Snapshot{}, ErrMaituoSubaccountReportNotFound
	} else if err != nil {
		return maituo.Snapshot{}, fmt.Errorf("find Maituo report for subaccount export: %w", err)
	}

	noteRows, err := p.pool.Query(ctx, `
		SELECT note_id,note_url,category,subaccount,campaign_name,placement,keyword_category_note,
		       spend,search_users,search_cost,estimated_postback_cost,search_rate_pct,cpc,ctr_pct,source_row_number,content_hash
		FROM maituo_customer_daily_notes
		WHERE report_date=$1 AND subaccount=$2 AND deleted_at IS NULL
		ORDER BY source_row_number,note_id,campaign_name,placement
	`, reportDate, subaccount)
	if err != nil {
		return maituo.Snapshot{}, fmt.Errorf("query Maituo subaccount notes: %w", err)
	}
	for noteRows.Next() {
		var row maituo.NoteDetail
		if err := noteRows.Scan(
			&row.NoteID, &row.NoteURL, &row.Category, &row.Subaccount, &row.CampaignName, &row.Placement,
			&row.KeywordCategoryNote, &row.Spend, &row.SearchUsers, &row.SearchCost,
			&row.EstimatedPostbackCost, &row.SearchRatePct, &row.CPC, &row.CTRPct,
			&row.SourceRow, &row.ContentHash,
		); err != nil {
			noteRows.Close()
			return maituo.Snapshot{}, fmt.Errorf("scan Maituo subaccount note: %w", err)
		}
		snapshot.Notes = append(snapshot.Notes, row)
	}
	if err := noteRows.Err(); err != nil {
		noteRows.Close()
		return maituo.Snapshot{}, fmt.Errorf("iterate Maituo subaccount notes: %w", err)
	}
	noteRows.Close()

	subRows, err := p.pool.Query(ctx, `
		SELECT spu,subaccount,placement,search_cost,estimated_postback_cost,spend,search_users,
		       search_rate_pct,cpc,ctr_pct,note_count,source_row_number,content_hash
		FROM maituo_customer_daily_subaccounts
		WHERE report_date=$1 AND subaccount=$2 AND deleted_at IS NULL
		ORDER BY source_row_number,spu,placement
	`, reportDate, subaccount)
	if err != nil {
		return maituo.Snapshot{}, fmt.Errorf("query Maituo subaccount summary: %w", err)
	}
	defer subRows.Close()
	for subRows.Next() {
		var row maituo.SubaccountOverview
		if err := subRows.Scan(&row.SPU, &row.Subaccount, &row.Placement, &row.SearchCost, &row.EstimatedPostbackCost, &row.Spend, &row.SearchUsers, &row.SearchRatePct, &row.CPC, &row.CTRPct, &row.NoteCount, &row.SourceRow, &row.ContentHash); err != nil {
			return maituo.Snapshot{}, fmt.Errorf("scan Maituo subaccount summary: %w", err)
		}
		snapshot.Subaccounts = append(snapshot.Subaccounts, row)
	}
	if err := subRows.Err(); err != nil {
		return maituo.Snapshot{}, fmt.Errorf("iterate Maituo subaccount summary: %w", err)
	}
	if len(snapshot.Notes) == 0 && len(snapshot.Subaccounts) == 0 {
		return maituo.Snapshot{}, ErrMaituoSubaccountReportNotFound
	}
	return snapshot, nil
}
