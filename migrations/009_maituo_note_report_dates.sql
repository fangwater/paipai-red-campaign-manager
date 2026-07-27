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
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'maituo_customer_daily_notes'::regclass
          AND contype = 'p'
          AND pg_get_constraintdef(oid) NOT LIKE 'PRIMARY KEY (report_date,%'
    ) THEN
        ALTER TABLE maituo_customer_daily_notes DROP CONSTRAINT maituo_customer_daily_notes_pkey;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'maituo_customer_daily_notes'::regclass
          AND contype = 'p'
    ) THEN
        ALTER TABLE maituo_customer_daily_notes
            ADD PRIMARY KEY (report_date, note_id, subaccount, campaign_name, placement);
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_maituo_import_runs_report_date
    ON maituo_customer_daily_import_runs (report_date DESC, completed_at DESC)
    WHERE status = 'succeeded';

CREATE INDEX IF NOT EXISTS idx_maituo_notes_report_date_active
    ON maituo_customer_daily_notes (report_date DESC, note_id)
    WHERE deleted_at IS NULL;
