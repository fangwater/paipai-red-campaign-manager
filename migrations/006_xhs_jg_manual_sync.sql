ALTER TABLE xhs_jg_advertisers
    ADD COLUMN IF NOT EXISTS last_incremental_synced_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_campaign_incremental_synced_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_unit_incremental_synced_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS xhs_jg_sync_runs (
    run_id BIGSERIAL PRIMARY KEY,
    mode TEXT NOT NULL CHECK (mode IN ('incremental', 'full')),
    trigger_type TEXT NOT NULL CHECK (trigger_type IN ('api', 'schedule', 'startup', 'cli')),
    requested_advertiser_id BIGINT,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    advertisers_count INTEGER NOT NULL DEFAULT 0,
    campaigns_count INTEGER NOT NULL DEFAULT 0,
    units_count INTEGER NOT NULL DEFAULT 0,
    creativities_count INTEGER NOT NULL DEFAULT 0,
    deactivated_count BIGINT NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

ALTER TABLE xhs_jg_sync_runs
    ADD COLUMN IF NOT EXISTS target TEXT NOT NULL DEFAULT 'all';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'xhs_jg_sync_runs_target_check') THEN
        ALTER TABLE xhs_jg_sync_runs
            ADD CONSTRAINT xhs_jg_sync_runs_target_check
            CHECK (target IN ('all', 'campaigns', 'units', 'creativities'));
    END IF;
END $$;

UPDATE xhs_jg_advertisers SET
    last_campaign_incremental_synced_at = COALESCE(last_campaign_incremental_synced_at, last_incremental_synced_at),
    last_unit_incremental_synced_at = COALESCE(last_unit_incremental_synced_at, last_incremental_synced_at)
WHERE last_incremental_synced_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_xhs_jg_sync_runs_started
    ON xhs_jg_sync_runs (started_at DESC);

CREATE INDEX IF NOT EXISTS idx_xhs_jg_sync_runs_status
    ON xhs_jg_sync_runs (status, started_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_xhs_jg_sync_runs_single_running
    ON xhs_jg_sync_runs ((1))
    WHERE status = 'running';

COMMENT ON TABLE xhs_jg_sync_runs IS '小红书聚光计划、单元、创意手动刷新作业记录';
COMMENT ON COLUMN xhs_jg_advertisers.last_incremental_synced_at IS '兼容旧版统一同步的历史游标；新逻辑不再读写';
COMMENT ON COLUMN xhs_jg_advertisers.last_campaign_incremental_synced_at IS '计划手动增量刷新的独立游标';
COMMENT ON COLUMN xhs_jg_advertisers.last_unit_incremental_synced_at IS '单元手动增量刷新的独立游标';
