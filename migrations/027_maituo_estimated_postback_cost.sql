DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM public.maituo_customer_daily_notes
        WHERE search_cost IS NULL
          AND estimated_postback_cost IS NOT NULL
    ) OR EXISTS (
        SELECT 1
        FROM public.maituo_customer_daily_subaccounts
        WHERE search_cost IS NULL
          AND estimated_postback_cost IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'Cannot derive Maituo estimated postback cost without search cost';
    END IF;
END
$$;

UPDATE public.maituo_customer_daily_notes
SET estimated_postback_cost = ROUND(ROUND(search_cost, 2) * 0.63::NUMERIC, 2)
WHERE search_cost IS NOT NULL
  AND estimated_postback_cost IS DISTINCT FROM ROUND(ROUND(search_cost, 2) * 0.63::NUMERIC, 2);

UPDATE public.maituo_customer_daily_subaccounts
SET estimated_postback_cost = ROUND(ROUND(search_cost, 2) * 0.63::NUMERIC, 2)
WHERE search_cost IS NOT NULL
  AND estimated_postback_cost IS DISTINCT FROM ROUND(ROUND(search_cost, 2) * 0.63::NUMERIC, 2);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'public.maituo_customer_daily_notes'::REGCLASS
          AND conname = 'maituo_notes_estimated_postback_cost_formula'
          AND convalidated
          AND pg_get_constraintdef(oid) =
              'CHECK ((NOT (estimated_postback_cost IS DISTINCT FROM round((round(search_cost, 2) * 0.63), 2))))'
    ) THEN
        ALTER TABLE public.maituo_customer_daily_notes
            DROP CONSTRAINT IF EXISTS maituo_notes_estimated_postback_cost_formula;
        ALTER TABLE public.maituo_customer_daily_notes
            ADD CONSTRAINT maituo_notes_estimated_postback_cost_formula
            CHECK (
                estimated_postback_cost IS NOT DISTINCT FROM
                ROUND(ROUND(search_cost, 2) * 0.63::NUMERIC, 2)
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'public.maituo_customer_daily_subaccounts'::REGCLASS
          AND conname = 'maituo_subaccounts_estimated_postback_cost_formula'
          AND convalidated
          AND pg_get_constraintdef(oid) =
              'CHECK ((NOT (estimated_postback_cost IS DISTINCT FROM round((round(search_cost, 2) * 0.63), 2))))'
    ) THEN
        ALTER TABLE public.maituo_customer_daily_subaccounts
            DROP CONSTRAINT IF EXISTS maituo_subaccounts_estimated_postback_cost_formula;
        ALTER TABLE public.maituo_customer_daily_subaccounts
            ADD CONSTRAINT maituo_subaccounts_estimated_postback_cost_formula
            CHECK (
                estimated_postback_cost IS NOT DISTINCT FROM
                ROUND(ROUND(search_cost, 2) * 0.63::NUMERIC, 2)
            );
    END IF;
END
$$;
