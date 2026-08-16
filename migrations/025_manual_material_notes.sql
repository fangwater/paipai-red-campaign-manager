ALTER TABLE manual_materials
    ADD COLUMN IF NOT EXISTS note_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS note_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS note_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cover_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS commercial_intensity TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS audience TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS user_scenario TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'manual_materials_note_id_format'
    ) THEN
        ALTER TABLE manual_materials
            ADD CONSTRAINT manual_materials_note_id_format
            CHECK (note_id = '' OR note_id ~ '^[0-9a-f]{24}$');
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'manual_materials_note_url_length'
    ) THEN
        ALTER TABLE manual_materials
            ADD CONSTRAINT manual_materials_note_url_length
            CHECK (CHAR_LENGTH(note_url) <= 500);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'manual_materials_note_type_length'
    ) THEN
        ALTER TABLE manual_materials
            ADD CONSTRAINT manual_materials_note_type_length
            CHECK (CHAR_LENGTH(note_type) <= 100);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'manual_materials_cover_type_length'
    ) THEN
        ALTER TABLE manual_materials
            ADD CONSTRAINT manual_materials_cover_type_length
            CHECK (CHAR_LENGTH(cover_type) <= 100);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'manual_materials_commercial_intensity_length'
    ) THEN
        ALTER TABLE manual_materials
            ADD CONSTRAINT manual_materials_commercial_intensity_length
            CHECK (CHAR_LENGTH(commercial_intensity) <= 100);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'manual_materials_audience_length'
    ) THEN
        ALTER TABLE manual_materials
            ADD CONSTRAINT manual_materials_audience_length
            CHECK (CHAR_LENGTH(audience) <= 100);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'manual_materials_user_scenario_length'
    ) THEN
        ALTER TABLE manual_materials
            ADD CONSTRAINT manual_materials_user_scenario_length
            CHECK (CHAR_LENGTH(user_scenario) <= 100);
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_manual_materials_note_id
    ON manual_materials (note_id)
    WHERE note_id <> '';

COMMENT ON COLUMN manual_materials.note_id IS
    'Xiaohongshu note ID associated with this manually composed material.';
COMMENT ON COLUMN manual_materials.note_url IS
    'Public Xiaohongshu note URL associated with this material.';
