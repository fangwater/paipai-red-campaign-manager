ALTER TABLE xhs_jg_advertisers
    ADD COLUMN IF NOT EXISTS last_unit_full_synced_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_creativity_full_synced_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS xhs_jg_units (
    advertiser_id BIGINT NOT NULL REFERENCES xhs_jg_advertisers(advertiser_id) ON DELETE CASCADE,
    unit_id BIGINT NOT NULL,
    campaign_id BIGINT NOT NULL,
    unit_name TEXT NOT NULL DEFAULT '',
    unit_enable INTEGER NOT NULL DEFAULT 0,
    unit_filter_state INTEGER NOT NULL DEFAULT 0,
    event_bid BIGINT NOT NULL DEFAULT 0,
    target_type INTEGER NOT NULL DEFAULT 0,
    not_available_status INTEGER NOT NULL DEFAULT 0,
    creation_type INTEGER NOT NULL DEFAULT 0,
    unit_created_at TIMESTAMPTZ,
    unit_updated_at TIMESTAMPTZ,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (advertiser_id, unit_id)
);

CREATE INDEX IF NOT EXISTS idx_xhs_jg_units_campaign
    ON xhs_jg_units (advertiser_id, campaign_id, unit_filter_state)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_xhs_jg_units_name
    ON xhs_jg_units (advertiser_id, unit_name)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_xhs_jg_units_raw
    ON xhs_jg_units USING GIN (raw_payload);

CREATE TABLE IF NOT EXISTS xhs_jg_creativities (
    advertiser_id BIGINT NOT NULL REFERENCES xhs_jg_advertisers(advertiser_id) ON DELETE CASCADE,
    creativity_id BIGINT NOT NULL,
    campaign_id BIGINT NOT NULL,
    unit_id BIGINT NOT NULL,
    creativity_name TEXT NOT NULL DEFAULT '',
    creativity_enable INTEGER NOT NULL DEFAULT 0,
    creativity_filter_state INTEGER NOT NULL DEFAULT 0,
    material_type INTEGER NOT NULL DEFAULT 0,
    conversion_type INTEGER NOT NULL DEFAULT 0,
    note_id TEXT NOT NULL DEFAULT '',
    item_id TEXT NOT NULL DEFAULT '',
    audit_status INTEGER NOT NULL DEFAULT 0,
    creativity_audit_state INTEGER NOT NULL DEFAULT 0,
    creation_type INTEGER NOT NULL DEFAULT 0,
    creativity_created_at TIMESTAMPTZ,
    creativity_updated_at TIMESTAMPTZ,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (advertiser_id, creativity_id)
);

CREATE INDEX IF NOT EXISTS idx_xhs_jg_creativities_unit
    ON xhs_jg_creativities (advertiser_id, unit_id, creativity_filter_state)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_xhs_jg_creativities_campaign
    ON xhs_jg_creativities (advertiser_id, campaign_id, creativity_filter_state)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_xhs_jg_creativities_note
    ON xhs_jg_creativities (advertiser_id, note_id)
    WHERE deleted_at IS NULL AND note_id <> '';

CREATE INDEX IF NOT EXISTS idx_xhs_jg_creativities_raw
    ON xhs_jg_creativities USING GIN (raw_payload);

COMMENT ON TABLE xhs_jg_units IS '小红书聚光单元最新全量快照；deleted_at 表示已不在本次未删除结果中';
COMMENT ON COLUMN xhs_jg_units.raw_payload IS '聚光获取单元列表接口返回的完整单元原文';
COMMENT ON TABLE xhs_jg_creativities IS '小红书聚光创意最新全量快照；deleted_at 表示已不在 status=2 结果中';
COMMENT ON COLUMN xhs_jg_creativities.raw_payload IS '聚光创意查询接口返回的完整创意原文';
