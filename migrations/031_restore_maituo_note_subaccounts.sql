-- Restore the account and plan dimensions required to split daily note details.
-- Migration 026 retained the original account/plan rows in its immutable archive,
-- allowing historical reports to regain the exact source-level rows.
DO $$
DECLARE
    has_subaccount BOOLEAN;
    has_campaign_name BOOLEAN;
    primary_key_name TEXT;
    primary_key_definition TEXT;
BEGIN
    IF to_regclass('public.maituo_customer_daily_notes_account_plan_archive') IS NULL THEN
        RETURN;
    END IF;

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
        RAISE EXCEPTION 'Maituo notes schema has only one account/plan column';
    END IF;
    IF has_subaccount THEN
        SELECT pg_get_constraintdef(oid)
        INTO primary_key_definition
        FROM pg_constraint
        WHERE conrelid = 'public.maituo_customer_daily_notes'::regclass
          AND contype = 'p';
        IF primary_key_definition IS DISTINCT FROM 'PRIMARY KEY (report_date, note_id, subaccount, campaign_name, placement)' THEN
            RAISE EXCEPTION 'Unexpected restored Maituo notes primary key: %', primary_key_definition;
        END IF;
        RETURN;
    END IF;

    LOCK TABLE public.maituo_customer_daily_notes IN ACCESS EXCLUSIVE MODE;

    SELECT conname
    INTO primary_key_name
    FROM pg_constraint
    WHERE conrelid = 'public.maituo_customer_daily_notes'::regclass
      AND contype = 'p';

    ALTER TABLE public.maituo_customer_daily_notes
        ADD COLUMN subaccount TEXT NOT NULL DEFAULT '',
        ADD COLUMN campaign_name TEXT NOT NULL DEFAULT '';
    EXECUTE format('ALTER TABLE public.maituo_customer_daily_notes DROP CONSTRAINT %I', primary_key_name);
    ALTER TABLE public.maituo_customer_daily_notes
        ADD PRIMARY KEY (report_date, note_id, subaccount, campaign_name, placement);

    -- Rows written after the collapse have no attributable account. Replace only
    -- dates/notes that have archived source rows, leaving unrelated new records
    -- explicitly unassigned rather than assigning them to an account by inference.
    DELETE FROM public.maituo_customer_daily_notes notes
    WHERE EXISTS (
        SELECT 1
        FROM public.maituo_customer_daily_notes_account_plan_archive archive
        WHERE archive.deleted_at IS NULL
          AND archive.report_date = notes.report_date
          AND archive.note_id = notes.note_id
          AND archive.placement = notes.placement
    );

    INSERT INTO public.maituo_customer_daily_notes (
        report_date, note_id, note_url, category, subaccount, campaign_name, placement,
        keyword_category_note, spend, search_users, search_cost, estimated_postback_cost,
        search_rate_pct, cpc, ctr_pct, source_row_number, content_hash, import_run_id,
        first_seen_at, updated_at, deleted_at
    )
    SELECT archive.report_date, archive.note_id, archive.note_url, archive.category,
           archive.subaccount, archive.campaign_name, archive.placement,
           archive.keyword_category_note, archive.spend, archive.search_users,
           archive.search_cost,
           ROUND(ROUND(archive.search_cost, 2) * 0.63::NUMERIC, 2),
           archive.search_rate_pct, archive.cpc, archive.ctr_pct,
           archive.source_row_number, archive.content_hash, archive.import_run_id,
           archive.first_seen_at, archive.updated_at, NULL
    FROM public.maituo_customer_daily_notes_account_plan_archive archive
    WHERE archive.deleted_at IS NULL;

    ALTER TABLE public.maituo_customer_daily_notes
        ALTER COLUMN subaccount DROP DEFAULT,
        ALTER COLUMN campaign_name DROP DEFAULT;
END
$$;
