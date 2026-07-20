CREATE TABLE IF NOT EXISTS service_provider_content_tables (
    provider_code TEXT PRIMARY KEY,
    provider_name TEXT NOT NULL UNIQUE,
    source_type TEXT NOT NULL DEFAULT 'feishu_sheet'
        CHECK (source_type = 'feishu_sheet'),
    source_url TEXT,
    wiki_token TEXT,
    spreadsheet_token TEXT,
    sheet_id TEXT,
    sheet_name TEXT NOT NULL DEFAULT '达人笔记执行表',
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    last_synced_at TIMESTAMPTZ,
    last_sync_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (last_sync_status IN ('pending', 'running', 'succeeded', 'failed')),
    last_sync_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (NOT enabled OR (source_url IS NOT NULL AND wiki_token IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS service_provider_note_executions (
    provider_code TEXT NOT NULL REFERENCES service_provider_content_tables(provider_code),
    record_key TEXT NOT NULL,
    source_row_number INTEGER NOT NULL CHECK (source_row_number >= 2),
    submission_date TEXT,
    note_id TEXT,
    content_type TEXT,
    cover_type TEXT,
    commercial_intensity TEXT,
    audience TEXT,
    user_scenario TEXT,
    note_type TEXT,
    progress TEXT,
    review_feedback TEXT,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (provider_code, record_key)
);

CREATE INDEX IF NOT EXISTS idx_provider_note_executions_active
    ON service_provider_note_executions (provider_code, note_id)
    WHERE deleted_at IS NULL;

INSERT INTO service_provider_content_tables (
    provider_code, provider_name, source_url, wiki_token, sheet_id, sheet_name, enabled
) VALUES
    (
        'manjie',
        '曼杰',
        'https://my.feishu.cn/wiki/T9IJwUdbxiYEIBktxR4caypbneV?sheet=a7d9da',
        'T9IJwUdbxiYEIBktxR4caypbneV',
        'a7d9da',
        '达人笔记执行表',
        TRUE
    ),
    (
        'youyiyouer',
        '有一有二',
        NULL,
        NULL,
        NULL,
        '达人笔记执行表',
        FALSE
    )
ON CONFLICT (provider_code) DO NOTHING;
