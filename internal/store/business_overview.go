package store

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"paipai-red-campaign-manager/internal/model"

	"github.com/jackc/pgx/v5/pgtype"
)

const overviewSPU = "辅酶"

var overviewAgencyOrder = []string{"智元", "曼杰", "引响", "飓风", "有一有二"}

type overviewTrendRow struct {
	Date       string
	Spend      *float64
	SearchUV   *float64
	OrderUV    *float64
	SearchCost *float64
}

func (p *Postgres) BusinessOverview(ctx context.Context, days int, spu string) (model.BusinessOverview, error) {
	if spu == "" {
		spu = overviewSPU
	}
	result := model.BusinessOverview{Days: days, SPU: spu}
	trend, err := p.loadBusinessOverviewTrend(ctx, days, spu)
	if err != nil {
		return result, err
	}
	result.Trend = trend
	newNotes, err := p.loadBusinessOverviewNotes(ctx, days, spu)
	if err != nil {
		return result, err
	}
	result.NewNotes = newNotes
	return result, nil
}

func (p *Postgres) loadBusinessOverviewTrend(ctx context.Context, days int, spu string) (model.OverviewTrend, error) {
	var latestDate string
	if err := p.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(report_date)::TEXT, '')
		FROM maituo_customer_daily_spus
		WHERE deleted_at IS NULL AND spu=$1
	`, spu).Scan(&latestDate); err != nil {
		return model.OverviewTrend{}, fmt.Errorf("query business overview latest trend date: %w", err)
	}
	result := model.OverviewTrend{Metrics: []model.OverviewMetric{}}
	if latestDate == "" {
		return result, nil
	}
	latest, err := time.Parse(time.DateOnly, latestDate)
	if err != nil {
		return result, fmt.Errorf("parse business overview latest trend date: %w", err)
	}
	currentStart := latest.AddDate(0, 0, -(days - 1))
	previousEnd := currentStart.AddDate(0, 0, -1)
	previousStart := previousEnd.AddDate(0, 0, -(days - 1))
	result.StartDate = currentStart.Format(time.DateOnly)
	result.EndDate = latestDate
	result.PreviousStartDate = previousStart.Format(time.DateOnly)
	result.PreviousEndDate = previousEnd.Format(time.DateOnly)

	rows, err := p.pool.Query(ctx, `
		SELECT dates.day::DATE::TEXT,
			spus.auction_spend::DOUBLE PRECISION,
			CASE WHEN $3='辅酶' THEN trends.coenzyme_search_uv ELSE trends.krill_oil_search_uv END::DOUBLE PRECISION,
			CASE WHEN $3='辅酶' THEN trends.coenzyme_order_uv ELSE trends.krill_oil_order_uv END::DOUBLE PRECISION,
			spus.search_cost::DOUBLE PRECISION
		FROM generate_series($1::DATE, $2::DATE, INTERVAL '1 day') dates(day)
		LEFT JOIN maituo_customer_daily_spus spus
		  ON spus.report_date=dates.day::DATE AND spus.spu=$3 AND spus.deleted_at IS NULL
		LEFT JOIN maituo_customer_daily_trends trends
		  ON trends.report_date=dates.day::DATE AND trends.deleted_at IS NULL
		ORDER BY dates.day
	`, result.PreviousStartDate, result.EndDate, spu)
	if err != nil {
		return result, fmt.Errorf("query business overview trend: %w", err)
	}
	defer rows.Close()
	allRows := make([]overviewTrendRow, 0, days*2)
	for rows.Next() {
		var row overviewTrendRow
		var spend, searchUV, orderUV, searchCost pgtype.Float8
		if err := rows.Scan(&row.Date, &spend, &searchUV, &orderUV, &searchCost); err != nil {
			return result, fmt.Errorf("scan business overview trend: %w", err)
		}
		row.Spend = overviewNullableFloat(spend)
		row.SearchUV = overviewNullableFloat(searchUV)
		row.OrderUV = overviewNullableFloat(orderUV)
		row.SearchCost = overviewNullableFloat(searchCost)
		allRows = append(allRows, row)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate business overview trend: %w", err)
	}
	previousRows := allRows[:len(allRows)-days]
	currentRows := allRows[len(allRows)-days:]
	for _, row := range currentRows {
		if row.Spend != nil || row.SearchUV != nil || row.OrderUV != nil || row.SearchCost != nil {
			result.AvailableDays++
		}
	}
	result.Metrics = []model.OverviewMetric{
		overviewMetric("spend", "每日消耗", "currency", currentRows, previousRows, func(row overviewTrendRow) *float64 { return row.Spend }, false),
		overviewMetric("search_cost", "回搜成本", "currency", currentRows, previousRows, func(row overviewTrendRow) *float64 { return row.SearchCost }, true),
		overviewMetric("search_uv", "淘搜 UV", "count", currentRows, previousRows, func(row overviewTrendRow) *float64 { return row.SearchUV }, false),
		overviewMetric("order_uv", "成交 UV", "count", currentRows, previousRows, func(row overviewTrendRow) *float64 { return row.OrderUV }, false),
	}
	return result, nil
}

func overviewMetric(
	key, label, unit string,
	currentRows, previousRows []overviewTrendRow,
	value func(overviewTrendRow) *float64,
	average bool,
) model.OverviewMetric {
	metric := model.OverviewMetric{Key: key, Label: label, Unit: unit, Points: make([]model.OverviewMetricPoint, 0, len(currentRows))}
	for _, row := range currentRows {
		metric.Points = append(metric.Points, model.OverviewMetricPoint{Date: row.Date, Value: value(row)})
	}
	metric.CurrentValue = overviewPeriodValue(currentRows, value, average)
	metric.PreviousValue = overviewPeriodValue(previousRows, value, average)
	if metric.CurrentValue != nil && metric.PreviousValue != nil && *metric.PreviousValue != 0 {
		change := (*metric.CurrentValue - *metric.PreviousValue) / *metric.PreviousValue
		metric.ChangePct = &change
	}
	return metric
}

func overviewPeriodValue(rows []overviewTrendRow, value func(overviewTrendRow) *float64, average bool) *float64 {
	total := 0.0
	count := 0
	for _, row := range rows {
		if current := value(row); current != nil {
			total += *current
			count++
		}
	}
	if count == 0 {
		return nil
	}
	if average {
		total /= float64(count)
	}
	rounded := math.Round(total*100) / 100
	return &rounded
}

func overviewNullableFloat(value pgtype.Float8) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

func (p *Postgres) loadBusinessOverviewNotes(ctx context.Context, days int, spu string) (model.OverviewNewNotes, error) {
	result := model.OverviewNewNotes{Daily: []model.OverviewDailyNotes{}, Agencies: []model.OverviewAgency{}}
	for _, agency := range overviewAgencyOrder {
		audienceTags := []string{}
		if agency == "飓风" {
			audienceTags = []string{spu + "选购"}
		}
		result.Agencies = append(result.Agencies, model.OverviewAgency{Agency: agency, AudienceTags: audienceTags, Notes: []model.OverviewNote{}})
	}
	var latestDate string
	if err := p.pool.QueryRow(ctx, `
		WITH base AS (
			SELECT CASE records.fields ->> '下单账号'
				WHEN '杭州智元文化传播有限公司' THEN '智元'
				WHEN '江苏拾光宝盒信息技术有限公司' THEN '曼杰'
				WHEN '引响网络科技（上海）有限公司' THEN '引响'
				WHEN '武汉飓风无限广告有限公司' THEN '飓风'
				WHEN '上海有一有二网络技术有限公司' THEN '有一有二'
			END AS agency,
			CASE WHEN jsonb_typeof(records.fields -> '发布时间')='number'
				THEN (TO_TIMESTAMP((records.fields ->> '发布时间')::DOUBLE PRECISION / 1000) AT TIME ZONE 'Asia/Shanghai')::DATE
			END AS published_date
			FROM lark_bitable_records records
			JOIN lark_bitable_tables tables ON tables.app_token=records.app_token AND tables.table_id=records.table_id
			WHERE records.deleted_at IS NULL AND tables.name='蒲公英数据'
			  AND records.fields ->> 'spu名称' ILIKE $1
		)
		SELECT COALESCE(MAX(published_date)::TEXT, '') FROM base WHERE agency IS NOT NULL
	`, "%"+spu+"%").Scan(&latestDate); err != nil {
		return result, fmt.Errorf("query business overview latest Dandelion date: %w", err)
	}
	if latestDate == "" {
		return result, nil
	}
	latest, err := time.Parse(time.DateOnly, latestDate)
	if err != nil {
		return result, fmt.Errorf("parse business overview latest Dandelion date: %w", err)
	}
	start := latest.AddDate(0, 0, -(days - 1))
	result.StartDate = start.Format(time.DateOnly)
	result.EndDate = latestDate
	dailyIndexes := make(map[string]int, days)
	for offset := 0; offset < days; offset++ {
		date := start.AddDate(0, 0, offset).Format(time.DateOnly)
		dailyIndexes[date] = len(result.Daily)
		result.Daily = append(result.Daily, model.OverviewDailyNotes{Date: date})
	}
	agencyIndexes := make(map[string]int, len(result.Agencies))
	audienceSets := make([]map[string]struct{}, len(result.Agencies))
	for index, agency := range result.Agencies {
		agencyIndexes[agency.Agency] = index
		audienceSets[index] = map[string]struct{}{}
	}

	rows, err := p.pool.Query(ctx, `
		WITH base AS (
			SELECT records.record_id,
				COALESCE(records.fields #>> '{笔记ID,0,text}', records.fields ->> '笔记ID', '') AS note_id,
				COALESCE(records.fields #>> '{笔记标题,0,text}', records.fields ->> '笔记标题', '') AS title,
				COALESCE(records.fields #>> '{笔记链接,0,link}', '') AS note_url,
				COALESCE(records.fields #>> '{达人/发布账号,0,text}', '') AS author,
				COALESCE(records.fields ->> '笔记类型', '') AS note_type,
				COALESCE(records.fields ->> '内容标签', '') AS content_tag,
				CASE records.fields ->> '下单账号'
					WHEN '杭州智元文化传播有限公司' THEN '智元'
					WHEN '江苏拾光宝盒信息技术有限公司' THEN '曼杰'
					WHEN '引响网络科技（上海）有限公司' THEN '引响'
					WHEN '武汉飓风无限广告有限公司' THEN '飓风'
					WHEN '上海有一有二网络技术有限公司' THEN '有一有二'
				END AS agency,
				CASE records.fields ->> '下单账号'
					WHEN '杭州智元文化传播有限公司' THEN 'zhiyuan'
					WHEN '江苏拾光宝盒信息技术有限公司' THEN 'manjie'
					WHEN '上海有一有二网络技术有限公司' THEN 'youyiyouer'
				END AS provider_code,
				CASE WHEN jsonb_typeof(records.fields -> '发布时间')='number'
					THEN (TO_TIMESTAMP((records.fields ->> '发布时间')::DOUBLE PRECISION / 1000) AT TIME ZONE 'Asia/Shanghai')::DATE
				END AS published_date,
				records.synced_at
			FROM lark_bitable_records records
			JOIN lark_bitable_tables tables ON tables.app_token=records.app_token AND tables.table_id=records.table_id
			WHERE records.deleted_at IS NULL AND tables.name='蒲公英数据'
			  AND records.fields ->> 'spu名称' ILIKE $3
		), matched AS (
			SELECT base.*, execution.audience
			FROM base
			LEFT JOIN LATERAL (
				SELECT provider.audience
				FROM service_provider_note_executions provider
				WHERE provider.deleted_at IS NULL AND provider.note_id=base.note_id
				  AND provider.provider_code=base.provider_code
				ORDER BY provider.synced_at DESC
				LIMIT 1
			) execution ON TRUE
			WHERE base.agency IS NOT NULL AND base.note_id <> ''
			  AND base.published_date BETWEEN $1::DATE AND $2::DATE
		), deduplicated AS (
			SELECT DISTINCT ON (note_id) note_id, title, note_url, author, note_type, content_tag,
				agency, published_date, synced_at,
				CASE WHEN agency='飓风' THEN $4 ELSE COALESCE(audience, '') END AS audience
			FROM matched
			ORDER BY note_id, published_date DESC, synced_at DESC
		)
		SELECT note_id, title, note_url, author, note_type, content_tag, agency,
			published_date::TEXT, audience, MAX(synced_at) OVER () AS source_synced_at
		FROM deduplicated
		ORDER BY published_date DESC, agency, note_id
	`, result.StartDate, result.EndDate, "%"+spu+"%", spu+"选购")
	if err != nil {
		return result, fmt.Errorf("query business overview Dandelion notes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var note model.OverviewNote
		var syncedAt *time.Time
		if err := rows.Scan(
			&note.NoteID, &note.Title, &note.URL, &note.Author, &note.NoteType,
			&note.ContentTag, &note.Agency, &note.PublishedDate, &note.Audience, &syncedAt,
		); err != nil {
			return result, fmt.Errorf("scan business overview Dandelion note: %w", err)
		}
		result.SourceSyncedAt = syncedAt
		result.Total++
		if dailyIndex, exists := dailyIndexes[note.PublishedDate]; exists {
			result.Daily[dailyIndex].Count++
		}
		agencyIndex := agencyIndexes[note.Agency]
		result.Agencies[agencyIndex].Count++
		result.Agencies[agencyIndex].Notes = append(result.Agencies[agencyIndex].Notes, note)
		if note.Audience != "" {
			audienceSets[agencyIndex][note.Audience] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate business overview Dandelion notes: %w", err)
	}
	for index := range result.Agencies {
		for audience := range audienceSets[index] {
			if result.Agencies[index].Agency != "飓风" {
				result.Agencies[index].AudienceTags = append(result.Agencies[index].AudienceTags, audience)
			}
		}
		sort.Strings(result.Agencies[index].AudienceTags)
	}
	return result, nil
}
