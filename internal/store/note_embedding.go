package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"paipai-red-campaign-manager/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNoteEmbeddingRefreshLocked = errors.New("another note embedding refresh is already running")

func (p *Postgres) ProviderNotesForEmbedding(ctx context.Context, modelName string, dimensions int) ([]model.NoteEmbeddingSource, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT notes.note_id, notes.note_content, COALESCE(embeddings.content_hash, '')
		FROM service_provider_notes AS notes
		LEFT JOIN service_provider_note_embeddings AS embeddings
		  ON embeddings.note_id = notes.note_id
		 AND embeddings.model = $1
		 AND embeddings.dimensions = $2
		ORDER BY notes.note_id
	`, modelName, dimensions)
	if err != nil {
		return nil, fmt.Errorf("query provider notes for embedding: %w", err)
	}
	defer rows.Close()

	sources := make([]model.NoteEmbeddingSource, 0)
	for rows.Next() {
		var source model.NoteEmbeddingSource
		if err := rows.Scan(&source.NoteID, &source.NoteContent, &source.ExistingHash); err != nil {
			return nil, fmt.Errorf("scan provider note for embedding: %w", err)
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider notes for embedding: %w", err)
	}
	return sources, nil
}

func (p *Postgres) StartProviderNoteEmbeddingRun(
	ctx context.Context,
	modelName string,
	dimensions int,
	force bool,
	candidateCount int,
	skippedCount int,
) (int64, error) {
	if _, err := p.pool.Exec(ctx, `
		UPDATE service_provider_note_embedding_runs
		SET status = 'failed',
			failed_count = GREATEST(candidate_count - embedded_count, 0),
			error_message = 'stale embedding run recovered before a new refresh',
			updated_at = NOW(),
			completed_at = NOW()
		WHERE status = 'running'
		  AND updated_at < NOW() - INTERVAL '15 minutes'
	`); err != nil {
		return 0, fmt.Errorf("recover stale embedding runs: %w", err)
	}

	var runID int64
	err := p.pool.QueryRow(ctx, `
		INSERT INTO service_provider_note_embedding_runs (
			model, dimensions, force_refresh, status, candidate_count, skipped_count
		) VALUES ($1, $2, $3, 'running', $4, $5)
		RETURNING id
	`, modelName, dimensions, force, candidateCount, skippedCount).Scan(&runID)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) &&
			postgresError.Code == "23505" &&
			postgresError.ConstraintName == "idx_provider_note_embedding_runs_running" {
			return 0, ErrNoteEmbeddingRefreshLocked
		}
		return 0, fmt.Errorf("start provider note embedding run: %w", err)
	}
	return runID, nil
}

func (p *Postgres) SaveProviderNoteEmbeddingBatch(
	ctx context.Context,
	runID int64,
	modelName string,
	dimensions int,
	records []model.NoteEmbeddingRecord,
	tokenCount int64,
) error {
	if len(records) == 0 {
		return nil
	}
	literals := make([]string, len(records))
	for index, record := range records {
		if len(record.Embedding) != dimensions {
			return fmt.Errorf("note %s embedding dimensions = %d, want %d", record.NoteID, len(record.Embedding), dimensions)
		}
		literal, err := vectorLiteral(record.Embedding)
		if err != nil {
			return fmt.Errorf("encode note %s embedding: %w", record.NoteID, err)
		}
		literals[index] = literal
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin provider note embedding batch: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	batch := &pgx.Batch{}
	for index, record := range records {
		batch.Queue(`
			INSERT INTO service_provider_note_embeddings (
				note_id, model, dimensions, content_hash, embedding, chunk_count,
				embedded_at, updated_at
			) VALUES ($1, $2, $3, $4, $5::vector, 1, NOW(), NOW())
			ON CONFLICT (note_id, model, dimensions) DO UPDATE SET
				content_hash = EXCLUDED.content_hash,
				embedding = EXCLUDED.embedding,
				chunk_count = 1,
				embedded_at = NOW(),
				updated_at = NOW()
		`, record.NoteID, modelName, dimensions, record.ContentHash, literals[index])
	}
	results := tx.SendBatch(ctx, batch)
	for index := 0; index < batch.Len(); index++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("upsert provider note embedding %d: %w", index+1, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close provider note embedding batch: %w", err)
	}

	commandTag, err := tx.Exec(ctx, `
		UPDATE service_provider_note_embedding_runs
		SET embedded_count = embedded_count + $2,
			request_count = request_count + 1,
			token_count = token_count + $3,
			updated_at = NOW()
		WHERE id = $1 AND status = 'running'
	`, runID, len(records), tokenCount)
	if err != nil {
		return fmt.Errorf("update provider note embedding run progress: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return errors.New("provider note embedding run is no longer active")
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit provider note embedding batch: %w", err)
	}
	return nil
}

func (p *Postgres) FinishProviderNoteEmbeddingRun(
	ctx context.Context,
	runID int64,
	status string,
	failedCount int,
	errorMessage string,
) error {
	if status != "succeeded" && status != "partial" && status != "failed" {
		return fmt.Errorf("invalid provider note embedding run status %q", status)
	}
	commandTag, err := p.pool.Exec(ctx, `
		UPDATE service_provider_note_embedding_runs
		SET status = $2,
			failed_count = $3,
			error_message = NULLIF($4, ''),
			updated_at = NOW(),
			completed_at = NOW()
		WHERE id = $1 AND status = 'running'
	`, runID, status, failedCount, errorMessage)
	if err != nil {
		return fmt.Errorf("finish provider note embedding run: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return errors.New("provider note embedding run is no longer active")
	}
	return nil
}

func (p *Postgres) SimilarProviderNotes(
	ctx context.Context,
	queryEmbedding []float32,
	modelName string,
	dimensions int,
	limit int,
) ([]model.SimilarProviderNote, error) {
	if len(queryEmbedding) != dimensions {
		return nil, fmt.Errorf("query embedding dimensions = %d, want %d", len(queryEmbedding), dimensions)
	}
	if limit <= 0 || limit > 100 {
		return nil, errors.New("similar note limit must be between 1 and 100")
	}
	literal, err := vectorLiteral(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("encode query embedding: %w", err)
	}
	rows, err := p.pool.Query(ctx, `
		SELECT notes.note_id, notes.note_content,
			1 - (embeddings.embedding <=> $1::vector) AS similarity
		FROM service_provider_note_embeddings AS embeddings
		JOIN service_provider_notes AS notes ON notes.note_id = embeddings.note_id
		WHERE embeddings.model = $2
		  AND embeddings.dimensions = $3
		ORDER BY embeddings.embedding <=> $1::vector
		LIMIT $4
	`, literal, modelName, dimensions, limit)
	if err != nil {
		return nil, fmt.Errorf("query similar provider notes: %w", err)
	}
	defer rows.Close()

	notes := make([]model.SimilarProviderNote, 0, limit)
	for rows.Next() {
		var note model.SimilarProviderNote
		if err := rows.Scan(&note.NoteID, &note.NoteContent, &note.Similarity); err != nil {
			return nil, fmt.Errorf("scan similar provider note: %w", err)
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate similar provider notes: %w", err)
	}
	return notes, nil
}

func vectorLiteral(values []float32) (string, error) {
	for _, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return "", errors.New("embedding contains a non-finite value")
		}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
