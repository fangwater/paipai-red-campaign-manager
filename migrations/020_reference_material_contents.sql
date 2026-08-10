CREATE TABLE IF NOT EXISTS reference_material_contents (
    reference_note_id TEXT PRIMARY KEY
        CHECK (reference_note_id ~ '^[0-9a-f]{24}$'),
    note_content TEXT NOT NULL
        CHECK (BTRIM(note_content) <> '' AND CHAR_LENGTH(note_content) <= 20000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE reference_material_contents IS
    'Manually maintained content for note IDs referenced by provider manuscripts.';
COMMENT ON COLUMN reference_material_contents.note_content IS
    'Plain-text reference material content; manual content takes precedence over synchronized manuscripts.';
