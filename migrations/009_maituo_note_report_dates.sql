ALTER TABLE maituo_customer_daily_import_runs
    ADD COLUMN IF NOT EXISTS report_date DATE;

UPDATE maituo_customer_daily_import_runs
SET report_date = SUBSTRING(file_name FROM '([0-9]{4}-[0-9]{2}-[0-9]{2})')::DATE
WHERE report_date IS NULL
  AND file_name ~ '[0-9]{4}-[0-9]{2}-[0-9]{2}';

ALTER TABLE maituo_customer_daily_import_runs
    ALTER COLUMN report_date SET NOT NULL;

ALTER TABLE maituo_customer_daily_notes
    ADD COLUMN IF NOT EXISTS report_date DATE;

UPDATE maituo_customer_daily_notes notes
SET report_date = runs.report_date
FROM maituo_customer_daily_import_runs runs
WHERE notes.import_run_id = runs.id
  AND notes.report_date IS NULL;

ALTER TABLE maituo_customer_daily_notes
    ALTER COLUMN report_date SET NOT NULL;

DO $$
DECLARE
    has_subaccount BOOLEAN;
    has_campaign_name BOOLEAN;
    expected_primary_key TEXT;
    current_primary_key_name TEXT;
    current_primary_key TEXT;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'maituo_customer_daily_notes'::regclass
          AND attname = 'subaccount' AND NOT attisdropped
    ) INTO has_subaccount;
    SELECT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'maituo_customer_daily_notes'::regclass
          AND attname = 'campaign_name' AND NOT attisdropped
    ) INTO has_campaign_name;

    IF has_subaccount IS DISTINCT FROM has_campaign_name THEN
        RAISE EXCEPTION 'Maituo notes schema has only one legacy account/plan column';
    END IF;

    expected_primary_key := CASE WHEN has_subaccount
        THEN 'PRIMARY KEY (report_date, note_id, subaccount, campaign_name, placement)'
        ELSE 'PRIMARY KEY (report_date, note_id, placement)'
    END;

    SELECT conname, pg_get_constraintdef(oid)
    INTO current_primary_key_name, current_primary_key
    FROM pg_constraint
    WHERE conrelid = 'maituo_customer_daily_notes'::regclass
      AND contype = 'p';

    IF current_primary_key IS NOT NULL AND current_primary_key <> expected_primary_key THEN
        EXECUTE format(
            'ALTER TABLE maituo_customer_daily_notes DROP CONSTRAINT %I',
            current_primary_key_name
        );
    END IF;

    IF current_primary_key IS NULL OR current_primary_key <> expected_primary_key THEN
        IF has_subaccount THEN
            ALTER TABLE maituo_customer_daily_notes
                ADD PRIMARY KEY (report_date, note_id, subaccount, campaign_name, placement);
        ELSE
            ALTER TABLE maituo_customer_daily_notes
                ADD PRIMARY KEY (report_date, note_id, placement);
        END IF;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_maituo_import_runs_report_date
    ON maituo_customer_daily_import_runs (report_date DESC, completed_at DESC)
    WHERE status = 'succeeded';

CREATE INDEX IF NOT EXISTS idx_maituo_notes_report_date_active
    ON maituo_customer_daily_notes (report_date DESC, note_id)
    WHERE deleted_at IS NULL;
