package syncer

import (
	"context"
	"errors"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/model"
)

type sourceStub struct {
	snapshot  model.Snapshot
	documents []model.Document
	err       error
}

func (s sourceStub) Snapshot(context.Context) (model.Snapshot, error) {
	return s.snapshot, s.err
}

func (s sourceStub) FetchDocuments(context.Context, []model.DocumentRef) ([]model.Document, error) {
	return s.documents, nil
}

type destinationStub struct {
	gotSnapshot  model.Snapshot
	gotDocuments []model.Document
}

func (d *destinationStub) DocumentsToRefresh(_ context.Context, refs []model.DocumentRef, _ time.Time) ([]model.DocumentRef, error) {
	return refs, nil
}

func (d *destinationStub) ReplaceSnapshot(_ context.Context, snapshot model.Snapshot, documents []model.Document) (model.SyncResult, error) {
	d.gotSnapshot = snapshot
	d.gotDocuments = documents
	return model.SyncResult{Tables: len(snapshot.Tables), Fetched: 1, Upserted: 1, Documents: len(documents)}, nil
}

func TestRun(t *testing.T) {
	snapshot := model.Snapshot{
		Tables:       []model.Table{{ID: "table-1", Records: []model.Record{{ID: "rec-1"}}}},
		DocumentRefs: []model.DocumentRef{{Provider: "feishu", ResourceKey: "docx:doc-1"}},
	}
	destination := &destinationStub{}
	s := New(sourceStub{
		snapshot:  snapshot,
		documents: []model.Document{{Provider: "feishu", ResourceKey: "docx:doc-1"}},
	}, destination, time.Hour)

	result, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Upserted != 1 || len(destination.gotSnapshot.Tables) != 1 || len(destination.gotDocuments) != 1 {
		t.Fatalf("Run() result = %+v, tables = %d, documents = %d", result, len(destination.gotSnapshot.Tables), len(destination.gotDocuments))
	}
}

func TestRunStopsOnSourceError(t *testing.T) {
	wantErr := errors.New("source failed")
	destination := &destinationStub{}
	s := New(sourceStub{err: wantErr}, destination, time.Hour)

	_, err := s.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
	if destination.gotSnapshot.Tables != nil {
		t.Fatal("destination called after source error")
	}
}
