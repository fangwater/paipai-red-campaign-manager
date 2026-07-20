CREATE TABLE IF NOT EXISTS lark_bitable_records (
    app_token TEXT NOT NULL,
    table_id TEXT NOT NULL,
    record_id TEXT NOT NULL,
    fields JSONB NOT NULL DEFAULT '{}'::jsonb,
    lark_created_at TIMESTAMPTZ,
    lark_updated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (app_token, table_id, record_id)
);

CREATE INDEX IF NOT EXISTS idx_lark_bitable_records_active
    ON lark_bitable_records (app_token, table_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_lark_bitable_records_fields
    ON lark_bitable_records USING GIN (fields);

CREATE TABLE IF NOT EXISTS sync_runs (
    id BIGSERIAL PRIMARY KEY,
    app_token TEXT NOT NULL,
    table_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    fetched_count INTEGER NOT NULL DEFAULT 0,
    upserted_count INTEGER NOT NULL DEFAULT 0,
    deleted_count INTEGER NOT NULL DEFAULT 0,
    tables_count INTEGER NOT NULL DEFAULT 0,
    documents_fetched INTEGER NOT NULL DEFAULT 0,
    document_errors INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_sync_runs_table_started
    ON sync_runs (app_token, table_id, started_at DESC);
CREATE TABLE IF NOT EXISTS lark_bitable_tables (
    app_token TEXT NOT NULL,
    table_id TEXT NOT NULL,
    name TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 0,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (app_token, table_id)
);

CREATE TABLE IF NOT EXISTS lark_linked_documents (
    provider TEXT NOT NULL,
    resource_key TEXT NOT NULL,
    source_url TEXT NOT NULL,
    document_type TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    revision_id INTEGER NOT NULL DEFAULT 0,
    fetch_status TEXT NOT NULL CHECK (fetch_status IN ('succeeded', 'failed', 'auth_required')),
    error_message TEXT,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, resource_key)
);

CREATE INDEX IF NOT EXISTS idx_lark_linked_documents_status
    ON lark_linked_documents (fetch_status, fetched_at);

CREATE TABLE IF NOT EXISTS lark_record_documents (
    app_token TEXT NOT NULL,
    table_id TEXT NOT NULL,
    record_id TEXT NOT NULL,
    field_name TEXT NOT NULL,
    provider TEXT NOT NULL,
    resource_key TEXT NOT NULL,
    source_url TEXT NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (
        app_token,
        table_id,
        record_id,
        field_name,
        provider,
        resource_key
    )
);

CREATE INDEX IF NOT EXISTS idx_lark_record_documents_resource
    ON lark_record_documents (provider, resource_key);


ALTER TABLE sync_runs ADD COLUMN IF NOT EXISTS tables_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sync_runs ADD COLUMN IF NOT EXISTS documents_fetched INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sync_runs ADD COLUMN IF NOT EXISTS document_errors INTEGER NOT NULL DEFAULT 0;
