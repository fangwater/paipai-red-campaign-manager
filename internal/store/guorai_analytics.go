package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"paipai-red-campaign-manager/internal/model"

	"github.com/jackc/pgx/v5"
)

const guoraiExcludedNoteAuthor = "MegaRed脉拓"

func (p *Postgres) GuoraiLatest(ctx context.Context, query model.GuoraiLatestQuery) (model.GuoraiLatestResult, error) {
	if query.EntityType != "note" && query.EntityType != "plan" {
		return model.GuoraiLatestResult{}, fmt.Errorf("unsupported Guorai entity type %q", query.EntityType)
	}
	if query.SPU == "" {
		query.SPU = "辅酶"
	}
	if query.Sort == "" {
		query.Sort = "roi"
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 25
	}
	result := model.GuoraiLatestResult{
		EntityType: query.EntityType,
		SPU:        query.SPU,
		Sort:       query.Sort,
		Page:       query.Page,
		PageSize:   query.PageSize,
		Items:      []model.GuoraiLatestItem{},
	}
	snapshot, err := p.latestGuoraiSnapshot(ctx, query.EntityType)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.Snapshot = &snapshot
	if err := p.loadGuoraiLatestSummary(ctx, query, &result); err != nil {
		return result, err
	}
	if query.EntityType == "note" {
		err = p.loadGuoraiLatestNotes(ctx, query, &result)
	} else {
		err = p.loadGuoraiLatestPlans(ctx, query, &result)
	}
	return result, err
}

func (p *Postgres) latestGuoraiSnapshot(ctx context.Context, entityType string) (model.GuoraiLatestSnapshot, error) {
	var snapshot model.GuoraiLatestSnapshot
	err := p.pool.QueryRow(ctx, `
		SELECT id, entity_type, snapshot_date::TEXT, window_start::TEXT, window_end::TEXT,
			(window_end - window_start + 1)::INTEGER, source_cutoff_date::TEXT, brand_name,
			attribution_type, attribution_model, COALESCE(attribution_window_days, 0),
			row_count, finished_at
		FROM guorai_fetch_runs
		WHERE entity_type=$1 AND status='succeeded'
		ORDER BY snapshot_date DESC, finished_at DESC NULLS LAST, id DESC
		LIMIT 1
	`, entityType).Scan(
		&snapshot.FetchID, &snapshot.EntityType, &snapshot.SnapshotDate,
		&snapshot.WindowStart, &snapshot.WindowEnd, &snapshot.WindowDays,
		&snapshot.SourceCutoffDate, &snapshot.BrandName, &snapshot.AttributionType,
		&snapshot.AttributionModel, &snapshot.AttributionWindowDays,
		&snapshot.RowCount, &snapshot.FinishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return snapshot, err
		}
		return snapshot, fmt.Errorf("query latest Guorai %s snapshot: %w", entityType, err)
	}
	return snapshot, nil
}

func (p *Postgres) loadGuoraiLatestSummary(ctx context.Context, query model.GuoraiLatestQuery, result *model.GuoraiLatestResult) error {
	var metricsJSON []byte
	var err error
	if result.EntityType == "note" {
		err = p.pool.QueryRow(ctx, `
			SELECT COUNT(*)::INTEGER,
				COUNT(DISTINCT NULLIF(BTRIM(notes.note_author_name), ''))::INTEGER,
				COUNT(*) FILTER (WHERE NULLIF(BTRIM(notes.spu_id), '') IS NOT NULL)::INTEGER,
				COUNT(*) FILTER (WHERE snapshots.is_new IS TRUE)::INTEGER,
			COUNT(*) FILTER (WHERE snapshots.total_pay_amt IS NOT NULL OR snapshots.note_ad_cost_volume IS NOT NULL
				OR snapshots.click_count IS NOT NULL OR snapshots.total_roi IS NOT NULL
				OR snapshots.interact_count IS NOT NULL)::INTEGER,
				jsonb_build_object(
					'total_pay_amount', SUM(snapshots.total_pay_amt)::DOUBLE PRECISION,
					'part_pay_amount', SUM(snapshots.part_pay_amt)::DOUBLE PRECISION,
					'ad_cost', SUM(snapshots.note_ad_cost_volume)::DOUBLE PRECISION,
					'click_count', SUM(snapshots.click_count)::BIGINT,
					'interaction_count', SUM(snapshots.interact_count)::BIGINT,
					'total_roi', (SUM(snapshots.total_pay_amt) / NULLIF(SUM(snapshots.note_ad_cost_volume), 0))::DOUBLE PRECISION
				)
			FROM guorai_note_snapshots snapshots
			JOIN guorai_notes notes USING (note_id)
			WHERE snapshots.fetch_id=$1
			  AND COALESCE(BTRIM(notes.note_author_name), '') NOT ILIKE $2
			  AND notes.spu_name ILIKE $3
		`, result.Snapshot.FetchID, guoraiExcludedNoteAuthor, "%"+query.SPU+"%").Scan(
			&result.Summary.ItemCount, &result.Summary.AccountCount, &result.Summary.LinkedCount,
			&result.Summary.NewCount, &result.Summary.MetricItemCount, &metricsJSON,
		)
	} else {
		err = p.pool.QueryRow(ctx, `
			WITH latest_report AS (
				SELECT MAX(report_date) AS report_date
				FROM maituo_customer_daily_subaccounts
				WHERE deleted_at IS NULL
			), selected_accounts AS (
				SELECT DISTINCT LOWER(BTRIM(report_rows.subaccount)) AS account_key
				FROM maituo_customer_daily_subaccounts report_rows
				JOIN latest_report ON latest_report.report_date=report_rows.report_date
				WHERE report_rows.deleted_at IS NULL AND report_rows.spu=$3
				  AND NULLIF(BTRIM(report_rows.subaccount), '') IS NOT NULL
			)
			SELECT COUNT(*)::INTEGER,
				COUNT(DISTINCT COALESCE(
					NULLIF(BTRIM(plans.account_name), ''),
					NULLIF(BTRIM(delivery_account.advertiser_name), '')
				))::INTEGER,
				COALESCE(SUM(links.note_count), 0)::INTEGER,
				COUNT(*) FILTER (WHERE snapshots.is_new IS TRUE)::INTEGER,
			COUNT(*) FILTER (WHERE snapshots.total_pay_amt IS NOT NULL OR snapshots.note_ad_cost_volume IS NOT NULL
				OR snapshots.click_count IS NOT NULL OR snapshots.total_roi IS NOT NULL
				OR snapshots.interact_count IS NOT NULL)::INTEGER,
				jsonb_build_object(
					'total_pay_amount', SUM(snapshots.total_pay_amt)::DOUBLE PRECISION,
					'part_pay_amount', SUM(snapshots.part_pay_amt)::DOUBLE PRECISION,
					'ad_cost', SUM(snapshots.note_ad_cost_volume)::DOUBLE PRECISION,
					'click_count', SUM(snapshots.click_count)::BIGINT,
					'interaction_count', SUM(snapshots.interact_count)::BIGINT,
					'total_roi', (SUM(snapshots.total_pay_amt) / NULLIF(SUM(snapshots.note_ad_cost_volume), 0))::DOUBLE PRECISION
				)
			FROM guorai_plan_snapshots snapshots
			JOIN guorai_plans plans USING (plan_id)
			LEFT JOIN LATERAL (
				SELECT advertisers.advertiser_name
				FROM xhs_jg_campaigns campaigns
				JOIN xhs_jg_advertisers advertisers USING (advertiser_id)
				WHERE campaigns.campaign_id=CASE
					WHEN plans.plan_id ~ '^[0-9]+$' THEN plans.plan_id::BIGINT
				END
				  AND campaigns.deleted_at IS NULL
				ORDER BY campaigns.last_seen_at DESC, campaigns.advertiser_id
				LIMIT 1
			) delivery_account ON TRUE
			JOIN selected_accounts ON selected_accounts.account_key=LOWER(COALESCE(NULLIF(BTRIM(plans.account_name), ''), NULLIF(BTRIM(delivery_account.advertiser_name), '')))
			LEFT JOIN LATERAL (
				SELECT COUNT(*) AS note_count
				FROM guorai_plan_notes current_links
				JOIN guorai_notes linked_notes ON linked_notes.note_id=current_links.note_id
				WHERE current_links.plan_id=plans.plan_id AND current_links.is_active
				  AND COALESCE(BTRIM(linked_notes.note_author_name), '') NOT ILIKE $2
			) links ON TRUE
			WHERE snapshots.fetch_id=$1
		`, result.Snapshot.FetchID, guoraiExcludedNoteAuthor, query.SPU).Scan(
			&result.Summary.ItemCount, &result.Summary.AccountCount, &result.Summary.LinkedCount,
			&result.Summary.NewCount, &result.Summary.MetricItemCount, &metricsJSON,
		)
	}
	if err != nil {
		return fmt.Errorf("query latest Guorai %s summary: %w", result.EntityType, err)
	}
	if err := json.Unmarshal(metricsJSON, &result.Summary.Metrics); err != nil {
		return fmt.Errorf("decode latest Guorai %s summary metrics: %w", result.EntityType, err)
	}
	return nil
}

func (p *Postgres) loadGuoraiLatestNotes(ctx context.Context, query model.GuoraiLatestQuery, result *model.GuoraiLatestResult) error {
	searchPattern := "%" + strings.TrimSpace(query.Search) + "%"
	offset := (query.Page - 1) * query.PageSize
	rows, err := p.pool.Query(ctx, `
		SELECT notes.note_id, notes.note_name, notes.note_author_name, notes.account_name,
			COALESCE(notes.note_publish_time::TEXT, ''), notes.note_pic, notes.spu_id, notes.spu_name,
			notes.tag, COALESCE(notes.note_type, 0), COALESCE(snapshots.is_new, FALSE),
			jsonb_build_object(
				'total_pay_amount', snapshots.total_pay_amt::DOUBLE PRECISION,
				'part_pay_amount', snapshots.part_pay_amt::DOUBLE PRECISION,
				'ad_cost', snapshots.note_ad_cost_volume::DOUBLE PRECISION,
				'click_count', snapshots.click_count,
				'interaction_count', snapshots.interact_count,
				'total_roi', snapshots.total_roi::DOUBLE PRECISION
			), COUNT(*) OVER()::INTEGER
		FROM guorai_note_snapshots snapshots
		JOIN guorai_notes notes USING (note_id)
		WHERE snapshots.fetch_id=$1
		  AND COALESCE(BTRIM(notes.note_author_name), '') NOT ILIKE $6
		  AND notes.spu_name ILIKE $7
		  AND (notes.note_id ILIKE $2 OR notes.note_name ILIKE $2 OR notes.note_author_name ILIKE $2
			OR notes.account_name ILIKE $2 OR notes.spu_name ILIKE $2 OR notes.tag ILIKE $2)
		ORDER BY
			CASE WHEN $5='payment' THEN snapshots.total_pay_amt END DESC NULLS LAST,
			CASE WHEN $5='cost' THEN snapshots.note_ad_cost_volume END DESC NULLS LAST,
			CASE WHEN $5='roi' THEN snapshots.total_roi END DESC NULLS LAST,
			CASE WHEN $5='publish_time' THEN notes.note_publish_time END DESC NULLS LAST,
			notes.note_publish_time DESC NULLS LAST, notes.note_id
		LIMIT $3 OFFSET $4
	`, result.Snapshot.FetchID, searchPattern, query.PageSize, offset, query.Sort, guoraiExcludedNoteAuthor, "%"+query.SPU+"%")
	if err != nil {
		return fmt.Errorf("query latest Guorai notes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item model.GuoraiLatestItem
		var metricsJSON []byte
		if err := rows.Scan(
			&item.ID, &item.Name, &item.AuthorName, &item.AccountName, &item.PublishTime,
			&item.PictureURL, &item.SPUID, &item.SPUName, &item.Tag, &item.NoteType,
			&item.IsNew, &metricsJSON, &result.Total,
		); err != nil {
			return fmt.Errorf("scan latest Guorai note: %w", err)
		}
		if err := json.Unmarshal(metricsJSON, &item.Metrics); err != nil {
			return fmt.Errorf("decode latest Guorai note metrics: %w", err)
		}
		item.URL = "https://www.xiaohongshu.com/explore/" + url.PathEscape(item.ID)
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate latest Guorai notes: %w", err)
	}
	return nil
}

func (p *Postgres) loadGuoraiLatestPlans(ctx context.Context, query model.GuoraiLatestQuery, result *model.GuoraiLatestResult) error {
	searchPattern := "%" + strings.TrimSpace(query.Search) + "%"
	offset := (query.Page - 1) * query.PageSize
	rows, err := p.pool.Query(ctx, `
		WITH latest_report AS (
			SELECT MAX(report_date) AS report_date
			FROM maituo_customer_daily_subaccounts
			WHERE deleted_at IS NULL
		), selected_accounts AS (
			SELECT DISTINCT LOWER(BTRIM(report_rows.subaccount)) AS account_key
			FROM maituo_customer_daily_subaccounts report_rows
			JOIN latest_report ON latest_report.report_date=report_rows.report_date
			WHERE report_rows.deleted_at IS NULL AND report_rows.spu=$7
			  AND NULLIF(BTRIM(report_rows.subaccount), '') IS NOT NULL
		)
		SELECT plans.plan_id, plans.plan_name, COALESCE(
			NULLIF(BTRIM(plans.account_name), ''),
			NULLIF(BTRIM(delivery_account.advertiser_name), ''),
			''
		),
			COALESCE(plans.plan_publish_time::TEXT, ''), plans.tag, plans.plan_type,
			COALESCE(links.note_count, 0)::INTEGER, COALESCE(snapshots.is_new, FALSE),
			jsonb_build_object(
				'total_pay_amount', snapshots.total_pay_amt::DOUBLE PRECISION,
				'part_pay_amount', snapshots.part_pay_amt::DOUBLE PRECISION,
				'ad_cost', snapshots.note_ad_cost_volume::DOUBLE PRECISION,
				'click_count', snapshots.click_count,
				'interaction_count', snapshots.interact_count,
				'total_roi', snapshots.total_roi::DOUBLE PRECISION
			), COUNT(*) OVER()::INTEGER
		FROM guorai_plan_snapshots snapshots
		JOIN guorai_plans plans USING (plan_id)
		LEFT JOIN LATERAL (
			SELECT advertisers.advertiser_name
			FROM xhs_jg_campaigns campaigns
			JOIN xhs_jg_advertisers advertisers USING (advertiser_id)
			WHERE campaigns.campaign_id=CASE
				WHEN plans.plan_id ~ '^[0-9]+$' THEN plans.plan_id::BIGINT
			END
			  AND campaigns.deleted_at IS NULL
			ORDER BY campaigns.last_seen_at DESC, campaigns.advertiser_id
			LIMIT 1
		) delivery_account ON TRUE
		JOIN selected_accounts ON selected_accounts.account_key=LOWER(COALESCE(NULLIF(BTRIM(plans.account_name), ''), NULLIF(BTRIM(delivery_account.advertiser_name), '')))
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS note_count
			FROM guorai_plan_notes current_links
			JOIN guorai_notes linked_notes ON linked_notes.note_id=current_links.note_id
			WHERE current_links.plan_id=plans.plan_id AND current_links.is_active
			  AND COALESCE(BTRIM(linked_notes.note_author_name), '') NOT ILIKE $6
		) links ON TRUE
		WHERE snapshots.fetch_id=$1
		  AND (plans.plan_id ILIKE $2 OR plans.plan_name ILIKE $2 OR plans.account_name ILIKE $2
			OR delivery_account.advertiser_name ILIKE $2
			OR plans.plan_type ILIKE $2 OR plans.tag ILIKE $2)
		ORDER BY
			CASE WHEN $5='payment' THEN snapshots.total_pay_amt END DESC NULLS LAST,
			CASE WHEN $5='cost' THEN snapshots.note_ad_cost_volume END DESC NULLS LAST,
			CASE WHEN $5='roi' THEN snapshots.total_roi END DESC NULLS LAST,
			CASE WHEN $5='publish_time' THEN plans.plan_publish_time END DESC NULLS LAST,
			plans.plan_publish_time DESC NULLS LAST, plans.plan_id
		LIMIT $3 OFFSET $4
	`, result.Snapshot.FetchID, searchPattern, query.PageSize, offset, query.Sort, guoraiExcludedNoteAuthor, query.SPU)
	if err != nil {
		return fmt.Errorf("query latest Guorai plans: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item model.GuoraiLatestItem
		var metricsJSON []byte
		if err := rows.Scan(
			&item.ID, &item.Name, &item.AccountName, &item.PublishTime, &item.Tag,
			&item.PlanType, &item.LinkedNoteCount, &item.IsNew, &metricsJSON, &result.Total,
		); err != nil {
			return fmt.Errorf("scan latest Guorai plan: %w", err)
		}
		if err := json.Unmarshal(metricsJSON, &item.Metrics); err != nil {
			return fmt.Errorf("decode latest Guorai plan metrics: %w", err)
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate latest Guorai plans: %w", err)
	}
	return nil
}
