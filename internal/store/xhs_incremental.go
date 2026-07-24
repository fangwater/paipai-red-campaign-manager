package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"paipai-red-campaign-manager/internal/xhs"

	"github.com/jackc/pgx/v5"
)

func (p *Postgres) UpsertXHSCampaigns(ctx context.Context, advertiser xhs.Advertiser, campaigns []xhs.Campaign) (XHSCampaignStoreResult, error) {
	var result XHSCampaignStoreResult
	tx, err := p.beginXHSEntitySnapshot(ctx, advertiser, "campaigns")
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	batch := &pgx.Batch{}
	for _, campaign := range campaigns {
		if campaign.CampaignID <= 0 {
			return result, errors.New("XHS Spotlight campaign ID must be positive")
		}
		raw, err := json.Marshal(campaign)
		if err != nil {
			return result, fmt.Errorf("encode XHS Spotlight campaign %d: %w", campaign.CampaignID, err)
		}
		createdAt, err := parseXHSCampaignTime(campaign.CampaignCreateTime)
		if err != nil {
			return result, fmt.Errorf("parse campaign %d create time: %w", campaign.CampaignID, err)
		}
		updatedAt, err := parseXHSCampaignTime(campaign.CampaignUpdateTime)
		if err != nil {
			return result, fmt.Errorf("parse campaign %d update time: %w", campaign.CampaignID, err)
		}
		startDate, err := parseXHSCampaignDate(campaign.StartTime)
		if err != nil {
			return result, fmt.Errorf("parse campaign %d start date: %w", campaign.CampaignID, err)
		}
		expireDate, err := parseXHSCampaignDate(campaign.ExpireTime)
		if err != nil {
			return result, fmt.Errorf("parse campaign %d expire date: %w", campaign.CampaignID, err)
		}
		batch.Queue(`
			INSERT INTO xhs_jg_campaigns (
				advertiser_id, campaign_id, campaign_name, campaign_filter_state,
				campaign_enable, marketing_target, placement, optimize_target,
				promotion_target, bidding_strategy, campaign_day_budget,
				campaign_created_at, campaign_updated_at, start_date, expire_date,
				raw_payload, first_seen_at, last_seen_at, synced_at, deleted_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::jsonb,NOW(),NOW(),NOW(),NULL)
			ON CONFLICT (advertiser_id, campaign_id) DO UPDATE SET
				campaign_name=EXCLUDED.campaign_name,
				campaign_filter_state=EXCLUDED.campaign_filter_state,
				campaign_enable=EXCLUDED.campaign_enable,
				marketing_target=EXCLUDED.marketing_target,
				placement=EXCLUDED.placement,
				optimize_target=EXCLUDED.optimize_target,
				promotion_target=EXCLUDED.promotion_target,
				bidding_strategy=EXCLUDED.bidding_strategy,
				campaign_day_budget=EXCLUDED.campaign_day_budget,
				campaign_created_at=EXCLUDED.campaign_created_at,
				campaign_updated_at=EXCLUDED.campaign_updated_at,
				start_date=EXCLUDED.start_date,
				expire_date=EXCLUDED.expire_date,
				raw_payload=EXCLUDED.raw_payload,
				last_seen_at=NOW(), synced_at=NOW(), deleted_at=NULL
		`, advertiser.ID, campaign.CampaignID, campaign.CampaignName, campaign.CampaignFilterState,
			campaign.CampaignEnable, campaign.MarketingTarget, campaign.Placement, campaign.OptimizeTarget,
			campaign.PromotionTarget, campaign.BiddingStrategy, campaign.CampaignDayBudget,
			createdAt, updatedAt, startDate, expireDate, string(raw))
		result.Upserted++
	}
	if err := executeXHSBatch(ctx, tx, batch, result.Upserted, "campaign incremental"); err != nil {
		return result, err
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit XHS Spotlight campaign incremental upsert: %w", err)
	}
	return result, nil
}

func (p *Postgres) UpsertXHSUnits(ctx context.Context, advertiser xhs.Advertiser, units []xhs.Unit) (XHSEntityStoreResult, error) {
	var result XHSEntityStoreResult
	tx, err := p.beginXHSEntitySnapshot(ctx, advertiser, "units")
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	batch := &pgx.Batch{}
	for _, unit := range units {
		if unit.UnitID <= 0 || unit.CampaignID <= 0 {
			return result, errors.New("XHS Spotlight unit and campaign IDs must be positive")
		}
		raw, err := json.Marshal(unit)
		if err != nil {
			return result, fmt.Errorf("encode XHS Spotlight unit %d: %w", unit.UnitID, err)
		}
		createdAt, err := parseXHSJGTime(unit.CreateTime)
		if err != nil {
			return result, fmt.Errorf("parse unit %d create time: %w", unit.UnitID, err)
		}
		updatedAt, err := parseXHSJGTime(unit.UpdateTime)
		if err != nil {
			return result, fmt.Errorf("parse unit %d update time: %w", unit.UnitID, err)
		}
		batch.Queue(`
			INSERT INTO xhs_jg_units (
				advertiser_id, unit_id, campaign_id, unit_name, unit_enable,
				unit_filter_state, event_bid, target_type, not_available_status,
				creation_type, unit_created_at, unit_updated_at, raw_payload,
				first_seen_at, last_seen_at, synced_at, deleted_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,NOW(),NOW(),NOW(),NULL)
			ON CONFLICT (advertiser_id, unit_id) DO UPDATE SET
				campaign_id=EXCLUDED.campaign_id,
				unit_name=EXCLUDED.unit_name,
				unit_enable=EXCLUDED.unit_enable,
				unit_filter_state=EXCLUDED.unit_filter_state,
				event_bid=EXCLUDED.event_bid,
				target_type=EXCLUDED.target_type,
				not_available_status=EXCLUDED.not_available_status,
				creation_type=EXCLUDED.creation_type,
				unit_created_at=EXCLUDED.unit_created_at,
				unit_updated_at=EXCLUDED.unit_updated_at,
				raw_payload=EXCLUDED.raw_payload,
				last_seen_at=NOW(), synced_at=NOW(), deleted_at=NULL
		`, advertiser.ID, unit.UnitID, unit.CampaignID, unit.Name, unit.Enable,
			unit.UnitFilterState, unit.EventBid, unit.TargetType, unit.NotAvailableStatus,
			unit.CreationType, createdAt, updatedAt, string(raw))
		result.Upserted++
	}
	if err := executeXHSBatch(ctx, tx, batch, result.Upserted, "unit incremental"); err != nil {
		return result, err
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit XHS Spotlight unit incremental upsert: %w", err)
	}
	return result, nil
}
