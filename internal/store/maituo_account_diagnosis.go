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
	maituoSearchPlanKPI       = 30.0
	maituoFeedPlanKPI         = 70.0
	maituoAccountOverviewDays = 30
)

type diagnosisHistoryRow struct {
	ReportDate   string
	NoteID       string
	NoteURL      string
	Account      string
	CampaignName string
	Placement    string
	Spend        float64
	SearchCost   *float64
	PostbackCost *float64
}

func (p *Postgres) MaituoAccountPlanDiagnosis(ctx context.Context, spu string) (maituo.AccountPlanDiagnosis, error) {
	result := maituo.AccountPlanDiagnosis{
		SPU:              spu,
		AccountKPI:       maituoAccountDiagnosisKPI,
		PlanKPIs:         map[string]float64{"搜索": maituoSearchPlanKPI, "信息流": maituoFeedPlanKPI},
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
	accountIndexes := map[string]int{}
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
		accountIndexes[key] = len(result.Accounts)
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

	historyRows, reportDates, err := p.maituoDiagnosisPlanHistory(ctx, result.ReportDate, accountIndexes)
	if err != nil {
		return result, err
	}
	planHistory := map[string]map[string]diagnosisHistoryRow{}
	for _, row := range historyRows {
		key := diagnosisPlanKey(row)
		if planHistory[key] == nil {
			planHistory[key] = map[string]diagnosisHistoryRow{}
		}
		planHistory[key][row.ReportDate] = row
	}
	noteIDs := make([]string, 0)
	seenNoteIDs := make(map[string]struct{})
	for _, row := range historyRows {
		if row.ReportDate != result.ReportDate {
			continue
		}
		if _, exists := seenNoteIDs[row.NoteID]; exists {
			continue
		}
		seenNoteIDs[row.NoteID] = struct{}{}
		noteIDs = append(noteIDs, row.NoteID)
	}
	dandelionNotes, dandelionSyncedAt, err := p.maituoDiagnosisDandelionNotes(ctx, noteIDs)
	if err != nil {
		return result, err
	}
	result.DandelionSyncedAt = dandelionSyncedAt
	for _, row := range historyRows {
		if row.ReportDate != result.ReportDate {
			continue
		}
		accountIndex, ok := accountIndexes[diagnosisAccountKey(row.Account, row.Placement)]
		if !ok {
			continue
		}
		cost := diagnosisPlanCost(row)
		kpi := diagnosisPlanKPI(row.Placement)
		consecutive := diagnosisConsecutiveOverKPI(planHistory[diagnosisPlanKey(row)], reportDates, kpi)
		action := diagnosisPlanAction(cost, consecutive, kpi)
		plan := maituo.PlanDiagnosis{
			NoteID: row.NoteID, NoteURL: row.NoteURL, CampaignName: row.CampaignName,
			Spend: roundMaituoMoney(row.Spend), OriginalCost: cost, Cost: cost,
			CostMetric: diagnosisCostMetric(row.Placement), KPI: kpi,
			OverKPI: cost != nil && *cost >= kpi, Action: action, ConsecutiveOverKPI: consecutive,
		}
		if supplement, exists := dandelionNotes[row.NoteID]; exists {
			copy := supplement
			plan.Dandelion = &copy
			result.DandelionMatched++
		} else {
			result.DandelionMissing++
		}
		account := &result.Accounts[accountIndex]
		account.Plans = append(account.Plans, plan)
		if plan.OverKPI {
			account.OverPlans++
		}
		switch action {
		case "enlarge":
			account.EnlargePlans++
		case "stop":
			account.StopPlans++
		}
	}
	for index := range result.Accounts {
		sort.Slice(result.Accounts[index].Plans, func(left, right int) bool {
			return result.Accounts[index].Plans[left].Spend > result.Accounts[index].Plans[right].Spend
		})
	}
	sort.Slice(result.Accounts, func(left, right int) bool {
		return result.Accounts[left].Spend > result.Accounts[right].Spend
	})
	return result, nil
}

func (p *Postgres) maituoDiagnosisPlanHistory(ctx context.Context, reportDate string, accountIndexes map[string]int) ([]diagnosisHistoryRow, []string, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT notes.report_date::TEXT, notes.note_id, notes.note_url, notes.subaccount,
			notes.campaign_name, notes.placement, notes.spend::DOUBLE PRECISION,
			notes.search_cost::DOUBLE PRECISION, notes.estimated_postback_cost::DOUBLE PRECISION
		FROM maituo_customer_daily_notes notes
		WHERE notes.deleted_at IS NULL AND notes.report_date <= $1::DATE
		ORDER BY notes.report_date DESC, notes.subaccount, notes.placement, notes.campaign_name, notes.note_id
	`, reportDate)
	if err != nil {
		return nil, nil, fmt.Errorf("query Maituo plan diagnosis history: %w", err)
	}
	defer rows.Close()
	history := []diagnosisHistoryRow{}
	reportDateSet := map[string]struct{}{}
	for rows.Next() {
		var row diagnosisHistoryRow
		var searchCost, postbackCost pgtype.Float8
		if err := rows.Scan(
			&row.ReportDate, &row.NoteID, &row.NoteURL, &row.Account, &row.CampaignName,
			&row.Placement, &row.Spend, &searchCost, &postbackCost,
		); err != nil {
			return nil, nil, fmt.Errorf("scan Maituo plan diagnosis history: %w", err)
		}
		if _, ok := accountIndexes[diagnosisAccountKey(row.Account, row.Placement)]; !ok {
			continue
		}
		row.SearchCost = diagnosisNullableCost(searchCost)
		row.PostbackCost = diagnosisNullableCost(postbackCost)
		history = append(history, row)
		reportDateSet[row.ReportDate] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate Maituo plan diagnosis history: %w", err)
	}
	reportDates := make([]string, 0, len(reportDateSet))
	for date := range reportDateSet {
		reportDates = append(reportDates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(reportDates)))
	return history, reportDates, nil
}

func (p *Postgres) maituoSearchUserOverlapPoints(ctx context.Context, spu, startDate, endDate string) ([]model.SearchUserOverlapPoint, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT dates.report_date::DATE::TEXT, overlap.spu_search_users,
			overlap.subaccount_search_users, overlap.overlap_users,
			overlap.overlap_coefficient::DOUBLE PRECISION,
			overlap.deduplication_factor::DOUBLE PRECISION,
			overlap.note_search_users, overlap.note_overlap_users,
			overlap.note_overlap_coefficient::DOUBLE PRECISION,
			overlap.note_deduplication_factor::DOUBLE PRECISION
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
		var spuUsers, subaccountUsers, overlapUsers, noteUsers, noteOverlapUsers pgtype.Int8
		var coefficient, deduplicationFactor, noteCoefficient, noteDeduplicationFactor pgtype.Float8
		if err := rows.Scan(
			&point.ReportDate, &spuUsers, &subaccountUsers, &overlapUsers,
			&coefficient, &deduplicationFactor, &noteUsers, &noteOverlapUsers,
			&noteCoefficient, &noteDeduplicationFactor,
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
		if noteUsers.Valid {
			value := noteUsers.Int64
			point.NoteSearchUsers = &value
		}
		if noteOverlapUsers.Valid {
			value := noteOverlapUsers.Int64
			point.NoteOverlapUsers = &value
		}
		if noteCoefficient.Valid {
			value := noteCoefficient.Float64
			point.NoteOverlapCoefficient = &value
		}
		if noteDeduplicationFactor.Valid {
			value := noteDeduplicationFactor.Float64
			point.NoteDeduplicationFactor = &value
		}
		result = append(result, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Maituo search-user overlap history: %w", err)
	}
	rows.Close()
	if err := p.maituoSearchUserPlacementCoefficients(ctx, spu, startDate, endDate, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *Postgres) maituoSearchUserPlacementCoefficients(ctx context.Context, spu, startDate, endDate string, points []model.SearchUserOverlapPoint) error {
	pointIndexes := make(map[string]int, len(points))
	for index := range points {
		pointIndexes[points[index].ReportDate] = index
	}
	rows, err := p.pool.Query(ctx, `
			WITH account_totals AS (
				SELECT report_date, spu, placement,
					SUM(search_users)::BIGINT AS subaccount_search_users
				FROM maituo_customer_daily_subaccounts
				WHERE deleted_at IS NULL
				  AND spu = $1
				  AND report_date BETWEEN $2::DATE AND $3::DATE
				GROUP BY report_date, spu, placement
			),
			subaccount_mappings AS (
				SELECT DISTINCT report_date, spu, BTRIM(subaccount) AS subaccount, placement
				FROM maituo_customer_daily_subaccounts
				WHERE deleted_at IS NULL
				  AND spu = $1
				  AND report_date BETWEEN $2::DATE AND $3::DATE
			),
			note_rows_by_spu AS (
				SELECT DISTINCT notes.report_date, notes.note_id, notes.subaccount,
					notes.campaign_name, notes.placement, notes.search_users, mappings.spu
				FROM maituo_customer_daily_notes notes
				CROSS JOIN LATERAL regexp_split_to_table(notes.subaccount, '[、,，]+') account_names(account_name)
				JOIN subaccount_mappings mappings
				  ON mappings.report_date = notes.report_date
				 AND mappings.subaccount = BTRIM(account_names.account_name)
				 AND mappings.placement = notes.placement
				WHERE notes.deleted_at IS NULL
				  AND notes.report_date BETWEEN $2::DATE AND $3::DATE
			),
			note_totals AS (
				SELECT report_date, spu, placement,
					SUM(search_users)::BIGINT AS note_search_users
				FROM note_rows_by_spu
				GROUP BY report_date, spu, placement
			)
			SELECT accounts.report_date::DATE::TEXT, accounts.placement,
				COALESCE(notes.note_search_users, 0)::BIGINT,
				accounts.subaccount_search_users,
				COALESCE(spus.search_users, 0)::BIGINT,
				CASE WHEN accounts.subaccount_search_users > 0
					THEN COALESCE(notes.note_search_users, 0)::NUMERIC / accounts.subaccount_search_users
				END::DOUBLE PRECISION,
				CASE WHEN spus.search_users > 0
					THEN COALESCE(notes.note_search_users, 0)::NUMERIC / spus.search_users
				END::DOUBLE PRECISION
			FROM account_totals accounts
			LEFT JOIN note_totals notes
			  ON notes.report_date = accounts.report_date
			 AND notes.spu = accounts.spu
			 AND notes.placement = accounts.placement
			LEFT JOIN maituo_customer_daily_spus spus
			  ON spus.report_date = accounts.report_date
			 AND spus.spu = accounts.spu
			 AND spus.deleted_at IS NULL
			ORDER BY accounts.report_date,
				CASE accounts.placement
				WHEN '信息流' THEN 1
				WHEN '搜索' THEN 2
				WHEN '视频内流' THEN 3
				ELSE 4
			END,
			accounts.placement
	`, spu, startDate, endDate)
	if err != nil {
		return fmt.Errorf("query Maituo search-user placement coefficients: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var reportDate, placement string
		var noteSearchUsers, subaccountSearchUsers, spuSearchUsers int64
		var coefficient, noteSPUCoefficient pgtype.Float8
		if err := rows.Scan(
			&reportDate, &placement, &noteSearchUsers, &subaccountSearchUsers, &spuSearchUsers,
			&coefficient, &noteSPUCoefficient,
		); err != nil {
			return fmt.Errorf("scan Maituo search-user placement coefficient: %w", err)
		}
		pointIndex, ok := pointIndexes[reportDate]
		if !ok {
			continue
		}
		item := model.SearchUserPlacementCoefficient{
			Placement: placement, SearchUsers: subaccountSearchUsers,
			NoteSearchUsers: noteSearchUsers, SubaccountSearchUsers: subaccountSearchUsers, SPUSearchUsers: spuSearchUsers,
		}
		if coefficient.Valid {
			value := coefficient.Float64
			item.Coefficient = &value
		}
		if noteSPUCoefficient.Valid {
			value := noteSPUCoefficient.Float64
			item.NoteSPUCoefficient = &value
		}
		points[pointIndex].PlacementCoefficients = append(points[pointIndex].PlacementCoefficients, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate Maituo search-user placement coefficients: %w", err)
	}
	return nil
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

func diagnosisPlanKey(row diagnosisHistoryRow) string {
	return row.NoteID + "\x00" + row.Account + "\x00" + row.CampaignName + "\x00" + row.Placement
}

func diagnosisPlanCost(row diagnosisHistoryRow) *float64 {
	if row.Placement == "信息流" {
		return row.PostbackCost
	}
	return row.SearchCost
}

func diagnosisCostMetric(placement string) string {
	if placement == "信息流" {
		return "预计回流后成本"
	}
	return "回搜成本"
}

func diagnosisPlanKPI(placement string) float64 {
	if placement == "信息流" {
		return maituoFeedPlanKPI
	}
	return maituoSearchPlanKPI
}

func diagnosisConsecutiveOverKPI(history map[string]diagnosisHistoryRow, reportDates []string, kpi float64) int {
	consecutive := 0
	for _, date := range reportDates {
		row, ok := history[date]
		if !ok {
			break
		}
		cost := diagnosisPlanCost(row)
		if cost == nil || *cost < kpi {
			break
		}
		consecutive++
	}
	return consecutive
}

func diagnosisPlanAction(cost *float64, consecutive int, kpi float64) string {
	if cost == nil {
		return "inactive"
	}
	if *cost < kpi {
		return "enlarge"
	}
	if consecutive >= 3 {
		return "stop"
	}
	return "observe"
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
