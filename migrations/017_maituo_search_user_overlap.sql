CREATE OR REPLACE VIEW maituo_customer_daily_search_user_overlap AS
WITH subaccount_totals AS (
    SELECT report_date, spu,
        SUM(search_users)::BIGINT AS subaccount_search_users
    FROM maituo_customer_daily_subaccounts
    WHERE deleted_at IS NULL
    GROUP BY report_date, spu
),
subaccount_mappings AS (
    SELECT DISTINCT report_date, spu, BTRIM(subaccount) AS subaccount, placement
    FROM maituo_customer_daily_subaccounts
    WHERE deleted_at IS NULL
),
note_rows_by_spu AS (
    SELECT DISTINCT notes.report_date, notes.note_id, notes.subaccount,
        notes.campaign_name, notes.placement, notes.search_users, mappings.spu
    FROM maituo_customer_daily_notes notes
    CROSS JOIN LATERAL regexp_split_to_table(notes.subaccount, '[、,，]+') account_names(account_name)
    JOIN subaccount_mappings mappings
      ON mappings.report_date = notes.report_date
     AND mappings.subaccount = BTRIM(account_names.account_name)
     AND mappings.placement = notes.placement
    WHERE notes.deleted_at IS NULL
),
note_totals AS (
    SELECT report_date, spu,
        SUM(search_users)::BIGINT AS note_search_users
    FROM note_rows_by_spu
    GROUP BY report_date, spu
)
SELECT spus.report_date,
    spus.spu,
    spus.search_users AS spu_search_users,
    totals.subaccount_search_users,
    totals.subaccount_search_users - spus.search_users AS overlap_users,
    CASE WHEN spus.search_users > 0
        THEN totals.subaccount_search_users::NUMERIC / spus.search_users
    END AS overlap_coefficient,
    CASE WHEN totals.subaccount_search_users > 0
        THEN spus.search_users::NUMERIC / totals.subaccount_search_users
    END AS deduplication_factor,
    notes.note_search_users,
    notes.note_search_users - spus.search_users AS note_overlap_users,
    CASE WHEN spus.search_users > 0
        THEN notes.note_search_users::NUMERIC / spus.search_users
    END AS note_overlap_coefficient,
    CASE WHEN notes.note_search_users > 0
        THEN spus.search_users::NUMERIC / notes.note_search_users
    END AS note_deduplication_factor
FROM maituo_customer_daily_spus spus
JOIN subaccount_totals totals USING (report_date, spu)
LEFT JOIN note_totals notes USING (report_date, spu)
WHERE spus.deleted_at IS NULL;

COMMENT ON VIEW maituo_customer_daily_search_user_overlap IS
    'Daily subaccount/SPU and note/SPU search-user overlap against SPU-deduplicated users.';

COMMENT ON COLUMN maituo_customer_daily_search_user_overlap.overlap_coefficient IS
    'Subaccount-attributed search users / SPU-deduplicated search users; 1 means no cross-subaccount overlap.';

COMMENT ON COLUMN maituo_customer_daily_search_user_overlap.deduplication_factor IS
    'SPU-deduplicated search users / subaccount-attributed search users; multiply attributed users by this factor to reconcile to SPU totals.';

COMMENT ON COLUMN maituo_customer_daily_search_user_overlap.note_overlap_coefficient IS
    'Note-attributed search users / SPU-deduplicated search users; multi-account note rows count once per SPU.';

COMMENT ON COLUMN maituo_customer_daily_search_user_overlap.note_deduplication_factor IS
    'SPU-deduplicated search users / note-attributed search users; multiply note users by this factor to reconcile to SPU totals.';
