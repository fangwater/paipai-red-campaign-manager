CREATE TABLE IF NOT EXISTS manual_materials (
    material_id TEXT PRIMARY KEY
        CHECK (material_id ~ '^[0-9a-f]{32}$'),
    title TEXT NOT NULL
        CHECK (BTRIM(title) <> '' AND CHAR_LENGTH(title) <= 200),
    body TEXT NOT NULL
        CHECK (BTRIM(body) <> '' AND CHAR_LENGTH(body) <= 20000),
    comments TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT manual_materials_comments_limit CHECK (CARDINALITY(comments) <= 20)
);

COMMENT ON TABLE manual_materials IS
    'Manually composed Xiaohongshu materials with structured title, body, comments, and images.';
COMMENT ON COLUMN manual_materials.comments IS
    'Ordered seed comments to publish under the note.';

CREATE TABLE IF NOT EXISTS manual_material_assets (
    material_id TEXT NOT NULL REFERENCES manual_materials(material_id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    asset_id TEXT NOT NULL REFERENCES manuscript_assets(asset_id),
    width INTEGER NOT NULL DEFAULT 0 CHECK (width >= 0),
    height INTEGER NOT NULL DEFAULT 0 CHECK (height >= 0),
    PRIMARY KEY (material_id, position)
);

CREATE INDEX IF NOT EXISTS idx_manual_material_assets_asset
    ON manual_material_assets (asset_id);
