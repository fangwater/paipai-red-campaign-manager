CREATE TABLE IF NOT EXISTS manuscript_assets (
    asset_id TEXT PRIMARY KEY
        CHECK (asset_id ~ '^[0-9a-f]{64}$'),
    content_type TEXT NOT NULL
        CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp', 'image/gif')),
    byte_size BIGINT NOT NULL CHECK (byte_size > 0 AND byte_size <= 10485760),
    width INTEGER NOT NULL DEFAULT 0 CHECK (width >= 0),
    height INTEGER NOT NULL DEFAULT 0 CHECK (height >= 0),
    content BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (octet_length(content) = byte_size)
);

ALTER TABLE service_provider_notes
    ADD COLUMN IF NOT EXISTS content_blocks JSONB NOT NULL DEFAULT '[]'::JSONB,
    ADD COLUMN IF NOT EXISTS source_url TEXT,
    ADD COLUMN IF NOT EXISTS source_resource_key TEXT,
    ADD COLUMN IF NOT EXISTS source_revision INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS extractor_version INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'service_provider_notes_content_blocks_array'
    ) THEN
        ALTER TABLE service_provider_notes
            ADD CONSTRAINT service_provider_notes_content_blocks_array
            CHECK (jsonb_typeof(content_blocks) = 'array');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS service_provider_note_assets (
    note_id TEXT NOT NULL REFERENCES service_provider_notes(note_id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    asset_id TEXT NOT NULL REFERENCES manuscript_assets(asset_id),
    width INTEGER NOT NULL DEFAULT 0 CHECK (width >= 0),
    height INTEGER NOT NULL DEFAULT 0 CHECK (height >= 0),
    caption TEXT,
    PRIMARY KEY (note_id, position)
);

CREATE INDEX IF NOT EXISTS idx_service_provider_note_assets_asset
    ON service_provider_note_assets (asset_id);
