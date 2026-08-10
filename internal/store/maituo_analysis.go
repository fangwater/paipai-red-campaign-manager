package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"

	"paipai-red-campaign-manager/internal/maituo"
	"paipai-red-campaign-manager/internal/model"

	"github.com/jackc/pgx/v5"
)

func (p *Postgres) MaituoNoteContent(ctx context.Context, noteID string) (maituo.NoteContent, error) {
	result := maituo.NoteContent{
		NoteID:           noteID,
		NoteURL:          "https://www.xiaohongshu.com/explore/" + url.PathEscape(noteID),
		Blocks:           []model.ManuscriptBlock{},
		ReferenceNoteIDs: []string{},
		Providers:        []string{},
		Tags: maituo.NoteTags{
			NoteType:            []string{},
			CoverType:           []string{},
			CommercialIntensity: []string{},
			Audience:            []string{},
			UserScenario:        []string{},
			Progress:            []string{},
			MissingFields:       []string{},
		},
	}
	var blocksJSON string
	err := p.pool.QueryRow(ctx, `
		SELECT notes.note_content, notes.content_blocks::TEXT, notes.reference_note_ids,
			COALESCE(
				ARRAY_AGG(DISTINCT tables.provider_name ORDER BY tables.provider_name)
					FILTER (WHERE tables.provider_name IS NOT NULL),
				'{}'::TEXT[]
			),
			COALESCE(
				ARRAY_AGG(DISTINCT BTRIM(executions.note_type) ORDER BY BTRIM(executions.note_type))
					FILTER (WHERE NULLIF(BTRIM(executions.note_type), '') IS NOT NULL),
				'{}'::TEXT[]
			),
			COALESCE(
				ARRAY_AGG(DISTINCT BTRIM(executions.cover_type) ORDER BY BTRIM(executions.cover_type))
					FILTER (WHERE NULLIF(BTRIM(executions.cover_type), '') IS NOT NULL),
				'{}'::TEXT[]
			),
			COALESCE(
				ARRAY_AGG(DISTINCT BTRIM(executions.commercial_intensity) ORDER BY BTRIM(executions.commercial_intensity))
					FILTER (WHERE NULLIF(BTRIM(executions.commercial_intensity), '') IS NOT NULL),
				'{}'::TEXT[]
			),
			COALESCE(
				ARRAY_AGG(DISTINCT BTRIM(executions.audience) ORDER BY BTRIM(executions.audience))
					FILTER (WHERE NULLIF(BTRIM(executions.audience), '') IS NOT NULL),
				'{}'::TEXT[]
			),
			COALESCE(
				ARRAY_AGG(DISTINCT BTRIM(executions.user_scenario) ORDER BY BTRIM(executions.user_scenario))
					FILTER (WHERE NULLIF(BTRIM(executions.user_scenario), '') IS NOT NULL),
				'{}'::TEXT[]
			),
			COALESCE(
				ARRAY_AGG(DISTINCT BTRIM(executions.progress) ORDER BY BTRIM(executions.progress))
					FILTER (WHERE NULLIF(BTRIM(executions.progress), '') IS NOT NULL),
				'{}'::TEXT[]
			)
		FROM service_provider_notes notes
		LEFT JOIN service_provider_note_executions executions
		  ON executions.note_id=notes.note_id AND executions.deleted_at IS NULL
		LEFT JOIN service_provider_content_tables tables
		  ON tables.provider_code=executions.provider_code
		WHERE notes.note_id=$1
		GROUP BY notes.note_id, notes.note_content, notes.content_blocks, notes.reference_note_ids
	`, noteID).Scan(
		&result.NoteContent, &blocksJSON, &result.ReferenceNoteIDs, &result.Providers,
		&result.Tags.NoteType, &result.Tags.CoverType, &result.Tags.CommercialIntensity,
		&result.Tags.Audience, &result.Tags.UserScenario, &result.Tags.Progress,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("query Maituo note content: %w", err)
	}
	if err := json.Unmarshal([]byte(blocksJSON), &result.Blocks); err != nil {
		return result, fmt.Errorf("decode Maituo note content blocks: %w", err)
	}
	completeNoteTags(&result.Tags)
	result.Found = true
	return result, nil
}

func completeNoteTags(tags *maituo.NoteTags) {
	tags.MissingFields = []string{}
	fields := []struct {
		key    string
		values []string
	}{
		{key: "note_type", values: tags.NoteType},
		{key: "cover_type", values: tags.CoverType},
		{key: "commercial_intensity", values: tags.CommercialIntensity},
		{key: "audience", values: tags.Audience},
		{key: "user_scenario", values: tags.UserScenario},
		{key: "progress", values: tags.Progress},
	}
	for _, field := range fields {
		if len(field.values) == 0 {
			tags.MissingFields = append(tags.MissingFields, field.key)
		}
	}
	tags.Complete = len(tags.MissingFields) == 0
}

func (p *Postgres) MaituoNoteCampaignAnalysis(ctx context.Context, query maituo.NoteCampaignAnalysisQuery) (maituo.NoteCampaignAnalysis, error) {
	if query.Sort == "" {
		query.Sort = "cumulative_spend"
	}
	result := maituo.NoteCampaignAnalysis{
		Window: query.Window, Sort: query.Sort, Page: query.Page, PageSize: query.PageSize,
		ReportDates: []string{}, Items: []maituo.NoteCampaignAnalysisItem{},
	}
	dateLimit := 0
	switch query.Window {
	case "3d":
		dateLimit = 3
	case "7d":
		dateLimit = 7
	}

	dateRows, err := p.pool.Query(ctx, `
		SELECT report_date::TEXT
		FROM (
			SELECT DISTINCT report_date
			FROM maituo_customer_daily_notes
			WHERE deleted_at IS NULL
			ORDER BY report_date DESC
			LIMIT NULLIF($1, 0)
		) selected
		ORDER BY report_date
	`, dateLimit)
	if err != nil {
		return result, fmt.Errorf("query Maituo analysis dates: %w", err)
	}
	for dateRows.Next() {
		var date string
		if err := dateRows.Scan(&date); err != nil {
			dateRows.Close()
			return result, fmt.Errorf("scan Maituo analysis date: %w", err)
		}
		result.ReportDates = append(result.ReportDates, date)
	}
	if err := dateRows.Err(); err != nil {
		dateRows.Close()
		return result, fmt.Errorf("iterate Maituo analysis dates: %w", err)
	}
	dateRows.Close()
	if len(result.ReportDates) == 0 {
		return result, nil
	}

	searchPattern := "%" + strings.TrimSpace(query.Search) + "%"
	offset := (query.Page - 1) * query.PageSize
	latestReportDate := result.ReportDates[len(result.ReportDates)-1]
	rows, err := p.pool.Query(ctx, `
		WITH selected_dates AS (
			SELECT value::DATE AS report_date FROM unnest($1::TEXT[]) value
		), daily AS (
			SELECT notes.report_date, notes.note_id, notes.campaign_name, notes.placement,
				SUM(notes.spend)::DOUBLE PRECISION AS spend,
				SUM(notes.search_users)::BIGINT AS search_users,
				SUM(COALESCE(notes.search_cost, 0))::DOUBLE PRECISION AS search_cost
			FROM maituo_customer_daily_notes notes
			JOIN selected_dates dates USING (report_date)
			WHERE notes.deleted_at IS NULL
			  AND ($2 = '%%' OR notes.note_id ILIKE $2 OR notes.campaign_name ILIKE $2 OR notes.placement ILIKE $2)
			  AND ($7 = '' OR EXISTS (
				SELECT 1
				FROM guorai_plan_notes links
				WHERE links.plan_id=$7 AND links.note_id=notes.note_id AND links.is_active
			  ))
			GROUP BY notes.report_date, notes.note_id, notes.campaign_name, notes.placement
		), summaries AS (
			SELECT note_id, campaign_name, placement,
				MIN(report_date)::TEXT AS first_report_date,
				MAX(report_date)::TEXT AS last_report_date,
				COUNT(*)::INTEGER AS active_days,
				COALESCE(MAX(spend) FILTER (WHERE report_date = $5::DATE), 0)::DOUBLE PRECISION AS latest_spend,
				SUM(spend)::DOUBLE PRECISION AS total_spend,
				SUM(search_users)::BIGINT AS total_search_users,
				COALESCE(MAX(search_cost) FILTER (WHERE report_date = $5::DATE), 0)::DOUBLE PRECISION AS latest_search_cost
			FROM daily
			GROUP BY note_id, campaign_name, placement
		)
		SELECT note_id, campaign_name, placement, first_report_date, last_report_date,
			active_days, latest_spend, total_spend, total_search_users, latest_search_cost,
			COUNT(*) OVER()::INTEGER AS total_count
		FROM summaries
		ORDER BY
			CASE WHEN $6 = 'daily_spend' THEN latest_spend END DESC,
			CASE WHEN $6 = 'cumulative_spend' THEN total_spend END DESC,
			total_spend DESC, total_search_users DESC, note_id, campaign_name, placement
		LIMIT $3 OFFSET $4
	`, result.ReportDates, searchPattern, query.PageSize, offset, latestReportDate, query.Sort, query.PlanID)
	if err != nil {
		return result, fmt.Errorf("query Maituo note campaign summaries: %w", err)
	}
	for rows.Next() {
		var item maituo.NoteCampaignAnalysisItem
		if err := rows.Scan(
			&item.NoteID, &item.CampaignName, &item.Placement,
			&item.FirstReportDate, &item.LastReportDate, &item.ActiveDays,
			&item.LatestSpend, &item.TotalSpend, &item.TotalSearchUsers, &item.LatestSearchCost, &result.Total,
		); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan Maituo note campaign summary: %w", err)
		}
		item.LatestSpend = roundMaituoMoney(item.LatestSpend)
		item.TotalSpend = roundMaituoMoney(item.TotalSpend)
		item.LatestSearchCost = roundMaituoMoney(item.LatestSearchCost)
		item.Points = []maituo.NoteCampaignPoint{}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate Maituo note campaign summaries: %w", err)
	}
	rows.Close()
	if len(result.Items) == 0 {
		return result, nil
	}

	noteIDs := make([]string, len(result.Items))
	campaignNames := make([]string, len(result.Items))
	placements := make([]string, len(result.Items))
	for index, item := range result.Items {
		noteIDs[index], campaignNames[index], placements[index] = item.NoteID, item.CampaignName, item.Placement
	}
	pointRows, err := p.pool.Query(ctx, `
		WITH selected_dates AS (
			SELECT value::DATE AS report_date FROM unnest($1::TEXT[]) value
		), selected_keys AS (
			SELECT * FROM unnest($2::TEXT[], $3::TEXT[], $4::TEXT[])
				WITH ORDINALITY AS key(note_id, campaign_name, placement, ordinal)
		), daily AS (
			SELECT notes.report_date, notes.note_id, notes.campaign_name, notes.placement,
				SUM(notes.spend)::DOUBLE PRECISION AS spend,
				SUM(notes.search_users)::BIGINT AS search_users,
				SUM(COALESCE(notes.search_cost, 0))::DOUBLE PRECISION AS search_cost
			FROM maituo_customer_daily_notes notes
			JOIN selected_dates dates USING (report_date)
			JOIN selected_keys key USING (note_id, campaign_name, placement)
			WHERE notes.deleted_at IS NULL
			GROUP BY notes.report_date, notes.note_id, notes.campaign_name, notes.placement
		)
		SELECT key.ordinal::INTEGER, dates.report_date::TEXT,
			COALESCE(daily.spend, 0)::DOUBLE PRECISION,
			COALESCE(daily.search_users, 0)::BIGINT,
			COALESCE(daily.search_cost, 0)::DOUBLE PRECISION
		FROM selected_keys key
		CROSS JOIN selected_dates dates
		LEFT JOIN daily ON daily.report_date = dates.report_date
			AND daily.note_id = key.note_id
			AND daily.campaign_name = key.campaign_name
			AND daily.placement = key.placement
		ORDER BY key.ordinal, dates.report_date
	`, result.ReportDates, noteIDs, campaignNames, placements)
	if err != nil {
		return result, fmt.Errorf("query Maituo note campaign points: %w", err)
	}
	cumulativeSpend := make([]float64, len(result.Items))
	cumulativeUsers := make([]int64, len(result.Items))
	for pointRows.Next() {
		var ordinal int
		var point maituo.NoteCampaignPoint
		if err := pointRows.Scan(&ordinal, &point.ReportDate, &point.Spend, &point.SearchUsers, &point.SearchCost); err != nil {
			pointRows.Close()
			return result, fmt.Errorf("scan Maituo note campaign point: %w", err)
		}
		point.Spend = roundMaituoMoney(point.Spend)
		point.SearchCost = roundMaituoMoney(point.SearchCost)
		index := ordinal - 1
		cumulativeSpend[index] += point.Spend
		cumulativeUsers[index] += point.SearchUsers
		point.CumulativeSpend = roundMaituoMoney(cumulativeSpend[index])
		point.CumulativeUsers = cumulativeUsers[index]
		result.Items[index].Points = append(result.Items[index].Points, point)
	}
	if err := pointRows.Err(); err != nil {
		pointRows.Close()
		return result, fmt.Errorf("iterate Maituo note campaign points: %w", err)
	}
	pointRows.Close()
	return result, nil
}

func roundMaituoMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func (p *Postgres) MaituoTrafficComparison(ctx context.Context, query maituo.TrafficComparisonQuery) (maituo.TrafficComparison, error) {
	result := maituo.TrafficComparison{
		Window: query.Window, Page: query.Page, PageSize: query.PageSize,
		ReportDates: []string{}, Items: []maituo.TrafficComparisonItem{},
	}
	dateLimit := 0
	switch query.Window {
	case "3d":
		dateLimit = 3
	case "7d":
		dateLimit = 7
	}

	dateRows, err := p.pool.Query(ctx, `
		SELECT report_date::TEXT
		FROM (
			SELECT DISTINCT report_date
			FROM maituo_customer_daily_notes
			WHERE deleted_at IS NULL
			ORDER BY report_date DESC
			LIMIT NULLIF($1, 0)
		) selected
		ORDER BY report_date
	`, dateLimit)
	if err != nil {
		return result, fmt.Errorf("query Maituo traffic comparison dates: %w", err)
	}
	for dateRows.Next() {
		var date string
		if err := dateRows.Scan(&date); err != nil {
			dateRows.Close()
			return result, fmt.Errorf("scan Maituo traffic comparison date: %w", err)
		}
		result.ReportDates = append(result.ReportDates, date)
	}
	if err := dateRows.Err(); err != nil {
		dateRows.Close()
		return result, fmt.Errorf("iterate Maituo traffic comparison dates: %w", err)
	}
	dateRows.Close()
	if len(result.ReportDates) == 0 {
		return result, nil
	}

	result.LatestDate = result.ReportDates[len(result.ReportDates)-1]
	searchPattern := "%" + strings.TrimSpace(query.Search) + "%"
	offset := (query.Page - 1) * query.PageSize
	rows, err := p.pool.Query(ctx, `
		WITH latest_daily AS (
			SELECT notes.note_id, notes.campaign_name, notes.placement,
				SUM(notes.spend)::DOUBLE PRECISION AS latest_spend,
				SUM(notes.search_users)::BIGINT AS latest_search_users,
				CASE WHEN SUM(notes.search_users) > 0
					THEN SUM(COALESCE(notes.search_cost, 0))::DOUBLE PRECISION
					ELSE NULL
				END AS latest_search_cost
			FROM maituo_customer_daily_notes notes
			WHERE notes.deleted_at IS NULL
			  AND notes.report_date = $1::DATE
			GROUP BY notes.note_id, notes.campaign_name, notes.placement
		), matching_groups AS (
			SELECT note_id, placement
			FROM latest_daily
			GROUP BY note_id, placement
			HAVING $2 = '%%' OR note_id ILIKE $2 OR placement ILIKE $2 OR BOOL_OR(campaign_name ILIKE $2)
		), summaries AS (
			SELECT daily.note_id, daily.placement,
				COUNT(*)::INTEGER AS campaign_count,
				COUNT(daily.latest_search_cost)::INTEGER AS comparable_campaign_count,
				COALESCE(MIN(daily.latest_search_cost), 0)::DOUBLE PRECISION AS latest_search_cost_min,
				COALESCE(MAX(daily.latest_search_cost), 0)::DOUBLE PRECISION AS latest_search_cost_max,
				CASE WHEN COUNT(daily.latest_search_cost) >= 2
					THEN (MAX(daily.latest_search_cost) - MIN(daily.latest_search_cost))::DOUBLE PRECISION
					ELSE 0::DOUBLE PRECISION
				END AS search_cost_gap,
				SUM(daily.latest_spend)::DOUBLE PRECISION AS latest_spend,
				SUM(daily.latest_search_users)::BIGINT AS latest_search_users
			FROM latest_daily daily
			JOIN matching_groups matching USING (note_id, placement)
			GROUP BY daily.note_id, daily.placement
		)
		SELECT note_id, placement, campaign_count, comparable_campaign_count,
			latest_search_cost_min, latest_search_cost_max, search_cost_gap,
			latest_spend, latest_search_users,
			COUNT(*) OVER()::INTEGER AS total_count
		FROM summaries
		ORDER BY search_cost_gap DESC, latest_search_cost_max DESC,
			latest_spend DESC, latest_search_users DESC, note_id, placement
		LIMIT $3 OFFSET $4
	`, result.LatestDate, searchPattern, query.PageSize, offset)
	if err != nil {
		return result, fmt.Errorf("query Maituo traffic comparison summaries: %w", err)
	}
	for rows.Next() {
		var item maituo.TrafficComparisonItem
		if err := rows.Scan(
			&item.NoteID, &item.Placement, &item.CampaignCount, &item.ComparableCampaignCount,
			&item.LatestSearchCostMin, &item.LatestSearchCostMax, &item.SearchCostGap,
			&item.LatestSpend, &item.LatestSearchUsers, &result.Total,
		); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan Maituo traffic comparison summary: %w", err)
		}
		item.LatestSearchCostMin = roundMaituoMoney(item.LatestSearchCostMin)
		item.LatestSearchCostMax = roundMaituoMoney(item.LatestSearchCostMax)
		item.SearchCostGap = roundMaituoMoney(item.SearchCostGap)
		item.LatestSpend = roundMaituoMoney(item.LatestSpend)
		item.Campaigns = []maituo.TrafficComparisonCampaign{}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate Maituo traffic comparison summaries: %w", err)
	}
	rows.Close()
	if len(result.Items) == 0 {
		return result, nil
	}

	noteIDs := make([]string, len(result.Items))
	placements := make([]string, len(result.Items))
	for index, item := range result.Items {
		noteIDs[index], placements[index] = item.NoteID, item.Placement
	}
	campaignRows, err := p.pool.Query(ctx, `
		WITH selected_dates AS (
			SELECT value::DATE AS report_date FROM unnest($1::TEXT[]) value
		), selected_groups AS (
			SELECT * FROM unnest($2::TEXT[], $3::TEXT[])
				WITH ORDINALITY AS selected(note_id, placement, ordinal)
		), daily AS (
			SELECT selected.ordinal, notes.report_date, notes.note_id, notes.campaign_name, notes.placement,
				SUM(notes.spend)::DOUBLE PRECISION AS spend,
				SUM(notes.search_users)::BIGINT AS search_users,
				CASE WHEN SUM(notes.search_users) > 0
					THEN SUM(COALESCE(notes.search_cost, 0))::DOUBLE PRECISION
					ELSE NULL
				END AS search_cost
			FROM maituo_customer_daily_notes notes
			JOIN selected_dates dates USING (report_date)
			JOIN selected_groups selected USING (note_id, placement)
			WHERE notes.deleted_at IS NULL
			GROUP BY selected.ordinal, notes.report_date, notes.note_id, notes.campaign_name, notes.placement
		), current_campaigns AS (
			SELECT DISTINCT ordinal, note_id, campaign_name, placement
			FROM daily
			WHERE report_date = $4::DATE
		), summaries AS (
			SELECT current.ordinal, current.note_id, current.campaign_name, current.placement,
				MIN(daily.report_date)::TEXT AS first_report_date,
				MAX(daily.report_date)::TEXT AS last_report_date,
				COUNT(*)::INTEGER AS active_days,
				COALESCE(MAX(daily.spend) FILTER (WHERE daily.report_date = $4::DATE), 0)::DOUBLE PRECISION AS latest_spend,
				COALESCE(MAX(daily.search_users) FILTER (WHERE daily.report_date = $4::DATE), 0)::BIGINT AS latest_search_users,
				COALESCE(MAX(daily.search_cost) FILTER (WHERE daily.report_date = $4::DATE), 0)::DOUBLE PRECISION AS latest_search_cost,
				(MAX(daily.search_cost) FILTER (WHERE daily.report_date = $4::DATE) IS NOT NULL) AS has_latest_search_cost,
				SUM(daily.spend)::DOUBLE PRECISION AS total_spend,
				SUM(daily.search_users)::BIGINT AS total_search_users
			FROM current_campaigns current
			JOIN daily USING (ordinal, note_id, campaign_name, placement)
			GROUP BY current.ordinal, current.note_id, current.campaign_name, current.placement
		)
		SELECT ordinal::INTEGER, campaign_name, first_report_date, last_report_date,
			active_days, latest_spend, latest_search_users, latest_search_cost, has_latest_search_cost,
			total_spend, total_search_users
		FROM summaries
		ORDER BY ordinal, latest_search_cost DESC, latest_spend DESC, campaign_name
	`, result.ReportDates, noteIDs, placements, result.LatestDate)
	if err != nil {
		return result, fmt.Errorf("query Maituo traffic comparison campaigns: %w", err)
	}
	for campaignRows.Next() {
		var ordinal int
		var campaign maituo.TrafficComparisonCampaign
		if err := campaignRows.Scan(
			&ordinal, &campaign.CampaignName, &campaign.FirstReportDate, &campaign.LastReportDate,
			&campaign.ActiveDays, &campaign.LatestSpend, &campaign.LatestSearchUsers,
			&campaign.LatestSearchCost, &campaign.HasLatestSearchCost, &campaign.TotalSpend, &campaign.TotalSearchUsers,
		); err != nil {
			campaignRows.Close()
			return result, fmt.Errorf("scan Maituo traffic comparison campaign: %w", err)
		}
		campaign.LatestSpend = roundMaituoMoney(campaign.LatestSpend)
		campaign.LatestSearchCost = roundMaituoMoney(campaign.LatestSearchCost)
		campaign.TotalSpend = roundMaituoMoney(campaign.TotalSpend)
		campaign.Points = []maituo.TrafficComparisonPoint{}
		result.Items[ordinal-1].Campaigns = append(result.Items[ordinal-1].Campaigns, campaign)
	}
	if err := campaignRows.Err(); err != nil {
		campaignRows.Close()
		return result, fmt.Errorf("iterate Maituo traffic comparison campaigns: %w", err)
	}
	campaignRows.Close()

	pointNoteIDs := []string{}
	pointCampaignNames := []string{}
	pointPlacements := []string{}
	campaignPointers := []*maituo.TrafficComparisonCampaign{}
	for itemIndex := range result.Items {
		for campaignIndex := range result.Items[itemIndex].Campaigns {
			campaign := &result.Items[itemIndex].Campaigns[campaignIndex]
			pointNoteIDs = append(pointNoteIDs, result.Items[itemIndex].NoteID)
			pointCampaignNames = append(pointCampaignNames, campaign.CampaignName)
			pointPlacements = append(pointPlacements, result.Items[itemIndex].Placement)
			campaignPointers = append(campaignPointers, campaign)
		}
	}
	if len(campaignPointers) == 0 {
		return result, nil
	}

	pointRows, err := p.pool.Query(ctx, `
		WITH selected_dates AS (
			SELECT value::DATE AS report_date FROM unnest($1::TEXT[]) value
		), selected_keys AS (
			SELECT * FROM unnest($2::TEXT[], $3::TEXT[], $4::TEXT[])
				WITH ORDINALITY AS selected(note_id, campaign_name, placement, ordinal)
		), daily AS (
			SELECT notes.report_date, notes.note_id, notes.campaign_name, notes.placement,
				SUM(notes.spend)::DOUBLE PRECISION AS spend,
				SUM(notes.search_users)::BIGINT AS search_users,
				CASE WHEN SUM(notes.search_users) > 0
					THEN SUM(COALESCE(notes.search_cost, 0))::DOUBLE PRECISION
					ELSE NULL
				END AS search_cost
			FROM maituo_customer_daily_notes notes
			JOIN selected_dates dates USING (report_date)
			JOIN selected_keys selected USING (note_id, campaign_name, placement)
			WHERE notes.deleted_at IS NULL
			GROUP BY notes.report_date, notes.note_id, notes.campaign_name, notes.placement
		)
		SELECT selected.ordinal::INTEGER, dates.report_date::TEXT,
			COALESCE(daily.spend, 0)::DOUBLE PRECISION,
			COALESCE(daily.search_users, 0)::BIGINT,
			COALESCE(daily.search_cost, 0)::DOUBLE PRECISION,
			(daily.search_cost IS NOT NULL) AS has_search_cost
		FROM selected_keys selected
		CROSS JOIN selected_dates dates
		LEFT JOIN daily ON daily.report_date = dates.report_date
			AND daily.note_id = selected.note_id
			AND daily.campaign_name = selected.campaign_name
			AND daily.placement = selected.placement
		ORDER BY selected.ordinal, dates.report_date
	`, result.ReportDates, pointNoteIDs, pointCampaignNames, pointPlacements)
	if err != nil {
		return result, fmt.Errorf("query Maituo traffic comparison points: %w", err)
	}
	for pointRows.Next() {
		var ordinal int
		var point maituo.TrafficComparisonPoint
		if err := pointRows.Scan(&ordinal, &point.ReportDate, &point.Spend, &point.SearchUsers, &point.SearchCost, &point.HasSearchCost); err != nil {
			pointRows.Close()
			return result, fmt.Errorf("scan Maituo traffic comparison point: %w", err)
		}
		point.Spend = roundMaituoMoney(point.Spend)
		point.SearchCost = roundMaituoMoney(point.SearchCost)
		campaignPointers[ordinal-1].Points = append(campaignPointers[ordinal-1].Points, point)
	}
	if err := pointRows.Err(); err != nil {
		pointRows.Close()
		return result, fmt.Errorf("iterate Maituo traffic comparison points: %w", err)
	}
	pointRows.Close()
	return result, nil
}

func (p *Postgres) MaituoTrafficDeliveryComparison(ctx context.Context, query maituo.TrafficDeliveryComparisonQuery) (maituo.TrafficDeliveryComparison, error) {
	result := maituo.TrafficDeliveryComparison{
		NoteID: query.NoteID, Placement: query.Placement, Campaigns: []maituo.TrafficDeliveryCampaign{},
	}
	page := 1
	for {
		links, err := p.MaituoXHSLinks(ctx, maituo.XHSLinkQuery{Search: query.NoteID, Page: page, PageSize: 100})
		if err != nil {
			return result, fmt.Errorf("query traffic delivery links: %w", err)
		}
		result.ReportDate = links.ReportDate
		for _, item := range links.Items {
			if item.NoteID != query.NoteID || item.Placement != query.Placement {
				continue
			}
			result.Campaigns = append(result.Campaigns, maituo.TrafficDeliveryCampaign{
				CampaignName: item.CampaignName,
				Subaccounts:  item.Subaccounts,
				Matches:      item.Matches,
			})
		}
		if page*links.PageSize >= links.Total || len(links.Items) == 0 {
			break
		}
		page++
	}
	return result, nil
}
