package embedding

import (
	"context"
	"fmt"
	"testing"

	"paipai-red-campaign-manager/internal/model"
)

type embeddingStoreStub struct {
	sources        []model.NoteEmbeddingSource
	candidateCount int
	skippedCount   int
	saved          []model.NoteEmbeddingRecord
	requests       int
	tokens         int64
	finishStatus   string
	failedCount    int
}

func (stub *embeddingStoreStub) ProviderNotesForEmbedding(context.Context, string, int) ([]model.NoteEmbeddingSource, error) {
	return stub.sources, nil
}

func (stub *embeddingStoreStub) StartProviderNoteEmbeddingRun(
	_ context.Context, _ string, _ int, _ bool, candidates int, skipped int,
) (int64, error) {
	stub.candidateCount = candidates
	stub.skippedCount = skipped
	return 42, nil
}

func (stub *embeddingStoreStub) SaveProviderNoteEmbeddingBatch(
	_ context.Context,
	_ int64,
	_ string,
	_ int,
	records []model.NoteEmbeddingRecord,
	tokens int64,
) error {
	stub.saved = append(stub.saved, records...)
	stub.requests++
	stub.tokens += tokens
	return nil
}

func (stub *embeddingStoreStub) FinishProviderNoteEmbeddingRun(
	_ context.Context, _ int64, status string, failed int, _ string,
) error {
	stub.finishStatus = status
	stub.failedCount = failed
	return nil
}

type embedderStub struct {
	batchSizes []int
}

func (stub *embedderStub) Embed(_ context.Context, inputs []string, _ string, dimensions int) ([][]float32, Usage, error) {
	stub.batchSizes = append(stub.batchSizes, len(inputs))
	vectors := make([][]float32, len(inputs))
	for index := range inputs {
		vectors[index] = make([]float32, dimensions)
		vectors[index][index%dimensions] = 1
	}
	return vectors, Usage{TotalTokens: int64(len(inputs) * 10)}, nil
}

func TestRefresherSkipsUnchangedAndBatchesCandidates(t *testing.T) {
	unchangedContent := "unchanged note"
	store := &embeddingStoreStub{
		sources: []model.NoteEmbeddingSource{{
			NoteID: "unchanged", NoteContent: unchangedContent,
			ExistingHash: hashContent(unchangedContent),
		}},
	}
	for index := 0; index < 21; index++ {
		store.sources = append(store.sources, model.NoteEmbeddingSource{
			NoteID: fmt.Sprintf("note-%02d", index), NoteContent: fmt.Sprintf("content %d", index),
		})
	}
	embedder := &embedderStub{}
	refresher, err := NewRefresher(store, embedder, "test-model", 4)
	if err != nil {
		t.Fatalf("NewRefresher() error = %v", err)
	}
	result, err := refresher.Refresh(context.Background(), false)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if result.Total != 22 || result.Candidates != 21 || result.Skipped != 1 ||
		result.Embedded != 21 || result.Requests != 2 || result.Tokens != 210 {
		t.Fatalf("result = %+v", result)
	}
	if fmt.Sprint(embedder.batchSizes) != "[20 1]" {
		t.Fatalf("batch sizes = %v", embedder.batchSizes)
	}
	if store.candidateCount != 21 || store.skippedCount != 1 || len(store.saved) != 21 ||
		store.requests != 2 || store.tokens != 210 || store.finishStatus != "succeeded" {
		t.Fatalf("store = %+v", store)
	}
}

func TestRefresherForceRefreshesUnchangedContent(t *testing.T) {
	content := "same content"
	store := &embeddingStoreStub{sources: []model.NoteEmbeddingSource{{
		NoteID: "note-1", NoteContent: content, ExistingHash: hashContent(content),
	}}}
	embedder := &embedderStub{}
	refresher, err := NewRefresher(store, embedder, "test-model", 2)
	if err != nil {
		t.Fatalf("NewRefresher() error = %v", err)
	}
	result, err := refresher.Refresh(context.Background(), true)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if result.Embedded != 1 || result.Skipped != 0 || len(embedder.batchSizes) != 1 {
		t.Fatalf("result = %+v, batches = %v", result, embedder.batchSizes)
	}
}
