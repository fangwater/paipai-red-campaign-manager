BEGIN;

DO $$
DECLARE
    primary_key_definition TEXT;
BEGIN
    IF to_regclass('public.maituo_customer_daily_notes') IS NULL THEN
        RAISE EXCEPTION 'Canonical Maituo notes table does not exist';
    END IF;
    IF (SELECT COUNT(*)
        FROM pg_attribute
        WHERE attrelid = 'public.maituo_customer_daily_notes'::regclass
          AND attname IN ('subaccount', 'campaign_name')
          AND NOT attisdropped) <> 2 THEN
        RAISE EXCEPTION 'Maituo notes table lacks account/plan dimensions';
    END IF;
    SELECT pg_get_constraintdef(oid)
    INTO primary_key_definition
    FROM pg_constraint
    WHERE conrelid = 'public.maituo_customer_daily_notes'::regclass
      AND contype = 'p';
    IF primary_key_definition IS DISTINCT FROM 'PRIMARY KEY (report_date, note_id, subaccount, campaign_name, placement)' THEN
        RAISE EXCEPTION 'Unexpected Maituo notes primary key: %', primary_key_definition;
    END IF;
END
$$;

CREATE SCHEMA IF NOT EXISTS paipai_readonly;
REVOKE ALL ON SCHEMA paipai_readonly FROM PUBLIC;

CREATE OR REPLACE VIEW paipai_readonly.lark_bitable_tables AS
SELECT table_id, name, revision, synced_at
FROM public.lark_bitable_tables
WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW paipai_readonly.lark_bitable_records AS
SELECT table_id, record_id, fields, lark_created_at, lark_updated_at, synced_at
FROM public.lark_bitable_records
WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW paipai_readonly.lark_linked_documents AS
SELECT provider, resource_key, source_url, document_type, title, content, revision_id,
       fetch_status, error_message, fetched_at, last_seen_at
FROM public.lark_linked_documents;

CREATE OR REPLACE VIEW paipai_readonly.maituo_customer_daily_kpis AS
SELECT report_date, metric, metric_value, data_basis, first_seen_at, updated_at
FROM public.maituo_customer_daily_kpis
WHERE deleted_at IS NULL;

DROP VIEW IF EXISTS paipai_readonly.maituo_customer_daily_notes;
CREATE VIEW paipai_readonly.maituo_customer_daily_notes AS
SELECT report_date, note_id, note_url, category, subaccount, campaign_name, placement,
       keyword_category_note, spend, search_users, search_cost, estimated_postback_cost,
       search_rate_pct, cpc, ctr_pct, first_seen_at, updated_at
FROM public.maituo_customer_daily_notes
WHERE deleted_at IS NULL;

DO $$
BEGIN
    IF (SELECT COUNT(*)
        FROM information_schema.columns
        WHERE table_schema = 'paipai_readonly'
          AND table_name = 'maituo_customer_daily_notes'
          AND column_name IN ('subaccount', 'campaign_name')) <> 2 OR POSITION(
        'account_plan_archive' IN
        pg_get_viewdef('paipai_readonly.maituo_customer_daily_notes'::regclass, TRUE)
    ) > 0 THEN
        RAISE EXCEPTION 'Maituo readonly view is not bound to the canonical table';
    END IF;
END
$$;

CREATE OR REPLACE VIEW paipai_readonly.maituo_customer_daily_spus AS
SELECT report_date, spu, auction_spend, search_users, search_cost, search_rate_pct,
       cpc, ctr_pct, note_count, first_seen_at, updated_at
FROM public.maituo_customer_daily_spus
WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW paipai_readonly.maituo_customer_daily_subaccounts AS
SELECT report_date, spu, subaccount, placement, search_cost, estimated_postback_cost,
       spend, search_users, search_rate_pct, cpc, ctr_pct, note_count,
       first_seen_at, updated_at
FROM public.maituo_customer_daily_subaccounts
WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW paipai_readonly.maituo_customer_daily_trends AS
SELECT report_date, coenzyme_spend, coenzyme_search_uv, coenzyme_order_uv,
       coenzyme_search_cost, krill_oil_spend, krill_oil_search_uv, krill_oil_order_uv,
       krill_oil_search_cost, total_search_uv, total_order_uv, total_search_cost,
       total_spend, total_recall_search_cost, first_seen_at, updated_at
FROM public.maituo_customer_daily_trends
WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW paipai_readonly.service_provider_note_executions AS
SELECT provider_code, record_key, source_row_number, submission_date, note_id,
       cover_type, commercial_intensity, audience, user_scenario, note_type,
       progress, synced_at
FROM public.service_provider_note_executions
WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW paipai_readonly.service_provider_notes AS
SELECT note_id, note_content
FROM public.service_provider_notes;

CREATE OR REPLACE VIEW paipai_readonly.xhs_jg_advertisers AS
SELECT advertiser_id, advertiser_name, first_seen_at, last_seen_at,
       last_full_synced_at, last_unit_full_synced_at, last_creativity_full_synced_at,
       last_incremental_synced_at, last_campaign_incremental_synced_at,
       last_unit_incremental_synced_at
FROM public.xhs_jg_advertisers;

CREATE OR REPLACE VIEW paipai_readonly.xhs_jg_campaigns AS
SELECT advertiser_id, campaign_id, campaign_name, campaign_filter_state,
       campaign_enable, marketing_target, placement, optimize_target, promotion_target,
       bidding_strategy, campaign_day_budget, campaign_created_at, campaign_updated_at,
       start_date, expire_date, first_seen_at, last_seen_at, synced_at
FROM public.xhs_jg_campaigns
WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW paipai_readonly.xhs_jg_units AS
SELECT advertiser_id, unit_id, campaign_id, unit_name, unit_enable, unit_filter_state,
       event_bid, target_type, not_available_status, creation_type, unit_created_at,
       unit_updated_at, first_seen_at, last_seen_at, synced_at
FROM public.xhs_jg_units
WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW paipai_readonly.xhs_jg_creativities AS
SELECT advertiser_id, creativity_id, campaign_id, unit_id, creativity_name,
       creativity_enable, creativity_filter_state, material_type, conversion_type,
       note_id, item_id, audit_status, creativity_audit_state, creation_type,
       creativity_created_at, creativity_updated_at, first_seen_at, last_seen_at, synced_at
FROM public.xhs_jg_creativities
WHERE deleted_at IS NULL;

REVOKE ALL ON ALL TABLES IN SCHEMA paipai_readonly FROM PUBLIC;
GRANT USAGE ON SCHEMA paipai_readonly TO paipai_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA paipai_readonly TO paipai_reader;

COMMIT;
