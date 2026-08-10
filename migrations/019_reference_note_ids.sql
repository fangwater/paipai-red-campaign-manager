ALTER TABLE service_provider_notes
    ADD COLUMN IF NOT EXISTS reference_note_ids TEXT[];

UPDATE service_provider_notes AS notes
SET reference_note_ids = COALESCE((
    SELECT ARRAY_AGG(reference_id ORDER BY first_position)
    FROM (
        SELECT LOWER(matches[1]) AS reference_id, MIN(position) AS first_position
        FROM regexp_matches(
            COALESCE(notes.note_content, ''),
            '(?i)xiaohongshu[.]com/(?:explore/|discovery/item/)([0-9a-f]{24})(?:[^0-9a-f]|$)',
            'g'
        ) WITH ORDINALITY AS matched(matches, position)
        WHERE LOWER(matches[1]) <> LOWER(notes.note_id)
        GROUP BY LOWER(matches[1])
    ) AS recognized
), ARRAY[]::TEXT[])
WHERE reference_note_ids IS NULL;

ALTER TABLE service_provider_notes
    ALTER COLUMN reference_note_ids SET DEFAULT ARRAY[]::TEXT[],
    ALTER COLUMN reference_note_ids SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_service_provider_notes_reference_note_ids
    ON service_provider_notes USING GIN (reference_note_ids);

COMMENT ON COLUMN service_provider_notes.reference_note_ids IS
    'Ordered unique Xiaohongshu note IDs recognized from valid detail links in the final manuscript, excluding the manuscript note ID.';
