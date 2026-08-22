package store

import (
	"context"
	"fmt"
	"strings"

	"paipai-red-campaign-manager/internal/maituo"
)

func (p *Postgres) MaituoXHSLinks(ctx context.Context, query maituo.XHSLinkQuery) (maituo.XHSLinkResult, error) {
	result := maituo.XHSLinkResult{Page: query.Page, PageSize: query.PageSize, Items: []maituo.XHSLinkItem{}}
	if err := p.pool.QueryRow(ctx, `SELECT COALESCE(MAX(report_date)::TEXT, '') FROM maituo_customer_daily_notes WHERE deleted_at IS NULL`).Scan(&result.ReportDate); err != nil {
		return result, fmt.Errorf("query latest Maituo report date: %w", err)
	}
	if result.ReportDate == "" {
		return result, nil
	}

	searchPattern := "%" + strings.TrimSpace(query.Search) + "%"
	offset := (query.Page - 1) * query.PageSize
	rows, err := p.pool.Query(ctx, `
		WITH daily AS (
			SELECT note_id, placement,
				SUM(spend)::DOUBLE PRECISION AS spend,
				SUM(search_users)::BIGINT AS search_users,
				SUM(COALESCE(search_cost, 0))::DOUBLE PRECISION AS search_cost
			FROM maituo_customer_daily_notes
			WHERE report_date=$1::DATE AND deleted_at IS NULL
			GROUP BY note_id, placement
		), filtered AS (
			SELECT daily.*
			FROM daily
			WHERE EXISTS (
				SELECT 1
				FROM xhs_jg_creativities creativity
				JOIN xhs_jg_campaigns campaign
				  ON campaign.advertiser_id=creativity.advertiser_id
				 AND campaign.campaign_id=creativity.campaign_id
				 AND campaign.deleted_at IS NULL
				WHERE creativity.deleted_at IS NULL
				  AND creativity.note_id=daily.note_id
				  AND ((daily.placement='信息流' AND campaign.placement=1) OR (daily.placement='搜索' AND campaign.placement=2))
			)
			AND ($2='%%' OR daily.note_id ILIKE $2 OR daily.placement ILIKE $2
			  OR EXISTS (
				SELECT 1
				FROM xhs_jg_creativities creativity
				JOIN xhs_jg_campaigns campaign
				  ON campaign.advertiser_id=creativity.advertiser_id
				 AND campaign.campaign_id=creativity.campaign_id
				 AND campaign.deleted_at IS NULL
				JOIN xhs_jg_advertisers advertiser ON advertiser.advertiser_id=campaign.advertiser_id
				LEFT JOIN xhs_jg_units unit
				  ON unit.advertiser_id=creativity.advertiser_id AND unit.unit_id=creativity.unit_id AND unit.deleted_at IS NULL
				WHERE creativity.deleted_at IS NULL
				  AND creativity.note_id=daily.note_id
				  AND ((daily.placement='信息流' AND campaign.placement=1) OR (daily.placement='搜索' AND campaign.placement=2))
				  AND (advertiser.advertiser_name ILIKE $2 OR campaign.campaign_name ILIKE $2
				    OR campaign.campaign_id::TEXT ILIKE $2
				    OR unit.unit_id::TEXT ILIKE $2 OR unit.unit_name ILIKE $2
				    OR creativity.creativity_id::TEXT ILIKE $2 OR creativity.creativity_name ILIKE $2)
			  ))
		)
		SELECT note_id, placement, spend, search_users, search_cost,
			COUNT(*) OVER()::INTEGER
		FROM filtered
		ORDER BY spend DESC, search_users DESC, note_id, placement
		LIMIT $3 OFFSET $4
	`, result.ReportDate, searchPattern, query.PageSize, offset)
	if err != nil {
		return result, fmt.Errorf("query Maituo XHS link keys: %w", err)
	}
	for rows.Next() {
		var item maituo.XHSLinkItem
		if err := rows.Scan(&item.NoteID, &item.Placement, &item.Spend, &item.SearchUsers, &item.SearchCost, &result.Total); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan Maituo XHS link key: %w", err)
		}
		item.Spend = roundMaituoMoney(item.Spend)
		item.SearchCost = roundMaituoMoney(item.SearchCost)
		// Daily metrics belong to this note-placement; real plans are returned in Matches.
		item.Matches = []maituo.XHSLinkMatch{}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate Maituo XHS link keys: %w", err)
	}
	rows.Close()
	if len(result.Items) == 0 {
		return result, nil
	}

	noteIDs := make([]string, len(result.Items))
	placements := make([]string, len(result.Items))
	for index := range result.Items {
		noteIDs[index] = result.Items[index].NoteID
		placements[index] = result.Items[index].Placement
	}
	linkRows, err := p.pool.Query(ctx, `
		WITH selected_keys AS (
			SELECT * FROM unnest($1::TEXT[], $2::TEXT[])
				WITH ORDINALITY AS key(note_id, placement, ordinal)
		)
		SELECT key.ordinal::INTEGER,
			advertiser.advertiser_id, advertiser.advertiser_name,
			campaign.campaign_id, campaign.campaign_name, campaign.campaign_filter_state, campaign.campaign_enable,
			campaign.marketing_target, campaign.placement, campaign.optimize_target,
			COALESCE(NULLIF(campaign.raw_payload->>'optimize_objective', '')::INTEGER, campaign.optimize_target),
			COALESCE(NULLIF(campaign.raw_payload->>'deep_optimize_objective', '')::INTEGER, -1), campaign.promotion_target,
			campaign.bidding_strategy, campaign.campaign_day_budget,
			COALESCE(campaign.campaign_created_at::TEXT, ''), COALESCE(campaign.campaign_updated_at::TEXT, ''),
			COALESCE(campaign.start_date::TEXT, ''), COALESCE(campaign.expire_date::TEXT, ''), campaign.synced_at::TEXT,
			COALESCE(unit.unit_id, creativity.unit_id), COALESCE(unit.unit_name, ''), COALESCE(unit.unit_enable, 0),
			COALESCE(unit.unit_filter_state, 0), COALESCE(unit.event_bid, 0), COALESCE(unit.target_type, 0),
			COALESCE(unit.not_available_status, 0), COALESCE(unit.creation_type, 0),
			COALESCE(unit.unit_created_at::TEXT, ''), COALESCE(unit.unit_updated_at::TEXT, ''), COALESCE(unit.synced_at::TEXT, ''),
			COALESCE(unit.raw_payload, '{}'::jsonb),
			creativity.creativity_id, creativity.creativity_name, creativity.creativity_enable,
			creativity.creativity_filter_state, creativity.material_type, creativity.conversion_type,
			creativity.note_id, creativity.item_id, creativity.audit_status, creativity.creativity_audit_state,
			creativity.creation_type, COALESCE(creativity.creativity_created_at::TEXT, ''),
			COALESCE(creativity.creativity_updated_at::TEXT, ''), creativity.synced_at::TEXT
		FROM selected_keys key
		JOIN xhs_jg_creativities creativity ON creativity.note_id=key.note_id AND creativity.deleted_at IS NULL
		JOIN xhs_jg_campaigns campaign
		  ON campaign.advertiser_id=creativity.advertiser_id AND campaign.campaign_id=creativity.campaign_id
		 AND campaign.deleted_at IS NULL
		 AND ((key.placement='信息流' AND campaign.placement=1) OR (key.placement='搜索' AND campaign.placement=2))
		JOIN xhs_jg_advertisers advertiser ON advertiser.advertiser_id=campaign.advertiser_id
		LEFT JOIN xhs_jg_units unit
		  ON unit.advertiser_id=creativity.advertiser_id AND unit.unit_id=creativity.unit_id AND unit.deleted_at IS NULL
		ORDER BY key.ordinal, advertiser.advertiser_id, campaign.campaign_id, creativity.unit_id, creativity.creativity_id
	`, noteIDs, placements)
	if err != nil {
		return result, fmt.Errorf("query linked XHS entities: %w", err)
	}
	lastOrdinal, lastMatch, lastUnit := -1, -1, int64(-1)
	var lastAdvertiser, lastCampaign int64 = -1, -1
	for linkRows.Next() {
		var ordinal int
		var match maituo.XHSLinkMatch
		var unit maituo.XHSLinkUnit
		var unitRaw []byte
		var creativity maituo.XHSLinkCreativity
		if err := linkRows.Scan(
			&ordinal, &match.AdvertiserID, &match.AdvertiserName,
			&match.CampaignID, &match.CampaignName, &match.CampaignFilterState, &match.CampaignEnable,
			&match.MarketingTarget, &match.Placement, &match.OptimizeTarget,
			&match.OptimizeObjective, &match.DeepOptimizeObjective, &match.PromotionTarget,
			&match.BiddingStrategy, &match.CampaignDayBudget, &match.CampaignCreatedAt, &match.CampaignUpdatedAt,
			&match.StartDate, &match.ExpireDate, &match.SyncedAt,
			&unit.UnitID, &unit.UnitName, &unit.UnitEnable, &unit.UnitFilterState, &unit.EventBid,
			&unit.TargetType, &unit.NotAvailableStatus, &unit.CreationType, &unit.CreatedAt, &unit.UpdatedAt, &unit.SyncedAt, &unitRaw,
			&creativity.CreativityID, &creativity.CreativityName, &creativity.CreativityEnable,
			&creativity.CreativityFilterState, &creativity.MaterialType, &creativity.ConversionType,
			&creativity.NoteID, &creativity.ItemID, &creativity.AuditStatus, &creativity.CreativityAuditState,
			&creativity.CreationType, &creativity.CreatedAt, &creativity.UpdatedAt, &creativity.SyncedAt,
		); err != nil {
			linkRows.Close()
			return result, fmt.Errorf("scan linked XHS entity: %w", err)
		}
		itemIndex := ordinal - 1
		if ordinal != lastOrdinal || match.AdvertiserID != lastAdvertiser || match.CampaignID != lastCampaign {
			match.Units = []maituo.XHSLinkUnit{}
			result.Items[itemIndex].Matches = append(result.Items[itemIndex].Matches, match)
			lastMatch = len(result.Items[itemIndex].Matches) - 1
			lastOrdinal, lastAdvertiser, lastCampaign, lastUnit = ordinal, match.AdvertiserID, match.CampaignID, -1
		}
		matchRef := &result.Items[itemIndex].Matches[lastMatch]
		if unit.UnitID != lastUnit {
			unit.Delivery = maituo.ParseXHSLinkUnitDelivery(unitRaw)
			unit.Creativities = []maituo.XHSLinkCreativity{}
			matchRef.Units = append(matchRef.Units, unit)
			lastUnit = unit.UnitID
		}
		unitIndex := len(matchRef.Units) - 1
		matchRef.Units[unitIndex].Creativities = append(matchRef.Units[unitIndex].Creativities, creativity)
	}
	if err := linkRows.Err(); err != nil {
		linkRows.Close()
		return result, fmt.Errorf("iterate linked XHS entities: %w", err)
	}
	linkRows.Close()
	return result, nil
}
