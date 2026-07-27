ALTER TABLE maituo_customer_daily_kpis
    ADD COLUMN IF NOT EXISTS report_date DATE;

UPDATE maituo_customer_daily_kpis target
SET report_date = runs.report_date
FROM maituo_customer_daily_import_runs runs
WHERE target.import_run_id = runs.id
  AND target.report_date IS NULL;

ALTER TABLE maituo_customer_daily_kpis
    ALTER COLUMN report_date SET NOT NULL;

ALTER TABLE maituo_customer_daily_spus
    ADD COLUMN IF NOT EXISTS report_date DATE;

UPDATE maituo_customer_daily_spus target
SET report_date = runs.report_date
FROM maituo_customer_daily_import_runs runs
WHERE target.import_run_id = runs.id
  AND target.report_date IS NULL;

ALTER TABLE maituo_customer_daily_spus
    ALTER COLUMN report_date SET NOT NULL;

ALTER TABLE maituo_customer_daily_subaccounts
    ADD COLUMN IF NOT EXISTS report_date DATE;

UPDATE maituo_customer_daily_subaccounts target
SET report_date = runs.report_date
FROM maituo_customer_daily_import_runs runs
WHERE target.import_run_id = runs.id
  AND target.report_date IS NULL;

ALTER TABLE maituo_customer_daily_subaccounts
    ALTER COLUMN report_date SET NOT NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'maituo_customer_daily_kpis'::regclass
          AND contype = 'p'
          AND pg_get_constraintdef(oid) NOT LIKE 'PRIMARY KEY (report_date,%'
    ) THEN
        ALTER TABLE maituo_customer_daily_kpis DROP CONSTRAINT maituo_customer_daily_kpis_pkey;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'maituo_customer_daily_kpis'::regclass AND contype = 'p'
    ) THEN
        ALTER TABLE maituo_customer_daily_kpis ADD PRIMARY KEY (report_date, metric);
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'maituo_customer_daily_spus'::regclass
          AND contype = 'p'
          AND pg_get_constraintdef(oid) NOT LIKE 'PRIMARY KEY (report_date,%'
    ) THEN
        ALTER TABLE maituo_customer_daily_spus DROP CONSTRAINT maituo_customer_daily_spus_pkey;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'maituo_customer_daily_spus'::regclass AND contype = 'p'
    ) THEN
        ALTER TABLE maituo_customer_daily_spus ADD PRIMARY KEY (report_date, spu);
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'maituo_customer_daily_subaccounts'::regclass
          AND contype = 'p'
          AND pg_get_constraintdef(oid) NOT LIKE 'PRIMARY KEY (report_date,%'
    ) THEN
        ALTER TABLE maituo_customer_daily_subaccounts DROP CONSTRAINT maituo_customer_daily_subaccounts_pkey;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'maituo_customer_daily_subaccounts'::regclass AND contype = 'p'
    ) THEN
        ALTER TABLE maituo_customer_daily_subaccounts
            ADD PRIMARY KEY (report_date, spu, subaccount, placement);
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_maituo_kpis_report_date_active
    ON maituo_customer_daily_kpis (report_date DESC, metric)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_maituo_spus_report_date_active
    ON maituo_customer_daily_spus (report_date DESC, spu)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_maituo_subaccounts_report_date_active
    ON maituo_customer_daily_subaccounts (report_date DESC, spu, subaccount)
    WHERE deleted_at IS NULL;
