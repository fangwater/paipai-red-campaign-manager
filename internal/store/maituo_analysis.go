package store

import (
	"context"
	"fmt"
	"math"
	"strings"

	"paipai-red-campaign-manager/internal/maituo"
)

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
	`, result.ReportDates, searchPattern, query.PageSize, offset, latestReportDate, query.Sort)
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
