ALTER TABLE maituo_customer_daily_import_runs
    ADD COLUMN IF NOT EXISTS present_sheets TEXT[];

UPDATE maituo_customer_daily_import_runs
SET present_sheets = ARRAY['总览KPI', '笔记明细', '分SPU总览', '分子账户', '淘搜趋势']::TEXT[]
WHERE present_sheets IS NULL;

ALTER TABLE maituo_customer_daily_import_runs
    ALTER COLUMN present_sheets SET NOT NULL,
    ALTER COLUMN present_sheets SET DEFAULT ARRAY['总览KPI', '笔记明细', '分SPU总览', '分子账户', '淘搜趋势']::TEXT[];
