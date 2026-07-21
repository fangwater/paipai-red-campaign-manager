package syncer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"paipai-red-campaign-manager/internal/model"
)

type providerSourceStub struct {
	failedCode string
}

func (s providerSourceStub) FetchProviderContent(_ context.Context, table model.ProviderContentTable) (model.ProviderContentSnapshot, error) {
	if table.ProviderCode == s.failedCode {
		return model.ProviderContentSnapshot{}, errors.New("source unavailable")
	}
	return model.ProviderContentSnapshot{
		Table: table,
		Records: []model.ProviderNoteExecution{{
			RecordKey: "row:2", SourceRowNumber: 2, NoteID: "note-1",
		}},
		NoteRefs:   []model.DocumentRef{{RecordID: "note-1"}},
		NoteErrors: 1,
	}, nil
}

func (s providerSourceStub) FetchProviderNotes(_ context.Context, refs []model.DocumentRef) ([]model.ProviderNote, int, error) {
	notes := make([]model.ProviderNote, 0, len(refs))
	for _, ref := range refs {
		notes = append(notes, model.ProviderNote{NoteID: ref.RecordID, NoteContent: "正文"})
	}
	return notes, 0, nil
}

type providerDestinationStub struct {
	tables  []model.ProviderContentTable
	started []string
	failed  []string
	saved   []string
}

func (d *providerDestinationStub) ProviderContentTables(context.Context) ([]model.ProviderContentTable, error) {
	return d.tables, nil
}

func (d *providerDestinationStub) ProviderNotesToFetch(_ context.Context, refs []model.DocumentRef) ([]model.DocumentRef, error) {
	return refs, nil
}

func (d *providerDestinationStub) MarkProviderContentSyncStarted(_ context.Context, providerCode string) error {
	d.started = append(d.started, providerCode)
	return nil
}

func (d *providerDestinationStub) MarkProviderContentSyncFailed(_ context.Context, providerCode string, _ error) error {
	d.failed = append(d.failed, providerCode)
	return nil
}

func (d *providerDestinationStub) ReplaceProviderContentSnapshot(_ context.Context, snapshot model.ProviderContentSnapshot) (model.ProviderSyncResult, error) {
	d.saved = append(d.saved, snapshot.Table.ProviderCode)
	return model.ProviderSyncResult{
		Providers: 1, Fetched: len(snapshot.Records), Upserted: len(snapshot.Records),
		Notes: len(snapshot.Notes), NoteErrors: snapshot.NoteErrors,
	}, nil
}

func TestProviderRunContinuesAfterProviderFailure(t *testing.T) {
	destination := &providerDestinationStub{tables: []model.ProviderContentTable{
		{ProviderCode: "manjie", ProviderName: "曼杰"},
		{ProviderCode: "youyiyouer", ProviderName: "有一有二"},
	}}
	service := NewProvider(providerSourceStub{failedCode: "manjie"}, destination)

	result, err := service.Run(context.Background())
	if err == nil || !containsError(err, "source unavailable") {
		t.Fatalf("Run() error = %v, want source failure", err)
	}
	if result.Providers != 1 || result.Upserted != 1 || result.Notes != 1 || result.NoteErrors != 1 {
		t.Fatalf("Run() result = %+v", result)
	}
	if len(destination.started) != 2 || len(destination.failed) != 1 || destination.failed[0] != "manjie" {
		t.Fatalf("started = %v, failed = %v", destination.started, destination.failed)
	}
	if len(destination.saved) != 1 || destination.saved[0] != "youyiyouer" {
		t.Fatalf("saved = %v", destination.saved)
	}
}

func containsError(err error, text string) bool {
	return err != nil && strings.Contains(err.Error(), text)
}
