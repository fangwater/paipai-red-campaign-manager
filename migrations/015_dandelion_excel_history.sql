CREATE TABLE IF NOT EXISTS dandelion_excel_import_runs (
    run_id BIGINT PRIMARY KEY REFERENCES sync_runs(id) ON DELETE CASCADE,
    file_name TEXT NOT NULL,
    file_sha256 TEXT NOT NULL,
    report_date DATE NOT NULL,
    sheet_name TEXT NOT NULL,
    header_row INTEGER NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    fetched_count INTEGER NOT NULL DEFAULT 0,
    inserted_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0,
    unchanged_count INTEGER NOT NULL DEFAULT 0,
    deleted_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_dandelion_excel_import_runs_report_date
    ON dandelion_excel_import_runs (report_date DESC, completed_at DESC);

WITH latest_run AS (
    SELECT id, fetched_count, upserted_count, started_at, completed_at
    FROM sync_runs
    WHERE app_token = 'excel:dandelion'
      AND table_id = 'dandelion_excel_upload'
      AND status = 'succeeded'
    ORDER BY COALESCE(completed_at, started_at) DESC, id DESC
    LIMIT 1
),
latest_data AS (
    SELECT MAX(
        (
            TO_TIMESTAMP((fields ->> '数据更新日期')::DOUBLE PRECISION / 1000)
            AT TIME ZONE 'Asia/Shanghai'
        )::DATE
    ) AS report_date
    FROM lark_bitable_records
    WHERE app_token = 'excel:dandelion'
      AND table_id = 'dandelion_excel_upload'
      AND deleted_at IS NULL
      AND JSONB_TYPEOF(fields -> '数据更新日期') = 'number'
)
INSERT INTO dandelion_excel_import_runs (
    run_id, file_name, file_sha256, report_date, sheet_name, header_row, status,
    fetched_count, inserted_count, updated_count, unchanged_count, started_at, completed_at
)
SELECT
    latest_run.id,
    '历史蒲公英上传-' || TO_CHAR(latest_data.report_date, 'YYYY-MM-DD') || '.xlsx',
    'legacy-run-' || latest_run.id,
    latest_data.report_date, '蒲公英数据', 0, 'succeeded',
    latest_run.fetched_count, 0, latest_run.upserted_count,
    GREATEST(latest_run.fetched_count - latest_run.upserted_count, 0),
    latest_run.started_at, COALESCE(latest_run.completed_at, latest_run.started_at)
FROM latest_run CROSS JOIN latest_data
WHERE latest_data.report_date IS NOT NULL
ON CONFLICT (run_id) DO NOTHING;
