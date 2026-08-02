package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/model"
)

type Store interface {
	ProviderNotesForEmbedding(context.Context, string, int) ([]model.NoteEmbeddingSource, error)
	StartProviderNoteEmbeddingRun(context.Context, string, int, bool, int, int) (int64, error)
	SaveProviderNoteEmbeddingBatch(context.Context, int64, string, int, []model.NoteEmbeddingRecord, int64) error
	FinishProviderNoteEmbeddingRun(context.Context, int64, string, int, string) error
}

type Refresher struct {
	store      Store
	embedder   Embedder
	model      string
	dimensions int
}

func NewRefresher(store Store, embedder Embedder, modelName string, dimensions int) (*Refresher, error) {
	if store == nil {
		return nil, errors.New("embedding store is required")
	}
	if embedder == nil {
		return nil, errors.New("embedding client is required")
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, errors.New("embedding model is required")
	}
	if dimensions <= 0 {
		return nil, errors.New("embedding dimensions must be positive")
	}
	return &Refresher{store: store, embedder: embedder, model: modelName, dimensions: dimensions}, nil
}

func (refresher *Refresher) Refresh(ctx context.Context, force bool) (result model.NoteEmbeddingRefreshResult, err error) {
	result.Model = refresher.model
	result.Dimensions = refresher.dimensions

	sources, err := refresher.store.ProviderNotesForEmbedding(ctx, refresher.model, refresher.dimensions)
	if err != nil {
		return result, err
	}
	result.Total = len(sources)

	type candidate struct {
		noteID      string
		content     string
		contentHash string
	}
	candidates := make([]candidate, 0, len(sources))
	for _, source := range sources {
		content := normalizeContent(source.NoteContent)
		if content == "" || strings.EqualFold(content, "nan") {
			result.Skipped++
			continue
		}
		contentHash := hashContent(content)
		if !force && contentHash == source.ExistingHash {
			result.Skipped++
			continue
		}
		candidates = append(candidates, candidate{
			noteID: source.NoteID, content: content, contentHash: contentHash,
		})
	}
	result.Candidates = len(candidates)

	result.RunID, err = refresher.store.StartProviderNoteEmbeddingRun(
		ctx, refresher.model, refresher.dimensions, force, result.Candidates, result.Skipped,
	)
	if err != nil {
		return result, err
	}

	fail := func(runErr error) (model.NoteEmbeddingRefreshResult, error) {
		result.Failed = result.Candidates - result.Embedded
		status := "failed"
		if result.Embedded > 0 {
			status = "partial"
		}
		finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if finishErr := refresher.store.FinishProviderNoteEmbeddingRun(
			finishCtx, result.RunID, status, result.Failed, runErr.Error(),
		); finishErr != nil {
			runErr = errors.Join(runErr, finishErr)
		}
		return result, runErr
	}

	for start := 0; start < len(candidates); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(candidates) {
			end = len(candidates)
		}
		batch := candidates[start:end]
		inputs := make([]string, len(batch))
		for index := range batch {
			inputs[index] = batch[index].content
		}

		vectors, usage, embedErr := refresher.embedder.Embed(
			ctx, inputs, refresher.model, refresher.dimensions,
		)
		if embedErr != nil {
			return fail(fmt.Errorf("embed notes %d-%d: %w", start+1, end, embedErr))
		}
		records := make([]model.NoteEmbeddingRecord, len(batch))
		for index := range batch {
			records[index] = model.NoteEmbeddingRecord{
				NoteID: batch[index].noteID, ContentHash: batch[index].contentHash,
				Embedding: vectors[index],
			}
		}
		if saveErr := refresher.store.SaveProviderNoteEmbeddingBatch(
			ctx, result.RunID, refresher.model, refresher.dimensions, records, usage.TotalTokens,
		); saveErr != nil {
			return fail(fmt.Errorf("save notes %d-%d embeddings: %w", start+1, end, saveErr))
		}
		result.Embedded += len(records)
		result.Requests++
		result.Tokens += usage.TotalTokens
	}

	if err := refresher.store.FinishProviderNoteEmbeddingRun(ctx, result.RunID, "succeeded", 0, ""); err != nil {
		return result, err
	}
	return result, nil
}

func normalizeContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return strings.TrimSpace(content)
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
