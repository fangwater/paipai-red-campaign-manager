CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS service_provider_note_embeddings (
    note_id TEXT NOT NULL REFERENCES service_provider_notes(note_id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    dimensions INTEGER NOT NULL CHECK (dimensions > 0),
    content_hash TEXT NOT NULL,
    embedding VECTOR NOT NULL,
    chunk_count INTEGER NOT NULL CHECK (chunk_count > 0),
    embedded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (note_id, model, dimensions),
    CONSTRAINT service_provider_note_embeddings_embedding_dimensions_check
        CHECK (vector_dims(embedding) = dimensions)
);

-- Upgrade the pre-pgvector REAL[] draft in place without discarding embeddings.
ALTER TABLE service_provider_note_embeddings
    DROP CONSTRAINT IF EXISTS service_provider_note_embeddings_check;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'service_provider_note_embeddings'
          AND column_name = 'embedding'
          AND udt_name = '_float4'
    ) THEN
        ALTER TABLE service_provider_note_embeddings
            ALTER COLUMN embedding TYPE VECTOR USING embedding::VECTOR;
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'service_provider_note_embeddings'::regclass
          AND conname = 'service_provider_note_embeddings_embedding_dimensions_check'
    ) THEN
        ALTER TABLE service_provider_note_embeddings
            ADD CONSTRAINT service_provider_note_embeddings_embedding_dimensions_check
            CHECK (vector_dims(embedding) = dimensions);
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_provider_note_embeddings_model
    ON service_provider_note_embeddings (model, dimensions, embedded_at DESC);

CREATE TABLE IF NOT EXISTS service_provider_note_embedding_runs (
    id BIGSERIAL PRIMARY KEY,
    model TEXT NOT NULL,
    dimensions INTEGER NOT NULL CHECK (dimensions > 0),
    force_refresh BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'succeeded', 'partial', 'failed')),
    candidate_count INTEGER NOT NULL DEFAULT 0,
    embedded_count INTEGER NOT NULL DEFAULT 0,
    skipped_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    request_count INTEGER NOT NULL DEFAULT 0,
    token_count BIGINT NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

ALTER TABLE service_provider_note_embedding_runs
    ADD COLUMN IF NOT EXISTS request_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS token_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_note_embedding_runs_running
    ON service_provider_note_embedding_runs ((1))
    WHERE status = 'running';

CREATE INDEX IF NOT EXISTS idx_provider_note_embedding_runs_started
    ON service_provider_note_embedding_runs (started_at DESC);
