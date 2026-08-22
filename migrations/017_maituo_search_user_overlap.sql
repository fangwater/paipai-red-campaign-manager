CREATE OR REPLACE VIEW maituo_customer_daily_search_user_overlap AS
WITH subaccount_totals AS (
    SELECT report_date, spu,
        SUM(search_users)::BIGINT AS subaccount_search_users
    FROM maituo_customer_daily_subaccounts
    WHERE deleted_at IS NULL
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
    NULL::BIGINT AS note_search_users,
    NULL::BIGINT AS note_overlap_users,
    NULL::NUMERIC AS note_overlap_coefficient,
    NULL::NUMERIC AS note_deduplication_factor
FROM maituo_customer_daily_spus spus
JOIN subaccount_totals totals USING (report_date, spu)
WHERE spus.deleted_at IS NULL;

COMMENT ON VIEW maituo_customer_daily_search_user_overlap IS
    'Daily subaccount/SPU search-user overlap; retired note-attribution columns are NULL.';

COMMENT ON COLUMN maituo_customer_daily_search_user_overlap.overlap_coefficient IS
    'Subaccount-attributed search users / SPU-deduplicated search users; 1 means no cross-subaccount overlap.';

COMMENT ON COLUMN maituo_customer_daily_search_user_overlap.deduplication_factor IS
    'SPU-deduplicated search users / subaccount-attributed search users; multiply attributed users by this factor to reconcile to SPU totals.';

COMMENT ON COLUMN maituo_customer_daily_search_user_overlap.note_overlap_coefficient IS
    'Retired with account/plan attribution; always NULL.';

COMMENT ON COLUMN maituo_customer_daily_search_user_overlap.note_deduplication_factor IS
    'Retired with account/plan attribution; always NULL.';
