CREATE TABLE IF NOT EXISTS coenzyme_q10_sync_runs (
    run_id BIGSERIAL PRIMARY KEY,
    source_wiki_token TEXT NOT NULL,
    spreadsheet_token TEXT,
    sheet_id TEXT NOT NULL,
    sheet_name TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    fetched_count INTEGER NOT NULL DEFAULT 0,
    inserted_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0,
    unchanged_count INTEGER NOT NULL DEFAULT 0,
    earliest_date DATE,
    latest_date DATE,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_coenzyme_q10_sync_runs_single_running
    ON coenzyme_q10_sync_runs ((1))
    WHERE status = 'running';

CREATE INDEX IF NOT EXISTS idx_coenzyme_q10_sync_runs_started
    ON coenzyme_q10_sync_runs (started_at DESC);

CREATE TABLE IF NOT EXISTS coenzyme_q10_daily (
    report_date DATE PRIMARY KEY,
    spend NUMERIC(20, 6),
    impressions BIGINT,
    clicks BIGINT,
    ctr NUMERIC(20, 12),
    cpc NUMERIC(20, 12),
    cpm NUMERIC(20, 12),
    all_transaction_gmv NUMERIC(20, 6),
    all_store_roi NUMERIC(20, 12),
    post_refund_gmv NUMERIC(20, 6),
    post_refund_roi NUMERIC(20, 12),
    coenzyme_gmv NUMERIC(20, 6),
    coenzyme_roi NUMERIC(20, 12),
    same_day_gmv NUMERIC(20, 6),
    same_day_roi NUMERIC(20, 12),
    search_spend NUMERIC(20, 6),
    search_gmv NUMERIC(20, 6),
    search_roi NUMERIC(20, 12),
    search_spend_ratio NUMERIC(20, 12),
    source_row_number INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    sync_run_id BIGINT NOT NULL REFERENCES coenzyme_q10_sync_runs(run_id),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_coenzyme_q10_daily_updated
    ON coenzyme_q10_daily (updated_at DESC);

COMMENT ON TABLE coenzyme_q10_daily IS '飞书“辅酶q10日数据”页签的按日期增量快照；源表暂缺的日期不会被删除';
