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
    cover_type TEXT,
    commercial_intensity TEXT,
    audience TEXT,
    user_scenario TEXT,
    note_type TEXT,
    progress TEXT,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (provider_code, record_key)
);

ALTER TABLE service_provider_note_executions DROP COLUMN IF EXISTS content_type;
ALTER TABLE service_provider_note_executions DROP COLUMN IF EXISTS review_feedback;

CREATE TABLE IF NOT EXISTS service_provider_notes (
    note_id TEXT PRIMARY KEY,
    note_content TEXT NOT NULL
);

DELETE FROM service_provider_note_executions
WHERE note_id IS NULL OR note_id !~ '^[0-9a-fA-F]{24}$';
DELETE FROM service_provider_notes
WHERE note_id !~ '^[0-9a-fA-F]{24}$';

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
        'https://se3u0tsx62.feishu.cn/wiki/MnqJwPARoi94UGkZ9ZicKAR1nHc?sheet=d487ec',
        'MnqJwPARoi94UGkZ9ZicKAR1nHc',
        'd487ec',
        '达人笔记执行表',
        TRUE
	),
	(
		'zhiyuan',
		'智元',
		'https://xcngqzsbenir.feishu.cn/wiki/WdSrwOXtxiG1OlksDFVcutVKnQg?sheet=QbHF0h',
		'WdSrwOXtxiG1OlksDFVcutVKnQg',
		'QbHF0h',
		'koc稿件审核表',
		TRUE
    )
ON CONFLICT (provider_code) DO NOTHING;

UPDATE service_provider_content_tables
SET source_url = 'https://se3u0tsx62.feishu.cn/wiki/MnqJwPARoi94UGkZ9ZicKAR1nHc?sheet=d487ec',
    wiki_token = 'MnqJwPARoi94UGkZ9ZicKAR1nHc', sheet_id = 'd487ec', enabled = TRUE,
    updated_at = NOW()
WHERE provider_code = 'youyiyouer';

UPDATE service_provider_content_tables
SET source_url = 'https://xcngqzsbenir.feishu.cn/wiki/WdSrwOXtxiG1OlksDFVcutVKnQg?sheet=QbHF0h',
	wiki_token = 'WdSrwOXtxiG1OlksDFVcutVKnQg', sheet_id = 'QbHF0h',
	sheet_name = 'koc稿件审核表', enabled = TRUE, updated_at = NOW()
WHERE provider_code = 'zhiyuan';
