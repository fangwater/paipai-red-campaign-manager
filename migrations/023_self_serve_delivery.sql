CREATE TABLE IF NOT EXISTS delivery_drafts (
    id TEXT PRIMARY KEY CHECK (id ~ '^drf_[0-9a-f]{32}$'),
    advertiser_id BIGINT NOT NULL CHECK (advertiser_id > 0),
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'validated', 'pending_approval', 'approved', 'publishing', 'paused', 'active', 'failed', 'cancelled')),
    current_version INTEGER NOT NULL DEFAULT 1 CHECK (current_version > 0),
    spec JSONB NOT NULL CHECK (jsonb_typeof(spec) = 'object'),
    spec_hash TEXT NOT NULL CHECK (spec_hash ~ '^[0-9a-f]{64}$'),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 8 AND 160),
    created_by TEXT NOT NULL,
    updated_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (advertiser_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_delivery_drafts_advertiser
    ON delivery_drafts (advertiser_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS delivery_draft_versions (
    draft_id TEXT NOT NULL REFERENCES delivery_drafts(id) ON DELETE CASCADE,
    version INTEGER NOT NULL CHECK (version > 0),
    spec JSONB NOT NULL CHECK (jsonb_typeof(spec) = 'object'),
    spec_hash TEXT NOT NULL CHECK (spec_hash ~ '^[0-9a-f]{64}$'),
    change_reason TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (draft_id, version)
);

CREATE TABLE IF NOT EXISTS delivery_recommendations (
    id TEXT PRIMARY KEY CHECK (id ~ '^rec_[0-9a-f]{32}$'),
    draft_id TEXT NOT NULL REFERENCES delivery_drafts(id) ON DELETE CASCADE,
    draft_version INTEGER NOT NULL,
    schema_version TEXT NOT NULL,
    llm_provider TEXT NOT NULL,
    llm_model TEXT NOT NULL,
    ranker_family TEXT NOT NULL,
    ranker_version TEXT NOT NULL,
    rules_version TEXT NOT NULL,
    payload JSONB NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    warnings TEXT[] NOT NULL DEFAULT '{}',
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (draft_id, draft_version)
        REFERENCES delivery_draft_versions(draft_id, version) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_delivery_recommendations_draft
    ON delivery_recommendations (draft_id, draft_version, created_at DESC);

CREATE TABLE IF NOT EXISTS delivery_validations (
    id TEXT PRIMARY KEY CHECK (id ~ '^val_[0-9a-f]{32}$'),
    draft_id TEXT NOT NULL REFERENCES delivery_drafts(id) ON DELETE CASCADE,
    draft_version INTEGER NOT NULL,
    spec_hash TEXT NOT NULL,
    rules_version TEXT NOT NULL,
    contract_version TEXT NOT NULL,
    valid BOOLEAN NOT NULL,
    errors JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(errors) = 'array'),
    warnings JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(warnings) = 'array'),
    capability_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(capability_snapshot) = 'object'),
    valid_until TIMESTAMPTZ NOT NULL,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (draft_id, draft_version)
        REFERENCES delivery_draft_versions(draft_id, version) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_delivery_validations_current
    ON delivery_validations (draft_id, draft_version, created_at DESC);

CREATE TABLE IF NOT EXISTS delivery_approvals (
    id TEXT PRIMARY KEY CHECK (id ~ '^apr_[0-9a-f]{32}$'),
    draft_id TEXT NOT NULL REFERENCES delivery_drafts(id) ON DELETE CASCADE,
    draft_version INTEGER NOT NULL,
    spec_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('operator', 'budget_owner')),
    decision TEXT NOT NULL CHECK (decision IN ('approved', 'rejected')),
    actor TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    approved_budget_fen BIGINT NOT NULL CHECK (approved_budget_fen >= 0),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (draft_id, draft_version)
        REFERENCES delivery_draft_versions(draft_id, version) ON DELETE CASCADE
);

ALTER TABLE delivery_approvals
    DROP CONSTRAINT IF EXISTS delivery_approvals_draft_id_draft_version_role_key;
CREATE INDEX IF NOT EXISTS idx_delivery_approvals_latest
    ON delivery_approvals (draft_id, draft_version, role, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS delivery_publish_jobs (
    id TEXT PRIMARY KEY CHECK (id ~ '^job_[0-9a-f]{32}$'),
    draft_id TEXT NOT NULL REFERENCES delivery_drafts(id) ON DELETE RESTRICT,
    draft_version INTEGER NOT NULL,
    advertiser_id BIGINT NOT NULL CHECK (advertiser_id > 0),
    mode TEXT NOT NULL CHECK (mode IN ('dry_run', 'execute')),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'publishing', 'succeeded', 'failed')),
    current_step TEXT NOT NULL DEFAULT 'queued',
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 8 AND 200),
    request_preview JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(request_preview) = 'object'),
    result JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(result) = 'object'),
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    requested_by TEXT NOT NULL,
    requested_role TEXT NOT NULL CHECK (requested_role IN ('operator', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (draft_id, draft_version)
        REFERENCES delivery_draft_versions(draft_id, version) ON DELETE RESTRICT,
    UNIQUE (idempotency_key)
);

-- Keep the bundled migration repeatable if a previous binary created the table
-- before worker identity was persisted on publish jobs.
ALTER TABLE delivery_publish_jobs
    ADD COLUMN IF NOT EXISTS requested_role TEXT NOT NULL DEFAULT 'operator';
UPDATE delivery_publish_jobs
SET requested_role = 'operator'
WHERE requested_role NOT IN ('operator', 'admin');
ALTER TABLE delivery_publish_jobs ALTER COLUMN requested_role DROP DEFAULT;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'delivery_publish_jobs'::regclass
          AND conname = 'delivery_publish_jobs_requested_role_check'
    ) THEN
        ALTER TABLE delivery_publish_jobs
            ADD CONSTRAINT delivery_publish_jobs_requested_role_check
            CHECK (requested_role IN ('operator', 'admin'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_delivery_publish_jobs_queue
    ON delivery_publish_jobs (created_at)
    WHERE status = 'queued';

DROP INDEX IF EXISTS idx_delivery_publish_jobs_single_execute;
CREATE UNIQUE INDEX IF NOT EXISTS idx_delivery_publish_jobs_single_active_execute
    ON delivery_publish_jobs (draft_id, draft_version)
    WHERE mode = 'execute'
      AND (status IN ('queued', 'publishing', 'succeeded')
           OR (status = 'failed' AND COALESCE((result->>'executed')::boolean, TRUE)));

CREATE TABLE IF NOT EXISTS delivery_media_entities (
    id TEXT PRIMARY KEY CHECK (id ~ '^ent_[0-9a-f]{32}$'),
    job_id TEXT NOT NULL REFERENCES delivery_publish_jobs(id) ON DELETE RESTRICT,
    draft_id TEXT NOT NULL REFERENCES delivery_drafts(id) ON DELETE RESTRICT,
    advertiser_id BIGINT NOT NULL CHECK (advertiser_id > 0),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('campaign', 'unit', 'creativity')),
    local_key TEXT NOT NULL,
    parent_local_key TEXT NOT NULL DEFAULT '',
    media_id BIGINT NOT NULL CHECK (media_id > 0),
    parent_media_id BIGINT CHECK (parent_media_id > 0),
    desired_status TEXT NOT NULL DEFAULT 'paused' CHECK (desired_status IN ('paused', 'active', 'deleted')),
    observed_status TEXT NOT NULL DEFAULT 'unknown',
    upstream_payload JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(upstream_payload) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (advertiser_id, entity_type, media_id),
    UNIQUE (job_id, entity_type, local_key)
);

CREATE INDEX IF NOT EXISTS idx_delivery_media_entities_draft
    ON delivery_media_entities (draft_id, entity_type, created_at);

CREATE TABLE IF NOT EXISTS delivery_api_attempts (
    id BIGSERIAL PRIMARY KEY,
    job_id TEXT REFERENCES delivery_publish_jobs(id) ON DELETE SET NULL,
    advertiser_id BIGINT NOT NULL CHECK (advertiser_id > 0),
    operation TEXT NOT NULL,
    contract_version TEXT NOT NULL,
    request_hash TEXT NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    request_summary JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(request_summary) = 'object'),
    response_summary JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(response_summary) = 'object'),
    upstream_request_id TEXT NOT NULL DEFAULT '',
    success BOOLEAN NOT NULL,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    latency_ms BIGINT NOT NULL CHECK (latency_ms >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_delivery_api_attempts_job
    ON delivery_api_attempts (job_id, created_at);

CREATE TABLE IF NOT EXISTS delivery_performance_snapshots (
    id BIGSERIAL PRIMARY KEY,
    advertiser_id BIGINT NOT NULL CHECK (advertiser_id > 0),
    level TEXT NOT NULL CHECK (level IN ('account', 'campaign', 'unit', 'creativity', 'keyword')),
    realtime BOOLEAN NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    contract_version TEXT NOT NULL,
    payload JSONB NOT NULL CHECK (jsonb_typeof(payload) IN ('object', 'array')),
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_delivery_performance_lookup
    ON delivery_performance_snapshots (advertiser_id, level, end_date DESC, fetched_at DESC);

CREATE TABLE IF NOT EXISTS delivery_guardrail_events (
    id BIGSERIAL PRIMARY KEY,
    draft_id TEXT REFERENCES delivery_drafts(id) ON DELETE SET NULL,
    job_id TEXT REFERENCES delivery_publish_jobs(id) ON DELETE SET NULL,
    advertiser_id BIGINT NOT NULL CHECK (advertiser_id > 0),
    guardrail TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    action TEXT NOT NULL,
    detail JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(detail) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS delivery_audit_log (
    id BIGSERIAL PRIMARY KEY,
    actor TEXT NOT NULL,
    role TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    advertiser_id BIGINT,
    detail JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(detail) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_delivery_audit_resource
    ON delivery_audit_log (resource_type, resource_id, created_at DESC);

COMMENT ON TABLE delivery_drafts IS '自建投流当前草稿与状态；媒体写入只允许从审批后的不可变版本发起';
COMMENT ON TABLE delivery_publish_jobs IS '自建投流幂等发布作业；execute 作业由单 worker 逐层创建且计划始终先以暂停态创建';
COMMENT ON TABLE delivery_api_attempts IS '脱敏后的聚光上游调用审计，不存 OAuth Token';
