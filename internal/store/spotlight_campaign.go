package store

import (
	"context"
	"fmt"
	"strings"

	"paipai-red-campaign-manager/internal/maituo"

	"github.com/jackc/pgx/v5"
)

func (p *Postgres) SpotlightCampaigns(ctx context.Context, query maituo.SpotlightCampaignQuery) (maituo.SpotlightCampaignList, error) {
	result := maituo.SpotlightCampaignList{Page: query.Page, PageSize: query.PageSize, Items: []maituo.SpotlightCampaignSummary{}}
	pattern := "%" + strings.TrimSpace(query.Search) + "%"
	offset := (query.Page - 1) * query.PageSize
	rows, err := p.pool.Query(ctx, `
		SELECT campaign.advertiser_id, advertiser.advertiser_name,
			campaign.campaign_id, campaign.campaign_name, campaign.campaign_filter_state,
			campaign.campaign_enable, campaign.marketing_target, campaign.placement,
			campaign.bidding_strategy, campaign.campaign_day_budget,
			COALESCE(campaign.start_date::TEXT, ''), COALESCE(campaign.expire_date::TEXT, ''),
			COALESCE(campaign.campaign_updated_at::TEXT, ''), campaign.synced_at::TEXT,
			(SELECT COUNT(*)::INTEGER FROM xhs_jg_units unit
			 WHERE unit.advertiser_id=campaign.advertiser_id AND unit.campaign_id=campaign.campaign_id AND unit.deleted_at IS NULL),
			(SELECT COUNT(*)::INTEGER FROM xhs_jg_creativities creativity
			 WHERE creativity.advertiser_id=campaign.advertiser_id AND creativity.campaign_id=campaign.campaign_id AND creativity.deleted_at IS NULL),
			COUNT(*) OVER()::INTEGER
		FROM xhs_jg_campaigns campaign
		JOIN xhs_jg_advertisers advertiser ON advertiser.advertiser_id=campaign.advertiser_id
		WHERE campaign.deleted_at IS NULL
		  AND ($1='%%' OR campaign.campaign_name ILIKE $1 OR campaign.campaign_id::TEXT ILIKE $1)
		ORDER BY campaign.campaign_updated_at DESC NULLS LAST, campaign.campaign_id DESC
		LIMIT $2 OFFSET $3
	`, pattern, query.PageSize, offset)
	if err != nil {
		return result, fmt.Errorf("query Spotlight campaigns: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item maituo.SpotlightCampaignSummary
		if err := rows.Scan(
			&item.AdvertiserID, &item.AdvertiserName, &item.CampaignID, &item.CampaignName,
			&item.CampaignFilterState, &item.CampaignEnable, &item.MarketingTarget, &item.Placement,
			&item.BiddingStrategy, &item.CampaignDayBudget, &item.StartDate, &item.ExpireDate,
			&item.UpdatedAt, &item.SyncedAt, &item.UnitCount, &item.CreativityCount, &result.Total,
		); err != nil {
			return result, fmt.Errorf("scan Spotlight campaign: %w", err)
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate Spotlight campaigns: %w", err)
	}
	return result, nil
}

func (p *Postgres) SpotlightCampaignDetail(ctx context.Context, advertiserID, campaignID int64) (maituo.SpotlightCampaignDetail, bool, error) {
	result := maituo.SpotlightCampaignDetail{
		Units:        []maituo.SpotlightCampaignEntity{},
		Creativities: []maituo.SpotlightCampaignEntity{},
	}
	err := p.pool.QueryRow(ctx, `
		SELECT campaign.advertiser_id, advertiser.advertiser_name,
			campaign.campaign_id, campaign.campaign_name, campaign.campaign_filter_state,
			campaign.campaign_enable, campaign.marketing_target, campaign.placement,
			campaign.bidding_strategy, campaign.campaign_day_budget,
			COALESCE(campaign.start_date::TEXT, ''), COALESCE(campaign.expire_date::TEXT, ''),
			COALESCE(campaign.campaign_updated_at::TEXT, ''), campaign.synced_at::TEXT,
			(SELECT COUNT(*)::INTEGER FROM xhs_jg_units unit
			 WHERE unit.advertiser_id=campaign.advertiser_id AND unit.campaign_id=campaign.campaign_id AND unit.deleted_at IS NULL),
			(SELECT COUNT(*)::INTEGER FROM xhs_jg_creativities creativity
			 WHERE creativity.advertiser_id=campaign.advertiser_id AND creativity.campaign_id=campaign.campaign_id AND creativity.deleted_at IS NULL),
			campaign.raw_payload
		FROM xhs_jg_campaigns campaign
		JOIN xhs_jg_advertisers advertiser ON advertiser.advertiser_id=campaign.advertiser_id
		WHERE campaign.advertiser_id=$1 AND campaign.campaign_id=$2 AND campaign.deleted_at IS NULL
	`, advertiserID, campaignID).Scan(
		&result.Campaign.AdvertiserID, &result.Campaign.AdvertiserName,
		&result.Campaign.CampaignID, &result.Campaign.CampaignName,
		&result.Campaign.CampaignFilterState, &result.Campaign.CampaignEnable,
		&result.Campaign.MarketingTarget, &result.Campaign.Placement,
		&result.Campaign.BiddingStrategy, &result.Campaign.CampaignDayBudget,
		&result.Campaign.StartDate, &result.Campaign.ExpireDate,
		&result.Campaign.UpdatedAt, &result.Campaign.SyncedAt,
		&result.Campaign.UnitCount, &result.Campaign.CreativityCount, &result.RawPayload,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return result, false, nil
		}
		return result, false, fmt.Errorf("query Spotlight campaign detail: %w", err)
	}

	unitRows, err := p.pool.Query(ctx, `
		SELECT unit.unit_id, unit.unit_name, unit.campaign_id, unit.unit_enable,
			unit.unit_filter_state, COALESCE(unit.unit_created_at::TEXT, ''),
			COALESCE(unit.unit_updated_at::TEXT, ''), unit.synced_at::TEXT, unit.raw_payload
		FROM xhs_jg_units unit
		WHERE unit.advertiser_id=$1 AND unit.campaign_id=$2 AND unit.deleted_at IS NULL
		ORDER BY unit.unit_id
	`, advertiserID, campaignID)
	if err != nil {
		return result, false, fmt.Errorf("query Spotlight campaign units: %w", err)
	}
	for unitRows.Next() {
		var item maituo.SpotlightCampaignEntity
		if err := unitRows.Scan(&item.ID, &item.Name, &item.CampaignID, &item.Enable, &item.FilterState,
			&item.CreatedAt, &item.UpdatedAt, &item.SyncedAt, &item.RawPayload); err != nil {
			unitRows.Close()
			return result, false, fmt.Errorf("scan Spotlight campaign unit: %w", err)
		}
		result.Units = append(result.Units, item)
	}
	if err := unitRows.Err(); err != nil {
		unitRows.Close()
		return result, false, fmt.Errorf("iterate Spotlight campaign units: %w", err)
	}
	unitRows.Close()

	creativityRows, err := p.pool.Query(ctx, `
		SELECT creativity.creativity_id, creativity.creativity_name, creativity.campaign_id,
			creativity.unit_id, creativity.creativity_enable, creativity.creativity_filter_state,
			COALESCE(creativity.creativity_created_at::TEXT, ''),
			COALESCE(creativity.creativity_updated_at::TEXT, ''), creativity.synced_at::TEXT,
			creativity.raw_payload
		FROM xhs_jg_creativities creativity
		WHERE creativity.advertiser_id=$1 AND creativity.campaign_id=$2 AND creativity.deleted_at IS NULL
		ORDER BY creativity.unit_id, creativity.creativity_id
	`, advertiserID, campaignID)
	if err != nil {
		return result, false, fmt.Errorf("query Spotlight campaign creativities: %w", err)
	}
	defer creativityRows.Close()
	for creativityRows.Next() {
		var item maituo.SpotlightCampaignEntity
		if err := creativityRows.Scan(&item.ID, &item.Name, &item.CampaignID, &item.UnitID,
			&item.Enable, &item.FilterState, &item.CreatedAt, &item.UpdatedAt,
			&item.SyncedAt, &item.RawPayload); err != nil {
			return result, false, fmt.Errorf("scan Spotlight campaign creativity: %w", err)
		}
		result.Creativities = append(result.Creativities, item)
	}
	if err := creativityRows.Err(); err != nil {
		return result, false, fmt.Errorf("iterate Spotlight campaign creativities: %w", err)
	}
	return result, true, nil
}
