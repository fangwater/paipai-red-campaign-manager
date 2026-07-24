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

type XHSEntityStoreResult struct {
	Upserted    int
	Deactivated int64
}

func (p *Postgres) ReplaceXHSUnitSnapshot(ctx context.Context, advertiser xhs.Advertiser, units []xhs.Unit) (XHSEntityStoreResult, error) {
	var result XHSEntityStoreResult
	tx, err := p.beginXHSEntitySnapshot(ctx, advertiser, "units")
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE sync_xhs_unit_ids (unit_id BIGINT PRIMARY KEY) ON COMMIT DROP`); err != nil {
		return result, fmt.Errorf("create XHS Spotlight unit staging table: %w", err)
	}
	batch := &pgx.Batch{}
	queued := 0
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

		batch.Queue("INSERT INTO sync_xhs_unit_ids (unit_id) VALUES ($1) ON CONFLICT DO NOTHING", unit.UnitID)
		batch.Queue(`
			INSERT INTO xhs_jg_units (
				advertiser_id, unit_id, campaign_id, unit_name, unit_enable,
				unit_filter_state, event_bid, target_type, not_available_status,
				creation_type, unit_created_at, unit_updated_at, raw_payload,
				first_seen_at, last_seen_at, synced_at, deleted_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,NOW(),NOW(),NOW(),NULL)
			ON CONFLICT (advertiser_id, unit_id) DO UPDATE SET
				campaign_id = EXCLUDED.campaign_id,
				unit_name = EXCLUDED.unit_name,
				unit_enable = EXCLUDED.unit_enable,
				unit_filter_state = EXCLUDED.unit_filter_state,
				event_bid = EXCLUDED.event_bid,
				target_type = EXCLUDED.target_type,
				not_available_status = EXCLUDED.not_available_status,
				creation_type = EXCLUDED.creation_type,
				unit_created_at = EXCLUDED.unit_created_at,
				unit_updated_at = EXCLUDED.unit_updated_at,
				raw_payload = EXCLUDED.raw_payload,
				last_seen_at = NOW(), synced_at = NOW(), deleted_at = NULL
		`, advertiser.ID, unit.UnitID, unit.CampaignID, unit.Name, unit.Enable,
			unit.UnitFilterState, unit.EventBid, unit.TargetType, unit.NotAvailableStatus,
			unit.CreationType, createdAt, updatedAt, string(raw))
		queued += 2
		result.Upserted++
	}
	if err := executeXHSBatch(ctx, tx, batch, queued, "unit"); err != nil {
		return result, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE xhs_jg_units AS target SET deleted_at = NOW(), synced_at = NOW()
		WHERE target.advertiser_id = $1 AND target.deleted_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM sync_xhs_unit_ids AS source WHERE source.unit_id = target.unit_id)
	`, advertiser.ID)
	if err != nil {
		return result, fmt.Errorf("deactivate missing XHS Spotlight units: %w", err)
	}
	result.Deactivated = tag.RowsAffected()
	if _, err := tx.Exec(ctx, `UPDATE xhs_jg_advertisers SET last_unit_full_synced_at=NOW(), last_seen_at=NOW() WHERE advertiser_id=$1`, advertiser.ID); err != nil {
		return result, fmt.Errorf("finish XHS Spotlight unit sync: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit XHS Spotlight unit snapshot: %w", err)
	}
	return result, nil
}

func (p *Postgres) ReplaceXHSCreativitySnapshot(ctx context.Context, advertiser xhs.Advertiser, creativities []xhs.Creativity) (XHSEntityStoreResult, error) {
	var result XHSEntityStoreResult
	tx, err := p.beginXHSEntitySnapshot(ctx, advertiser, "creativities")
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE sync_xhs_creativity_ids (creativity_id BIGINT PRIMARY KEY) ON COMMIT DROP`); err != nil {
		return result, fmt.Errorf("create XHS Spotlight creativity staging table: %w", err)
	}
	batch := &pgx.Batch{}
	queued := 0
	for _, creativity := range creativities {
		if creativity.CreativityID <= 0 || creativity.CampaignID <= 0 || creativity.UnitID <= 0 {
			return result, errors.New("XHS Spotlight creativity, campaign, and unit IDs must be positive")
		}
		raw, err := json.Marshal(creativity)
		if err != nil {
			return result, fmt.Errorf("encode XHS Spotlight creativity %d: %w", creativity.CreativityID, err)
		}
		createdAt, err := parseXHSJGTime(creativity.CreativityCreateTime)
		if err != nil {
			return result, fmt.Errorf("parse creativity %d create time: %w", creativity.CreativityID, err)
		}
		updatedAt, err := parseXHSJGTime(creativity.CreativityUpdateTime)
		if err != nil {
			return result, fmt.Errorf("parse creativity %d update time: %w", creativity.CreativityID, err)
		}

		batch.Queue("INSERT INTO sync_xhs_creativity_ids (creativity_id) VALUES ($1) ON CONFLICT DO NOTHING", creativity.CreativityID)
		batch.Queue(`
			INSERT INTO xhs_jg_creativities (
				advertiser_id, creativity_id, campaign_id, unit_id, creativity_name,
				creativity_enable, creativity_filter_state, material_type, conversion_type,
				note_id, item_id, audit_status, creativity_audit_state, creation_type,
				creativity_created_at, creativity_updated_at, raw_payload,
				first_seen_at, last_seen_at, synced_at, deleted_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17::jsonb,NOW(),NOW(),NOW(),NULL)
			ON CONFLICT (advertiser_id, creativity_id) DO UPDATE SET
				campaign_id = EXCLUDED.campaign_id,
				unit_id = EXCLUDED.unit_id,
				creativity_name = EXCLUDED.creativity_name,
				creativity_enable = EXCLUDED.creativity_enable,
				creativity_filter_state = EXCLUDED.creativity_filter_state,
				material_type = EXCLUDED.material_type,
				conversion_type = EXCLUDED.conversion_type,
				note_id = EXCLUDED.note_id,
				item_id = EXCLUDED.item_id,
				audit_status = EXCLUDED.audit_status,
				creativity_audit_state = EXCLUDED.creativity_audit_state,
				creation_type = EXCLUDED.creation_type,
				creativity_created_at = EXCLUDED.creativity_created_at,
				creativity_updated_at = EXCLUDED.creativity_updated_at,
				raw_payload = EXCLUDED.raw_payload,
				last_seen_at = NOW(), synced_at = NOW(), deleted_at = NULL
		`, advertiser.ID, creativity.CreativityID, creativity.CampaignID, creativity.UnitID,
			creativity.CreativityName, creativity.CreativityEnable, creativity.CreativityFilterState,
			creativity.MaterialType, creativity.ConversionType, creativity.NoteID, creativity.ItemID,
			creativity.AuditStatus, creativity.CreativityAuditState, creativity.CreationType,
			createdAt, updatedAt, string(raw))
		queued += 2
		result.Upserted++
	}
	if err := executeXHSBatch(ctx, tx, batch, queued, "creativity"); err != nil {
		return result, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE xhs_jg_creativities AS target SET deleted_at = NOW(), synced_at = NOW()
		WHERE target.advertiser_id = $1 AND target.deleted_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM sync_xhs_creativity_ids AS source WHERE source.creativity_id = target.creativity_id)
	`, advertiser.ID)
	if err != nil {
		return result, fmt.Errorf("deactivate missing XHS Spotlight creativities: %w", err)
	}
	result.Deactivated = tag.RowsAffected()
	if _, err := tx.Exec(ctx, `UPDATE xhs_jg_advertisers SET last_creativity_full_synced_at=NOW(), last_seen_at=NOW() WHERE advertiser_id=$1`, advertiser.ID); err != nil {
		return result, fmt.Errorf("finish XHS Spotlight creativity sync: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit XHS Spotlight creativity snapshot: %w", err)
	}
	return result, nil
}

func (p *Postgres) beginXHSEntitySnapshot(ctx context.Context, advertiser xhs.Advertiser, dimension string) (pgx.Tx, error) {
	if advertiser.ID <= 0 {
		return nil, errors.New("XHS Spotlight advertiser ID must be positive")
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin XHS Spotlight %s transaction: %w", dimension, err)
	}
	lockKey := fmt.Sprintf("xhs-jg-%s:%d", dimension, advertiser.ID)
	var locked bool
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1,0))", lockKey).Scan(&locked); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, fmt.Errorf("acquire XHS Spotlight %s sync lock: %w", dimension, err)
	}
	if !locked {
		_ = tx.Rollback(context.Background())
		return nil, ErrXHSCampaignSyncLocked
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO xhs_jg_advertisers (advertiser_id, advertiser_name, first_seen_at, last_seen_at)
		VALUES ($1,$2,NOW(),NOW())
		ON CONFLICT (advertiser_id) DO UPDATE SET advertiser_name=EXCLUDED.advertiser_name,last_seen_at=NOW()
	`, advertiser.ID, advertiser.Name); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, fmt.Errorf("upsert XHS Spotlight advertiser %d: %w", advertiser.ID, err)
	}
	return tx, nil
}

func executeXHSBatch(ctx context.Context, tx pgx.Tx, batch *pgx.Batch, queued int, dimension string) error {
	results := tx.SendBatch(ctx, batch)
	for index := 0; index < queued; index++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("execute XHS Spotlight %s batch item %d: %w", dimension, index+1, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close XHS Spotlight %s batch: %w", dimension, err)
	}
	return nil
}

func parseXHSJGTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\u00a0", " "))
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.FixedZone("Asia/Shanghai", 8*60*60))
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
