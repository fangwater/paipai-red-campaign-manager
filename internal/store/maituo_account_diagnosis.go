package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
	"paipai-red-campaign-manager/internal/model"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	maituoAccountDiagnosisKPI = 70.0
	maituoAccountOverviewDays = 30
)

func (p *Postgres) MaituoAccountPlanDiagnosis(ctx context.Context, spu string) (maituo.AccountPlanDiagnosis, error) {
	// Account aggregates remain valid; plan diagnosis was retired with note-to-account attribution.
	result := maituo.AccountPlanDiagnosis{
		SPU:              spu,
		AccountKPI:       maituoAccountDiagnosisKPI,
		PlanKPIs:         map[string]float64{},
		AccountOverviews: []maituo.AccountOverview{},
		Accounts:         []maituo.AccountDiagnosis{},
	}
	if err := p.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(report_date)::TEXT, '')
		FROM maituo_customer_daily_subaccounts
		WHERE deleted_at IS NULL AND spu = $1
	`, spu).Scan(&result.ReportDate); err != nil {
		return result, fmt.Errorf("query Maituo account diagnosis date: %w", err)
	}
	if result.ReportDate == "" {
		return result, nil
	}

	latestDate, err := time.Parse(time.DateOnly, result.ReportDate)
	if err != nil {
		return result, fmt.Errorf("parse Maituo account diagnosis date: %w", err)
	}
	windowStart := latestDate.AddDate(0, 0, -(maituoAccountOverviewDays - 1)).Format(time.DateOnly)
	accountRows, err := p.pool.Query(ctx, `
		SELECT accounts.report_date::TEXT, accounts.subaccount, accounts.placement,
			accounts.spend::DOUBLE PRECISION, accounts.search_users,
			CASE WHEN accounts.placement = '信息流'
				THEN accounts.estimated_postback_cost::DOUBLE PRECISION
				ELSE accounts.search_cost::DOUBLE PRECISION
			END AS diagnosis_cost,
			accounts.search_rate_pct::DOUBLE PRECISION, accounts.cpc::DOUBLE PRECISION,
			accounts.ctr_pct::DOUBLE PRECISION, accounts.note_count
		FROM maituo_customer_daily_subaccounts accounts
		WHERE accounts.deleted_at IS NULL AND accounts.spu = $1
		  AND accounts.report_date BETWEEN $2::DATE AND $3::DATE
		ORDER BY accounts.report_date, accounts.subaccount, accounts.placement
	`, spu, windowStart, result.ReportDate)
	if err != nil {
		return result, fmt.Errorf("query Maituo account diagnosis history: %w", err)
	}
	defer accountRows.Close()

	type accountSnapshot struct {
		Spend         float64
		SearchUsers   int64
		Cost          *float64
		SearchRatePct *float64
		CPC           *float64
		CTRPct        *float64
		NoteCount     int64
	}
	history := map[string]map[string]accountSnapshot{}
	for accountRows.Next() {
		var reportDate, account, placement string
		var spend float64
		var searchUsers, noteCount int64
		var nullableCost, searchRatePct, cpc, ctrPct pgtype.Float8
		if err := accountRows.Scan(
			&reportDate, &account, &placement, &spend, &searchUsers, &nullableCost,
			&searchRatePct, &cpc, &ctrPct, &noteCount,
		); err != nil {
			return result, fmt.Errorf("scan Maituo account diagnosis history: %w", err)
		}
		snapshot := accountSnapshot{
			Spend: spend, SearchUsers: searchUsers, Cost: diagnosisNullableCost(nullableCost),
			SearchRatePct: diagnosisNullableCost(searchRatePct), CPC: diagnosisNullableCost(cpc),
			CTRPct: diagnosisNullableCost(ctrPct), NoteCount: noteCount,
		}
		key := diagnosisAccountKey(account, placement)
		if history[key] == nil {
			history[key] = map[string]accountSnapshot{}
		}
		history[key][reportDate] = snapshot
	}
	if err := accountRows.Err(); err != nil {
		return result, fmt.Errorf("iterate Maituo account diagnosis history: %w", err)
	}
	accountRows.Close()

	previousDate := latestDate.AddDate(0, 0, -1).Format(time.DateOnly)
	for key, snapshots := range history {
		current, ok := snapshots[result.ReportDate]
		if !ok {
			continue
		}
		account, placement := splitDiagnosisAccountKey(key)
		item := maituo.AccountDiagnosis{
			Account:       account,
			Placement:     placement,
			Spend:         roundMaituoMoney(current.Spend),
			SearchUsers:   current.SearchUsers,
			OriginalCost:  current.Cost,
			Cost:          current.Cost,
			SearchRatePct: current.SearchRatePct,
			CPC:           current.CPC,
			CTRPct:        current.CTRPct,
			NoteCount:     current.NoteCount,
			CostMetric:    diagnosisCostMetric(placement),
			KPI:           maituoAccountDiagnosisKPI,
			Status:        diagnosisAccountStatus(current.Cost),
			Points:        []maituo.AccountDiagnosisPoint{},
			Plans:         []maituo.PlanDiagnosis{},
		}
		if previous, exists := snapshots[previousDate]; exists {
			item.PreviousCost = previous.Cost
			if current.Cost != nil && previous.Cost != nil && *previous.Cost != 0 {
				change := (*current.Cost - *previous.Cost) / *previous.Cost
				item.ChangePct = &change
			}
		}
		for offset := -6; offset <= 0; offset++ {
			date := latestDate.AddDate(0, 0, offset).Format(time.DateOnly)
			point := maituo.AccountDiagnosisPoint{ReportDate: date}
			if snapshot, exists := snapshots[date]; exists {
				spend := roundMaituoMoney(snapshot.Spend)
				searchUsers := snapshot.SearchUsers
				noteCount := snapshot.NoteCount
				point.Spend = &spend
				point.SearchUsers = &searchUsers
				point.OriginalCost = snapshot.Cost
				point.Cost = snapshot.Cost
				point.SearchRatePct = snapshot.SearchRatePct
				point.CPC = snapshot.CPC
				point.CTRPct = snapshot.CTRPct
				point.NoteCount = &noteCount
			}
			item.Points = append(item.Points, point)
		}
		result.Accounts = append(result.Accounts, item)
	}

	overviewAccounts := make(map[string]struct{})
	for _, account := range result.Accounts {
		overviewAccounts[account.Account] = struct{}{}
	}
	for account := range overviewAccounts {
		overview := maituo.AccountOverview{Account: account, Points: []maituo.AccountOverviewPoint{}}
		searchSnapshots := history[diagnosisAccountKey(account, "搜索")]
		feedSnapshots := history[diagnosisAccountKey(account, "信息流")]
		for offset := -(maituoAccountOverviewDays - 1); offset <= 0; offset++ {
			date := latestDate.AddDate(0, 0, offset).Format(time.DateOnly)
			point := maituo.AccountOverviewPoint{ReportDate: date}
			if snapshot, exists := searchSnapshots[date]; exists {
				spend := roundMaituoMoney(snapshot.Spend)
				point.SearchSpend = &spend
				point.SearchCost = snapshot.Cost
				point.SearchCPC = snapshot.CPC
				point.SearchCTRPct = snapshot.CTRPct
				point.SearchRatePct = snapshot.SearchRatePct
			}
			if snapshot, exists := feedSnapshots[date]; exists {
				spend := roundMaituoMoney(snapshot.Spend)
				point.FeedSpend = &spend
				point.FeedCost = snapshot.Cost
				point.FeedCPC = snapshot.CPC
				point.FeedCTRPct = snapshot.CTRPct
				point.FeedSearchRatePct = snapshot.SearchRatePct
			}
			totalSpend := 0.0
			hasSpend := false
			if point.SearchSpend != nil {
				totalSpend += *point.SearchSpend
				hasSpend = true
			}
			if point.FeedSpend != nil {
				totalSpend += *point.FeedSpend
				hasSpend = true
			}
			if hasSpend {
				totalSpend = roundMaituoMoney(totalSpend)
				point.TotalSpend = &totalSpend
				if offset == 0 {
					overview.CurrentTotalSpend = totalSpend
				}
			}
			overview.Points = append(overview.Points, point)
		}
		result.AccountOverviews = append(result.AccountOverviews, overview)
	}
	sort.Slice(result.AccountOverviews, func(left, right int) bool {
		if result.AccountOverviews[left].CurrentTotalSpend == result.AccountOverviews[right].CurrentTotalSpend {
			return result.AccountOverviews[left].Account < result.AccountOverviews[right].Account
		}
		return result.AccountOverviews[left].CurrentTotalSpend > result.AccountOverviews[right].CurrentTotalSpend
	})

	sort.Slice(result.Accounts, func(left, right int) bool {
		return result.Accounts[left].Spend > result.Accounts[right].Spend
	})
	return result, nil
}

func (p *Postgres) maituoSearchUserOverlapPoints(ctx context.Context, spu, startDate, endDate string) ([]model.SearchUserOverlapPoint, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT dates.report_date::DATE::TEXT, overlap.spu_search_users,
			overlap.subaccount_search_users, overlap.overlap_users,
			overlap.overlap_coefficient::DOUBLE PRECISION,
			overlap.deduplication_factor::DOUBLE PRECISION
		FROM generate_series($2::DATE, $3::DATE, INTERVAL '1 day') dates(report_date)
		LEFT JOIN maituo_customer_daily_search_user_overlap overlap
		  ON overlap.report_date = dates.report_date::DATE
		 AND overlap.spu = $1
		ORDER BY dates.report_date
	`, spu, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("query Maituo search-user overlap history: %w", err)
	}
	defer rows.Close()

	result := make([]model.SearchUserOverlapPoint, 0, maituoAccountOverviewDays)
	for rows.Next() {
		point := model.SearchUserOverlapPoint{PlacementCoefficients: []model.SearchUserPlacementCoefficient{}}
		var spuUsers, subaccountUsers, overlapUsers pgtype.Int8
		var coefficient, deduplicationFactor pgtype.Float8
		if err := rows.Scan(
			&point.ReportDate, &spuUsers, &subaccountUsers, &overlapUsers,
			&coefficient, &deduplicationFactor,
		); err != nil {
			return nil, fmt.Errorf("scan Maituo search-user overlap history: %w", err)
		}
		if spuUsers.Valid {
			value := spuUsers.Int64
			point.SPUSearchUsers = &value
		}
		if subaccountUsers.Valid {
			value := subaccountUsers.Int64
			point.SubaccountSearchUsers = &value
		}
		if overlapUsers.Valid {
			value := overlapUsers.Int64
			point.OverlapUsers = &value
		}
		if coefficient.Valid {
			value := coefficient.Float64
			point.OverlapCoefficient = &value
		}
		if deduplicationFactor.Valid {
			value := deduplicationFactor.Float64
			point.DeduplicationFactor = &value
		}
		result = append(result, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Maituo search-user overlap history: %w", err)
	}
	return result, nil
}

func (p *Postgres) maituoDiagnosisDandelionNotes(ctx context.Context, noteIDs []string) (map[string]maituo.DandelionNoteSupplement, string, error) {
	result := make(map[string]maituo.DandelionNoteSupplement, len(noteIDs))
	if len(noteIDs) == 0 {
		return result, "", nil
	}
	rows, err := p.pool.Query(ctx, `
		WITH base AS (
			SELECT records.record_id,
				records.fields #>> '{笔记ID,0,text}' AS note_id,
				COALESCE(records.fields #>> '{笔记标题,0,text}', '') AS title,
				COALESCE(records.fields #>> '{达人/发布账号,0,text}', '') AS author,
				COALESCE(records.fields ->> '笔记类型', '') AS note_type,
				COALESCE(records.fields ->> '内容标签', '') AS content_tag,
				CASE WHEN jsonb_typeof(records.fields -> '发布时间') = 'number'
					THEN TO_CHAR(TO_TIMESTAMP((records.fields ->> '发布时间')::DOUBLE PRECISION / 1000) AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')
					ELSE '' END AS published_date,
				CASE WHEN jsonb_typeof(records.fields -> '数据更新日期') = 'number'
					THEN TO_CHAR(TO_TIMESTAMP((records.fields ->> '数据更新日期')::DOUBLE PRECISION / 1000) AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')
					ELSE '' END AS data_updated_date,
				CASE WHEN jsonb_typeof(records.fields -> '数据更新日期') = 'number'
					THEN (records.fields ->> '数据更新日期')::BIGINT END AS data_updated_epoch,
				CASE WHEN jsonb_typeof(records.fields -> '蒲公英金额') = 'number' THEN (records.fields ->> '蒲公英金额')::DOUBLE PRECISION ELSE 0 END AS dandelion_amount,
				CASE WHEN jsonb_typeof(records.fields -> '曝光量') = 'number' THEN (records.fields ->> '曝光量')::BIGINT ELSE 0 END AS impressions,
				CASE WHEN jsonb_typeof(records.fields -> '阅读量') = 'number' THEN (records.fields ->> '阅读量')::BIGINT ELSE 0 END AS reads,
				CASE WHEN jsonb_typeof(records.fields -> '互动量') = 'number' THEN (records.fields ->> '互动量')::BIGINT ELSE 0 END AS interactions,
				CASE WHEN jsonb_typeof(records.fields -> '阅读单价') = 'number' THEN (records.fields ->> '阅读单价')::DOUBLE PRECISION ELSE 0 END AS read_cost,
				CASE WHEN jsonb_typeof(records.fields -> '互动单价') = 'number' THEN (records.fields ->> '互动单价')::DOUBLE PRECISION ELSE 0 END AS interaction_cost,
				records.lark_updated_at, records.synced_at,
				MAX(records.synced_at) OVER () AS source_synced_at
			FROM lark_bitable_records records
			JOIN lark_bitable_tables tables
			  ON tables.app_token = records.app_token
			 AND tables.table_id = records.table_id
			WHERE tables.name = '蒲公英数据'
			  AND tables.deleted_at IS NULL
			  AND records.deleted_at IS NULL
			  AND records.fields #>> '{笔记ID,0,text}' = ANY($1::text[])
		), ranked AS (
			SELECT base.*,
				ROW_NUMBER() OVER (
					PARTITION BY note_id
					ORDER BY data_updated_epoch DESC NULLS LAST, lark_updated_at DESC NULLS LAST, record_id DESC
				) AS row_rank
			FROM base
		)
		SELECT note_id, title, author, note_type, content_tag, published_date, data_updated_date,
			dandelion_amount, impressions, reads, interactions, read_cost, interaction_cost, source_synced_at
		FROM ranked
		WHERE row_rank = 1
		ORDER BY note_id
	`, noteIDs)
	if err != nil {
		return nil, "", fmt.Errorf("query Maituo diagnosis Dandelion supplements: %w", err)
	}
	defer rows.Close()

	syncedAt := ""
	for rows.Next() {
		var noteID string
		var item maituo.DandelionNoteSupplement
		var sourceSyncedAt time.Time
		if err := rows.Scan(
			&noteID, &item.Title, &item.Author, &item.NoteType, &item.ContentTag,
			&item.PublishedDate, &item.DataUpdatedDate, &item.DandelionAmount,
			&item.Impressions, &item.Reads, &item.Interactions, &item.ReadCost,
			&item.InteractionCost, &sourceSyncedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan Maituo diagnosis Dandelion supplement: %w", err)
		}
		item.Title = strings.Join(strings.Fields(item.Title), " ")
		item.Author = strings.Join(strings.Fields(item.Author), " ")
		result[noteID] = item
		syncedAt = sourceSyncedAt.Format(time.RFC3339)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate Maituo diagnosis Dandelion supplements: %w", err)
	}
	return result, syncedAt, nil
}

func diagnosisNullableCost(value pgtype.Float8) *float64 {
	if !value.Valid {
		return nil
	}
	rounded := roundMaituoMoney(value.Float64)
	return &rounded
}

func diagnosisAccountKey(account, placement string) string {
	return account + "\x00" + placement
}

func splitDiagnosisAccountKey(key string) (string, string) {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func diagnosisCostMetric(placement string) string {
	if placement == "信息流" {
		return "预计回流后成本"
	}
	return "回搜成本"
}

func diagnosisAccountStatus(cost *float64) string {
	if cost == nil {
		return "unattributed"
	}
	if *cost < maituoAccountDiagnosisKPI {
		return "good"
	}
	return "over"
}
