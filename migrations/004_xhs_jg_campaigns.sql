CREATE TABLE IF NOT EXISTS xhs_jg_advertisers (
    advertiser_id BIGINT PRIMARY KEY,
    advertiser_name TEXT NOT NULL DEFAULT '',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_full_synced_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS xhs_jg_campaigns (
    advertiser_id BIGINT NOT NULL REFERENCES xhs_jg_advertisers(advertiser_id) ON DELETE CASCADE,
    campaign_id BIGINT NOT NULL,
    campaign_name TEXT NOT NULL DEFAULT '',
    campaign_filter_state INTEGER NOT NULL DEFAULT 0,
    campaign_enable INTEGER NOT NULL DEFAULT 0,
    marketing_target INTEGER NOT NULL DEFAULT 0,
    placement INTEGER NOT NULL DEFAULT 0,
    optimize_target INTEGER NOT NULL DEFAULT 0,
    promotion_target INTEGER NOT NULL DEFAULT 0,
    bidding_strategy INTEGER NOT NULL DEFAULT 0,
    campaign_day_budget BIGINT NOT NULL DEFAULT 0,
    campaign_created_at TIMESTAMPTZ,
    campaign_updated_at TIMESTAMPTZ,
    start_date DATE,
    expire_date DATE,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (advertiser_id, campaign_id)
);

CREATE INDEX IF NOT EXISTS idx_xhs_jg_campaigns_active
    ON xhs_jg_campaigns (advertiser_id, campaign_filter_state, campaign_updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_xhs_jg_campaigns_name
    ON xhs_jg_campaigns (advertiser_id, campaign_name)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_xhs_jg_campaigns_raw
    ON xhs_jg_campaigns USING GIN (raw_payload);

COMMENT ON TABLE xhs_jg_advertisers IS '小红书聚光 OAuth 授权广告主及计划全量同步时间';
COMMENT ON TABLE xhs_jg_campaigns IS '小红书聚光推广计划最新全量快照；deleted_at 表示已不在 status=6 结果中';
COMMENT ON COLUMN xhs_jg_campaigns.raw_payload IS '聚光查询计划接口返回的完整计划原文';
