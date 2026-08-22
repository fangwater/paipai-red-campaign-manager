-- Replace the account/plan-grained note table with the canonical daily
-- note-placement table. The renamed legacy table is the immutable rollback
-- source for the one-time production conversion.
DO $$
DECLARE
    has_subaccount BOOLEAN;
    has_campaign_name BOOLEAN;
    conflict_count BIGINT;
    legacy_row_count BIGINT;
    legacy_active_row_count BIGINT;
    expected_row_count BIGINT;
    actual_row_count BIGINT;
    expected_spend NUMERIC;
    actual_spend NUMERIC;
    expected_search_users NUMERIC;
    actual_search_users NUMERIC;
    primary_key_name TEXT;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'public.maituo_customer_daily_notes'::regclass
          AND attname = 'subaccount' AND NOT attisdropped
    ) INTO has_subaccount;
    SELECT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'public.maituo_customer_daily_notes'::regclass
          AND attname = 'campaign_name' AND NOT attisdropped
    ) INTO has_campaign_name;

    IF has_subaccount IS DISTINCT FROM has_campaign_name THEN
        RAISE EXCEPTION 'Maituo notes schema has only one legacy account/plan column';
    END IF;
    IF NOT has_subaccount THEN
        RETURN;
    END IF;
    IF to_regclass('public.maituo_customer_daily_notes_account_plan_archive') IS NOT NULL THEN
        RAISE EXCEPTION 'Maituo legacy archive already exists while the formal table still has account/plan columns';
    END IF;

    LOCK TABLE public.maituo_customer_daily_notes IN ACCESS EXCLUSIVE MODE;

    EXECUTE $query$
        SELECT COUNT(*)
        FROM (
            SELECT report_date, note_id, placement
            FROM public.maituo_customer_daily_notes
            WHERE deleted_at IS NULL
            GROUP BY report_date, note_id, placement
            HAVING COUNT(*) > 1
               AND (
                    COUNT(DISTINCT note_url) > 1
                 OR COUNT(DISTINCT category) > 1
                 OR COUNT(DISTINCT keyword_category_note) > 1
                 OR (BOOL_OR(keyword_category_note IS NULL) AND BOOL_OR(keyword_category_note IS NOT NULL))
               )
        ) conflicts
    $query$ INTO conflict_count;
    IF conflict_count > 0 THEN
        RAISE EXCEPTION 'Cannot merge % Maituo note-placement groups with conflicting descriptive fields', conflict_count;
    END IF;

    EXECUTE 'SELECT COUNT(*) FROM public.maituo_customer_daily_notes'
        INTO legacy_row_count;
    EXECUTE $query$
        SELECT COUNT(*), COUNT(DISTINCT (report_date, note_id, placement)),
               COALESCE(SUM(spend), 0),
               COALESCE(SUM(search_users), 0)
        FROM public.maituo_customer_daily_notes
        WHERE deleted_at IS NULL
    $query$ INTO legacy_active_row_count, expected_row_count, expected_spend, expected_search_users;

    SELECT conname
    INTO primary_key_name
    FROM pg_constraint
    WHERE conrelid = 'public.maituo_customer_daily_notes'::regclass
      AND contype = 'p';

    ALTER TABLE public.maituo_customer_daily_notes
        RENAME TO maituo_customer_daily_notes_account_plan_archive;

    -- Existing dependent views stay attached to the archived table by OID.
    -- deploy/postgres/paipai_readonly.sql rebinds the public reader to the
    -- canonical table after the application migration succeeds.

    IF primary_key_name IS NOT NULL THEN
        EXECUTE format(
            'ALTER TABLE public.maituo_customer_daily_notes_account_plan_archive RENAME CONSTRAINT %I TO maituo_notes_account_plan_archive_pkey',
            primary_key_name
        );
    END IF;
    IF to_regclass('public.idx_maituo_notes_active') IS NOT NULL THEN
        ALTER INDEX public.idx_maituo_notes_active
            RENAME TO idx_maituo_notes_account_plan_archive_active;
    END IF;
    IF to_regclass('public.idx_maituo_notes_report_date_active') IS NOT NULL THEN
        ALTER INDEX public.idx_maituo_notes_report_date_active
            RENAME TO idx_maituo_notes_account_plan_archive_date;
    END IF;

    CREATE TABLE public.maituo_customer_daily_notes (
        report_date DATE NOT NULL,
        note_id TEXT NOT NULL,
        note_url TEXT NOT NULL,
        category TEXT NOT NULL,
        placement TEXT NOT NULL,
        keyword_category_note TEXT,
        spend NUMERIC(20, 6) NOT NULL,
        search_users BIGINT NOT NULL,
        search_cost NUMERIC(20, 6),
        estimated_postback_cost NUMERIC(20, 6),
        search_rate_pct NUMERIC(20, 6),
        cpc NUMERIC(20, 6) NOT NULL,
        ctr_pct NUMERIC(20, 6) NOT NULL,
        source_row_number INTEGER NOT NULL,
        content_hash TEXT NOT NULL,
        import_run_id BIGINT NOT NULL REFERENCES public.maituo_customer_daily_import_runs(id),
        first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        deleted_at TIMESTAMPTZ,
        PRIMARY KEY (report_date, note_id, placement)
    );

    WITH raw_components AS (
        SELECT archive.*,
               ROUND(archive.spend / NULLIF(archive.cpc, 0)) AS implied_clicks
        FROM public.maituo_customer_daily_notes_account_plan_archive archive
        WHERE archive.deleted_at IS NULL
    ), components AS (
        SELECT raw.*,
               ROUND(raw.implied_clicks * 100 / NULLIF(raw.ctr_pct, 0)) AS implied_impressions
        FROM raw_components raw
    ), rolled AS (
        SELECT report_date, note_id, placement,
               COUNT(*)::INTEGER AS component_count,
               MIN(note_url) AS note_url,
               MIN(category) AS category,
               MIN(keyword_category_note) AS keyword_category_note,
               SUM(spend) AS total_spend,
               SUM(search_users)::BIGINT AS total_search_users,
               SUM(implied_clicks) AS total_clicks,
               SUM(implied_impressions) AS total_impressions,
               MIN(search_cost) AS single_search_cost,
               MIN(search_rate_pct) AS single_search_rate_pct,
               MIN(cpc) AS single_cpc,
               MIN(ctr_pct) AS single_ctr_pct,
               MIN(source_row_number) AS source_row_number,
               MIN(content_hash) AS single_content_hash,
               MAX(import_run_id) AS import_run_id,
               MIN(first_seen_at) AS first_seen_at,
               MAX(updated_at) AS updated_at
        FROM components
        GROUP BY report_date, note_id, placement
    ), canonical AS (
        SELECT rolled.*,
               CASE WHEN component_count = 1 THEN single_search_cost
                    WHEN total_search_users > 0
                    THEN ROUND(total_spend / total_search_users, 2)
               END AS canonical_search_cost,
               CASE WHEN component_count = 1 AND single_search_cost IS NOT NULL
                    THEN ROUND(ROUND(single_search_cost, 2) * 0.63, 2)
                    WHEN component_count > 1 AND total_search_users > 0
                    THEN ROUND(ROUND(total_spend / total_search_users, 2) * 0.63, 2)
               END AS canonical_estimated_postback_cost,
               CASE WHEN component_count = 1 THEN single_search_rate_pct
                    WHEN total_search_users > 0 AND total_clicks > 0
                    THEN ROUND(total_search_users * 100 / total_clicks, 4)
               END AS canonical_search_rate_pct,
               CASE WHEN component_count = 1 THEN single_cpc
                    WHEN total_clicks > 0 THEN ROUND(total_spend / total_clicks, 4)
                    ELSE 0
               END AS canonical_cpc,
               CASE WHEN component_count = 1 THEN single_ctr_pct
                    WHEN total_impressions > 0 THEN ROUND(total_clicks * 100 / total_impressions, 4)
                    ELSE 0
               END AS canonical_ctr_pct
        FROM rolled
    )
    INSERT INTO public.maituo_customer_daily_notes (
        report_date, note_id, note_url, category, placement, keyword_category_note,
        spend, search_users, search_cost, estimated_postback_cost, search_rate_pct,
        cpc, ctr_pct, source_row_number, content_hash, import_run_id,
        first_seen_at, updated_at, deleted_at
    )
    SELECT report_date, note_id, note_url, category, placement, keyword_category_note,
           total_spend, total_search_users, canonical_search_cost,
           canonical_estimated_postback_cost, canonical_search_rate_pct,
           canonical_cpc, canonical_ctr_pct, source_row_number,
           CASE WHEN component_count = 1 THEN single_content_hash
                ELSE MD5(JSONB_BUILD_ARRAY(
                    report_date, note_id, note_url, category, placement,
                    keyword_category_note, total_spend, total_search_users,
                    canonical_search_cost, canonical_estimated_postback_cost,
                    canonical_search_rate_pct, canonical_cpc, canonical_ctr_pct
                )::TEXT)
           END,
           import_run_id, first_seen_at, updated_at, NULL
    FROM canonical;

    SELECT COUNT(*), COALESCE(SUM(spend), 0), COALESCE(SUM(search_users), 0)
    INTO actual_row_count, actual_spend, actual_search_users
    FROM public.maituo_customer_daily_notes
    WHERE deleted_at IS NULL;

    IF actual_row_count <> expected_row_count
       OR actual_spend IS DISTINCT FROM expected_spend
       OR actual_search_users IS DISTINCT FROM expected_search_users THEN
        RAISE EXCEPTION
            'Maituo note migration invariant failed: rows %/%, spend %/%, users %/%',
            actual_row_count, expected_row_count,
            actual_spend, expected_spend,
            actual_search_users, expected_search_users;
    END IF;
    IF (SELECT COUNT(*) FROM public.maituo_customer_daily_notes_account_plan_archive) <> legacy_row_count THEN
        RAISE EXCEPTION 'Maituo legacy archive row count changed during table replacement';
    END IF;
    IF legacy_active_row_count < expected_row_count THEN
        RAISE EXCEPTION 'Maituo canonical row count exceeds the active legacy row count';
    END IF;

    COMMENT ON TABLE public.maituo_customer_daily_notes_account_plan_archive IS
        'One-time immutable archive of account/plan-grained Maituo note rows before placement-level rollback.';
END
$$;

DO $$
DECLARE
    primary_key_definition TEXT;
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'public.maituo_customer_daily_notes'::regclass
          AND attname IN ('subaccount', 'campaign_name')
          AND NOT attisdropped
    ) THEN
        RAISE EXCEPTION 'Maituo formal notes table still contains retired account/plan columns';
    END IF;

    SELECT pg_get_constraintdef(oid)
    INTO primary_key_definition
    FROM pg_constraint
    WHERE conrelid = 'public.maituo_customer_daily_notes'::regclass
      AND contype = 'p';
    IF primary_key_definition IS DISTINCT FROM 'PRIMARY KEY (report_date, note_id, placement)' THEN
        RAISE EXCEPTION 'Unexpected Maituo notes primary key: %', primary_key_definition;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_maituo_notes_active
    ON public.maituo_customer_daily_notes (note_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_maituo_notes_report_date_active
    ON public.maituo_customer_daily_notes (report_date DESC, note_id)
    WHERE deleted_at IS NULL;
