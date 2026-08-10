ALTER TABLE service_provider_notes
    ADD COLUMN IF NOT EXISTS source_title TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN service_provider_notes.source_title IS
    'Title of the Feishu manuscript document used as the note source.';

UPDATE service_provider_notes
SET extractor_version = 4
WHERE extractor_version = 3;
