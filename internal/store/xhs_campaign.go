package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/xhs"

	"github.com/jackc/pgx/v5"
)

var ErrXHSCampaignSyncLocked = errors.New("another XHS Spotlight campaign sync is already running")

type XHSCampaignStoreResult struct {
	Upserted    int
	Deactivated int64
}

func (p *Postgres) ReplaceXHSCampaignSnapshot(ctx context.Context, advertiser xhs.Advertiser, campaigns []xhs.Campaign) (XHSCampaignStoreResult, error) {
	var result XHSCampaignStoreResult
	if advertiser.ID <= 0 {
		return result, errors.New("XHS Spotlight advertiser ID must be positive")
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, fmt.Errorf("begin XHS Spotlight campaign transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	lockKey := fmt.Sprintf("xhs-jg-campaigns:%d", advertiser.ID)
	var locked bool
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1,0))", lockKey).Scan(&locked); err != nil {
		return result, fmt.Errorf("acquire XHS Spotlight campaign sync lock: %w", err)
	}
	if !locked {
		return result, ErrXHSCampaignSyncLocked
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO xhs_jg_advertisers (advertiser_id, advertiser_name, first_seen_at, last_seen_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (advertiser_id) DO UPDATE SET
			advertiser_name = EXCLUDED.advertiser_name,
			last_seen_at = NOW()
	`, advertiser.ID, advertiser.Name); err != nil {
		return result, fmt.Errorf("upsert XHS Spotlight advertiser %d: %w", advertiser.ID, err)
	}
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE sync_xhs_campaign_ids (
			campaign_id BIGINT PRIMARY KEY
		) ON COMMIT DROP
	`); err != nil {
		return result, fmt.Errorf("create XHS Spotlight campaign staging table: %w", err)
	}

	batch := &pgx.Batch{}
	queued := 0
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

		batch.Queue("INSERT INTO sync_xhs_campaign_ids (campaign_id) VALUES ($1) ON CONFLICT DO NOTHING", campaign.CampaignID)
		batch.Queue(`
			INSERT INTO xhs_jg_campaigns (
				advertiser_id, campaign_id, campaign_name, campaign_filter_state,
				campaign_enable, marketing_target, placement, optimize_target,
				promotion_target, bidding_strategy, campaign_day_budget,
				campaign_created_at, campaign_updated_at, start_date, expire_date,
				raw_payload, first_seen_at, last_seen_at, synced_at, deleted_at
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::jsonb,
				NOW(),NOW(),NOW(),NULL
			)
			ON CONFLICT (advertiser_id, campaign_id) DO UPDATE SET
				campaign_name = EXCLUDED.campaign_name,
				campaign_filter_state = EXCLUDED.campaign_filter_state,
				campaign_enable = EXCLUDED.campaign_enable,
				marketing_target = EXCLUDED.marketing_target,
				placement = EXCLUDED.placement,
				optimize_target = EXCLUDED.optimize_target,
				promotion_target = EXCLUDED.promotion_target,
				bidding_strategy = EXCLUDED.bidding_strategy,
				campaign_day_budget = EXCLUDED.campaign_day_budget,
				campaign_created_at = EXCLUDED.campaign_created_at,
				campaign_updated_at = EXCLUDED.campaign_updated_at,
				start_date = EXCLUDED.start_date,
				expire_date = EXCLUDED.expire_date,
				raw_payload = EXCLUDED.raw_payload,
				last_seen_at = NOW(),
				synced_at = NOW(),
				deleted_at = NULL
		`, advertiser.ID, campaign.CampaignID, campaign.CampaignName, campaign.CampaignFilterState,
			campaign.CampaignEnable, campaign.MarketingTarget, campaign.Placement, campaign.OptimizeTarget,
			campaign.PromotionTarget, campaign.BiddingStrategy, campaign.CampaignDayBudget,
			createdAt, updatedAt, startDate, expireDate, string(raw))
		queued += 2
		result.Upserted++
	}

	results := tx.SendBatch(ctx, batch)
	for index := 0; index < queued; index++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return result, fmt.Errorf("execute XHS Spotlight campaign batch item %d: %w", index+1, err)
		}
	}
	if err := results.Close(); err != nil {
		return result, fmt.Errorf("close XHS Spotlight campaign batch: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE xhs_jg_campaigns AS target
		SET deleted_at = NOW(), synced_at = NOW()
		WHERE target.advertiser_id = $1
		  AND target.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM sync_xhs_campaign_ids AS source
			WHERE source.campaign_id = target.campaign_id
		  )
	`, advertiser.ID)
	if err != nil {
		return result, fmt.Errorf("deactivate missing XHS Spotlight campaigns: %w", err)
	}
	result.Deactivated = tag.RowsAffected()

	if _, err := tx.Exec(ctx, `
		UPDATE xhs_jg_advertisers
		SET last_full_synced_at = NOW(), last_seen_at = NOW()
		WHERE advertiser_id = $1
	`, advertiser.ID); err != nil {
		return result, fmt.Errorf("finish XHS Spotlight advertiser sync: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit XHS Spotlight campaign snapshot: %w", err)
	}
	return result, nil
}

func parseXHSCampaignTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.FixedZone("Asia/Shanghai", 8*60*60))
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseXHSCampaignDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
