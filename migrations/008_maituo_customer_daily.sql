CREATE TABLE IF NOT EXISTS maituo_customer_daily_import_runs (
    id BIGSERIAL PRIMARY KEY,
    file_name TEXT NOT NULL,
    file_sha256 TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    table_stats JSONB NOT NULL DEFAULT '[]'::jsonb,
    fetched_count INTEGER NOT NULL DEFAULT 0,
    inserted_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0,
    unchanged_count INTEGER NOT NULL DEFAULT 0,
    deleted_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_message TEXT
);

CREATE TABLE IF NOT EXISTS maituo_customer_daily_kpis (
    metric TEXT PRIMARY KEY,
    metric_value NUMERIC(20, 6) NOT NULL,
    data_basis TEXT NOT NULL,
    source_row_number INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    import_run_id BIGINT NOT NULL REFERENCES maituo_customer_daily_import_runs(id),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS maituo_customer_daily_notes (
    note_id TEXT NOT NULL,
    note_url TEXT NOT NULL,
    category TEXT NOT NULL,
    subaccount TEXT NOT NULL,
    campaign_name TEXT NOT NULL,
    placement TEXT NOT NULL,
    keyword_category_note TEXT,
    spend NUMERIC(20, 6) NOT NULL,
    search_users BIGINT NOT NULL,
    search_cost NUMERIC(20, 6),
    estimated_postback_cost NUMERIC(20, 6),
    search_rate_pct NUMERIC(20, 6),
    cpc NUMERIC(20, 6) NOT NULL,
    ctr_pct NUMERIC(20, 6) NOT NULL,
    source_row_number INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    import_run_id BIGINT NOT NULL REFERENCES maituo_customer_daily_import_runs(id),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (note_id, subaccount, campaign_name, placement)
);

CREATE TABLE IF NOT EXISTS maituo_customer_daily_spus (
    spu TEXT PRIMARY KEY,
    auction_spend NUMERIC(20, 6) NOT NULL,
    search_users BIGINT NOT NULL,
    search_cost NUMERIC(20, 6) NOT NULL,
    search_rate_pct NUMERIC(20, 6) NOT NULL,
    cpc NUMERIC(20, 6) NOT NULL,
    ctr_pct NUMERIC(20, 6) NOT NULL,
    note_count BIGINT NOT NULL,
    source_row_number INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    import_run_id BIGINT NOT NULL REFERENCES maituo_customer_daily_import_runs(id),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS maituo_customer_daily_subaccounts (
    spu TEXT NOT NULL,
    subaccount TEXT NOT NULL,
    placement TEXT NOT NULL,
    search_cost NUMERIC(20, 6),
    estimated_postback_cost NUMERIC(20, 6),
    spend NUMERIC(20, 6) NOT NULL,
    search_users BIGINT NOT NULL,
    search_rate_pct NUMERIC(20, 6),
    cpc NUMERIC(20, 6),
    ctr_pct NUMERIC(20, 6),
    note_count BIGINT NOT NULL,
    source_row_number INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    import_run_id BIGINT NOT NULL REFERENCES maituo_customer_daily_import_runs(id),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (spu, subaccount, placement)
);

CREATE TABLE IF NOT EXISTS maituo_customer_daily_trends (
    report_date DATE PRIMARY KEY,
    coenzyme_spend NUMERIC(20, 6),
    coenzyme_search_uv BIGINT,
    coenzyme_order_uv BIGINT,
    coenzyme_search_cost NUMERIC(20, 6),
    krill_oil_spend NUMERIC(20, 6),
    krill_oil_search_uv BIGINT,
    krill_oil_order_uv BIGINT,
    krill_oil_search_cost NUMERIC(20, 6),
    total_search_uv BIGINT,
    total_order_uv BIGINT,
    total_search_cost NUMERIC(20, 6),
    total_spend NUMERIC(20, 6),
    total_recall_search_cost NUMERIC(20, 6),
    source_row_number INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    import_run_id BIGINT NOT NULL REFERENCES maituo_customer_daily_import_runs(id),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_maituo_notes_active ON maituo_customer_daily_notes (note_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_maituo_kpis_active ON maituo_customer_daily_kpis (metric) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_maituo_spus_active ON maituo_customer_daily_spus (spu) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_maituo_subaccounts_active ON maituo_customer_daily_subaccounts (spu, subaccount) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_maituo_trends_active ON maituo_customer_daily_trends (report_date DESC) WHERE deleted_at IS NULL;

DROP TABLE IF EXISTS excel_import_rows;
DROP TABLE IF EXISTS excel_import_datasets;
DROP TABLE IF EXISTS excel_import_runs;
