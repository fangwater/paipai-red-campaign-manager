package store

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"

	"paipai-red-campaign-manager/internal/model"
)

const (
	contentAnalysisUnlabeled = "未标注"
	contentAnalysisBoomCost  = 20.0
	contentAnalysisROI       = 1.2
)

var contentAnalysisTypeOrder = []string{"多品横测", "直给推荐", "科普", "询问向", "经验分享", "实测打卡", "好物分享"}

func (p *Postgres) ContentAnalysis(ctx context.Context, query model.ContentAnalysisQuery) (model.ContentAnalysis, error) {
	if query.SPU == "" {
		query.SPU = "辅酶"
	}
	if query.Agency == "" {
		query.Agency = "全部"
	}
	if query.Dimension == "" {
		query.Dimension = "audience"
	}
	result := model.ContentAnalysis{
		SPU: query.SPU, Agency: query.Agency, Dimension: query.Dimension,
		PublishedStartDate: query.PublishedStartDate, PublishedEndDate: query.PublishedEndDate,
		Types: []string{}, Dimensions: []string{}, Cells: []model.ContentAnalysisCell{},
	}
	if err := p.loadContentAnalysisSources(ctx, query, &result.Sources); err != nil {
		return result, err
	}
	notes, err := p.loadContentAnalysisNotes(ctx, query)
	if err != nil {
		return result, err
	}
	buildContentAnalysis(&result, notes)
	return result, nil
}

func (p *Postgres) loadContentAnalysisSources(ctx context.Context, query model.ContentAnalysisQuery, sources *model.ContentAnalysisSources) error {
	err := p.pool.QueryRow(ctx, `
		WITH dandelion AS (
			SELECT MAX(CASE WHEN jsonb_typeof(records.fields -> '数据更新日期')='number'
				THEN (records.fields ->> '数据更新日期')::BIGINT END) AS data_updated_ms,
				MAX(records.synced_at) AS synced_at
			FROM lark_bitable_records records
			JOIN lark_bitable_tables tables USING (app_token, table_id)
			WHERE records.deleted_at IS NULL AND tables.deleted_at IS NULL AND tables.name='蒲公英数据'
			  AND records.fields ->> 'spu名称' ILIKE $1
			  AND CASE records.fields ->> '下单账号'
				WHEN '杭州智元文化传播有限公司' THEN '智元'
				WHEN '江苏拾光宝盒信息技术有限公司' THEN '曼杰'
				WHEN '上海有一有二网络技术有限公司' THEN '有一有二'
			  END IS NOT NULL
			  AND ($2='全部' OR CASE records.fields ->> '下单账号'
				WHEN '杭州智元文化传播有限公司' THEN '智元'
				WHEN '江苏拾光宝盒信息技术有限公司' THEN '曼杰'
				WHEN '上海有一有二网络技术有限公司' THEN '有一有二'
			  END=$2)
		), guorai AS (
			SELECT snapshot_date, window_start, window_end
			FROM guorai_fetch_runs
			WHERE entity_type='note' AND status='succeeded'
			ORDER BY snapshot_date DESC, finished_at DESC NULLS LAST, id DESC
			LIMIT 1
		)
		SELECT
			COALESCE(TO_CHAR(TO_TIMESTAMP(dandelion.data_updated_ms / 1000.0) AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD'), ''),
			COALESCE(dandelion.synced_at::TEXT, ''),
			COALESCE((SELECT MAX(report_date)::TEXT FROM maituo_customer_daily_notes WHERE deleted_at IS NULL), ''),
			COALESCE(guorai.snapshot_date::TEXT, ''), COALESCE(guorai.window_start::TEXT, ''), COALESCE(guorai.window_end::TEXT, ''),
			COALESCE((SELECT MAX(last_synced_at)::TEXT FROM service_provider_content_tables
				WHERE provider_code IN ('manjie','youyiyouer','zhiyuan') AND enabled), '')
		FROM dandelion LEFT JOIN guorai ON TRUE
	`, "%"+query.SPU+"%", query.Agency).Scan(
		&sources.DandelionDataDate, &sources.DandelionSyncedAt, &sources.MaituoReportDate,
		&sources.GuoraiSnapshotDate, &sources.GuoraiWindowStart, &sources.GuoraiWindowEnd,
		&sources.ManuscriptSyncedAt,
	)
	if err != nil {
		return fmt.Errorf("query content analysis source dates: %w", err)
	}
	return nil
}

func (p *Postgres) loadContentAnalysisNotes(ctx context.Context, query model.ContentAnalysisQuery) ([]model.ContentAnalysisNote, error) {
	rows, err := p.pool.Query(ctx, `
		WITH pgy_ranked AS (
			SELECT
				COALESCE(records.fields #>> '{笔记ID,0,text}', records.fields ->> '笔记ID', '') AS note_id,
				COALESCE(records.fields #>> '{笔记标题,0,text}', records.fields ->> '笔记标题', '') AS title,
				COALESCE(records.fields #>> '{笔记链接,0,link}', records.fields ->> '笔记链接', '') AS note_url,
				COALESCE(records.fields #>> '{达人/发布账号,0,text}', records.fields ->> '达人/发布账号', '') AS author,
				CASE WHEN jsonb_typeof(records.fields -> '发布时间')='number'
					THEN (TO_TIMESTAMP((records.fields ->> '发布时间')::DOUBLE PRECISION / 1000) AT TIME ZONE 'Asia/Shanghai')::DATE
				END AS published_date,
				CASE records.fields ->> '下单账号'
					WHEN '杭州智元文化传播有限公司' THEN '智元'
					WHEN '江苏拾光宝盒信息技术有限公司' THEN '曼杰'
					WHEN '上海有一有二网络技术有限公司' THEN '有一有二'
				END AS agency,
				CASE records.fields ->> '下单账号'
					WHEN '杭州智元文化传播有限公司' THEN 'zhiyuan'
					WHEN '江苏拾光宝盒信息技术有限公司' THEN 'manjie'
					WHEN '上海有一有二网络技术有限公司' THEN 'youyiyouer'
				END AS provider_code,
				CASE
					WHEN jsonb_typeof(records.fields -> '站外活跃成本（15天设备归因）')='number'
					THEN (records.fields ->> '站外活跃成本（15天设备归因）')::DOUBLE PRECISION
				END AS dandelion_cost,
				ROW_NUMBER() OVER (
					PARTITION BY COALESCE(records.fields #>> '{笔记ID,0,text}', records.fields ->> '笔记ID', '')
					ORDER BY CASE WHEN jsonb_typeof(records.fields -> '数据更新日期')='number'
						THEN (records.fields ->> '数据更新日期')::BIGINT END DESC NULLS LAST,
						records.synced_at DESC, records.record_id DESC
				) AS row_number
			FROM lark_bitable_records records
			JOIN lark_bitable_tables tables USING (app_token, table_id)
			WHERE records.deleted_at IS NULL AND tables.deleted_at IS NULL AND tables.name='蒲公英数据'
			  AND records.fields ->> 'spu名称' ILIKE $1
		), pgy AS (
			SELECT * FROM pgy_ranked
			WHERE row_number=1 AND provider_code IS NOT NULL AND note_id ~ '^[0-9a-fA-F]{24}$'
			  AND ($2='全部' OR agency=$2)
			  AND (NULLIF($3::TEXT, '') IS NULL OR published_date >= NULLIF($3::TEXT, '')::DATE)
			  AND (NULLIF($4::TEXT, '') IS NULL OR published_date <= NULLIF($4::TEXT, '')::DATE)
		), maituo_placements AS (
			SELECT LOWER(BTRIM(note_id)) AS note_id, placement,
				SUM(spend)::DOUBLE PRECISION AS spend,
				CASE WHEN placement='搜索'
					THEN (SUM(spend)/NULLIF(SUM(search_users),0))::DOUBLE PRECISION
					ELSE (SUM(spend)/NULLIF(SUM(CASE WHEN estimated_postback_cost>0
						THEN spend/estimated_postback_cost END),0))::DOUBLE PRECISION
				END AS cost
			FROM maituo_customer_daily_notes
			WHERE deleted_at IS NULL AND placement IN ('搜索','信息流')
			GROUP BY LOWER(BTRIM(note_id)), placement
		), maituo AS (
			SELECT note_id,
				COALESCE(MAX(spend) FILTER (WHERE placement='搜索'),0) AS search_spend,
				MAX(cost) FILTER (WHERE placement='搜索') AS search_cost,
				COALESCE(MAX(spend) FILTER (WHERE placement='信息流'),0) AS feed_spend,
				MAX(cost) FILTER (WHERE placement='信息流') AS feed_cost
			FROM maituo_placements GROUP BY note_id
		), latest_guorai AS (
			SELECT id FROM guorai_fetch_runs
			WHERE entity_type='note' AND status='succeeded'
			ORDER BY snapshot_date DESC, finished_at DESC NULLS LAST, id DESC
			LIMIT 1
		), guorai AS (
			SELECT LOWER(BTRIM(snapshots.note_id)) AS note_id, snapshots.total_roi::DOUBLE PRECISION AS roi
			FROM guorai_note_snapshots snapshots
			JOIN latest_guorai ON latest_guorai.id=snapshots.fetch_id
			JOIN guorai_notes notes USING (note_id)
			WHERE COALESCE(BTRIM(notes.note_author_name),'') NOT ILIKE 'MegaRed脉拓'
			  AND notes.spu_name ILIKE $1
		)
		SELECT pgy.note_id, pgy.title, pgy.note_url, pgy.author, COALESCE(pgy.published_date::TEXT, ''), pgy.agency,
			COALESCE(execution.note_type,''), COALESCE(execution.audience,''), COALESCE(execution.user_scenario,''),
			pgy.dandelion_cost,
			COALESCE(maituo.search_spend,0), maituo.search_cost,
			COALESCE(maituo.feed_spend,0), maituo.feed_cost,
			guorai.roi
		FROM pgy
		LEFT JOIN LATERAL (
			SELECT note_type, audience, user_scenario
			FROM service_provider_note_executions
			WHERE deleted_at IS NULL AND provider_code=pgy.provider_code AND LOWER(BTRIM(note_id))=LOWER(BTRIM(pgy.note_id))
			ORDER BY synced_at DESC, source_row_number DESC
			LIMIT 1
		) execution ON TRUE
		LEFT JOIN maituo ON maituo.note_id=LOWER(BTRIM(pgy.note_id))
		LEFT JOIN guorai ON guorai.note_id=LOWER(BTRIM(pgy.note_id))
		ORDER BY pgy.agency, pgy.note_id
	`, "%"+query.SPU+"%", query.Agency, query.PublishedStartDate, query.PublishedEndDate)
	if err != nil {
		return nil, fmt.Errorf("query content analysis notes: %w", err)
	}
	defer rows.Close()
	notes := []model.ContentAnalysisNote{}
	for rows.Next() {
		var note model.ContentAnalysisNote
		if err := rows.Scan(
			&note.NoteID, &note.Title, &note.URL, &note.Author, &note.PublishedDate, &note.Agency,
			&note.ContentType, &note.Audience, &note.Scenario, &note.DandelionCost,
			&note.SearchSpend, &note.SearchCost, &note.FeedSpend, &note.FeedCost, &note.ROI,
		); err != nil {
			return nil, fmt.Errorf("scan content analysis note: %w", err)
		}
		if note.Title == "" {
			note.Title = note.NoteID
		}
		if note.URL == "" {
			note.URL = "https://www.xiaohongshu.com/explore/" + url.PathEscape(note.NoteID)
		}
		note.ContentType = normalizeContentAnalysisLabel("type", note.ContentType)
		note.Audience = normalizeContentAnalysisLabel("audience", note.Audience)
		note.Scenario = normalizeContentAnalysisLabel("scenario", note.Scenario)
		note.Boom = note.DandelionCost != nil && *note.DandelionCost > 0 && *note.DandelionCost <= contentAnalysisBoomCost
		note.SearchQualified = note.SearchSpend >= 200 && note.SearchCost != nil && *note.SearchCost <= 30
		note.FeedQualified = note.FeedSpend >= 200 && note.FeedCost != nil && *note.FeedCost <= 70
		note.FlowEvaluated = (note.SearchSpend >= 200 && note.SearchCost != nil) || (note.FeedSpend >= 200 && note.FeedCost != nil)
		note.FlowQualified = note.SearchQualified || note.FeedQualified
		note.ROIQualified = note.ROI != nil && *note.ROI >= contentAnalysisROI
		note.AllQualified = note.Boom && note.FlowQualified && note.ROIQualified
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content analysis notes: %w", err)
	}
	return notes, nil
}

func buildContentAnalysis(result *model.ContentAnalysis, notes []model.ContentAnalysisNote) {
	typeCounts := map[string]int{}
	dimensionCounts := map[string]int{}
	cells := map[string]*model.ContentAnalysisCell{}
	for _, note := range notes {
		result.Coverage.TotalNotes++
		if note.ContentType != contentAnalysisUnlabeled {
			result.Coverage.ContentTypeTagged++
		}
		if note.Audience != contentAnalysisUnlabeled {
			result.Coverage.AudienceTagged++
		}
		if note.Scenario != contentAnalysisUnlabeled {
			result.Coverage.ScenarioTagged++
		}
		if note.DandelionCost != nil && *note.DandelionCost > 0 {
			result.Coverage.DandelionCostNotes++
		}
		if note.FlowEvaluated {
			result.Coverage.FlowEvaluatedNotes++
		}
		if note.ROI != nil {
			result.Coverage.ROIEvaluatedNotes++
		}
		if note.DandelionCost != nil && *note.DandelionCost > 0 && note.FlowEvaluated && note.ROI != nil {
			result.Coverage.AllMetricsNotes++
		}
		dimension := note.Audience
		if result.Dimension == "scenario" {
			dimension = note.Scenario
		}
		typeCounts[note.ContentType]++
		dimensionCounts[dimension]++
		key := note.ContentType + "\x00" + dimension
		cell := cells[key]
		if cell == nil {
			cell = &model.ContentAnalysisCell{
				ContentType: note.ContentType, Dimension: dimension, Notes: []model.ContentAnalysisNote{},
			}
			cells[key] = cell
		}
		cell.TotalNotes++
		cell.Notes = append(cell.Notes, note)
		if note.DandelionCost != nil && *note.DandelionCost > 0 {
			cell.DandelionEligible++
			if note.Boom {
				cell.BoomCount++
			}
		}
		if note.FlowEvaluated {
			cell.FlowEvaluated++
		}
		if note.FlowQualified {
			cell.FlowQualified++
		}
		if note.ROI != nil {
			cell.ROIEvaluated++
		}
		if note.ROIQualified {
			cell.ROIQualified++
		}
		if note.AllQualified {
			cell.AllQualified++
		}
	}
	for contentType := range typeCounts {
		result.Types = append(result.Types, contentType)
	}
	sort.Slice(result.Types, func(i, j int) bool {
		left, right := contentTypeRank(result.Types[i]), contentTypeRank(result.Types[j])
		if left != right {
			return left < right
		}
		return typeCounts[result.Types[i]] > typeCounts[result.Types[j]]
	})
	for dimension := range dimensionCounts {
		result.Dimensions = append(result.Dimensions, dimension)
	}
	sort.Slice(result.Dimensions, func(i, j int) bool {
		if result.Dimensions[i] == contentAnalysisUnlabeled {
			return false
		}
		if result.Dimensions[j] == contentAnalysisUnlabeled {
			return true
		}
		if dimensionCounts[result.Dimensions[i]] != dimensionCounts[result.Dimensions[j]] {
			return dimensionCounts[result.Dimensions[i]] > dimensionCounts[result.Dimensions[j]]
		}
		return strings.Compare(result.Dimensions[i], result.Dimensions[j]) < 0
	})
	typeIndexes := make(map[string]int, len(result.Types))
	for index, value := range result.Types {
		typeIndexes[value] = index
	}
	dimensionIndexes := make(map[string]int, len(result.Dimensions))
	for index, value := range result.Dimensions {
		dimensionIndexes[value] = index
	}
	for _, cell := range cells {
		if cell.DandelionEligible > 0 {
			rate := math.Round(float64(cell.BoomCount)/float64(cell.DandelionEligible)*10000) / 10000
			cell.BoomRate = &rate
		}
		sort.Slice(cell.Notes, func(i, j int) bool {
			left, right := cell.Notes[i], cell.Notes[j]
			if left.AllQualified != right.AllQualified {
				return left.AllQualified
			}
			if left.Boom != right.Boom {
				return left.Boom
			}
			if left.DandelionCost != nil && right.DandelionCost != nil && *left.DandelionCost != *right.DandelionCost {
				return *left.DandelionCost < *right.DandelionCost
			}
			return left.NoteID < right.NoteID
		})
		result.Cells = append(result.Cells, *cell)
	}
	sort.Slice(result.Cells, func(i, j int) bool {
		left, right := result.Cells[i], result.Cells[j]
		if typeIndexes[left.ContentType] != typeIndexes[right.ContentType] {
			return typeIndexes[left.ContentType] < typeIndexes[right.ContentType]
		}
		return dimensionIndexes[left.Dimension] < dimensionIndexes[right.Dimension]
	})
}

func normalizeContentAnalysisLabel(kind, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return contentAnalysisUnlabeled
	}
	switch kind + ":" + value {
	case "audience:中老年人":
		return "中老年"
	case "audience:考公考研人":
		return "考公考研"
	case "audience:备孕女性":
		return "备孕女生"
	case "audience:上班族":
		return "职场人"
	case "audience:运动人群":
		return "健身人"
	case "scenario:还原vs氧化":
		return "还原VS氧化"
	case "scenario:健身":
		return "运动恢复"
	default:
		return value
	}
}

func contentTypeRank(value string) int {
	for index, candidate := range contentAnalysisTypeOrder {
		if value == candidate {
			return index
		}
	}
	if value == contentAnalysisUnlabeled {
		return len(contentAnalysisTypeOrder) + 1
	}
	return len(contentAnalysisTypeOrder)
}
