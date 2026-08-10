package syncer

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"paipai-red-campaign-manager/internal/model"
)

type providerSourceStub struct {
	failedCode         string
	noteCount          int
	fetchedNoteBatches *[]int
}

func (s providerSourceStub) FetchProviderContent(_ context.Context, table model.ProviderContentTable) (model.ProviderContentSnapshot, error) {
	if table.ProviderCode == s.failedCode {
		return model.ProviderContentSnapshot{}, errors.New("source unavailable")
	}
	noteCount := s.noteCount
	if noteCount == 0 {
		noteCount = 1
	}
	refs := make([]model.DocumentRef, 0, noteCount)
	for index := 0; index < noteCount; index++ {
		refs = append(refs, model.DocumentRef{RecordID: fmt.Sprintf("note-%d", index+1)})
	}
	return model.ProviderContentSnapshot{
		Table: table,
		Records: []model.ProviderNoteExecution{{
			RecordKey: "row:2", SourceRowNumber: 2, NoteID: "note-1",
		}},
		NoteRefs:   refs,
		NoteErrors: 1,
	}, nil
}

func (s providerSourceStub) FetchProviderNotes(_ context.Context, refs []model.DocumentRef) ([]model.ProviderNote, int, error) {
	if s.fetchedNoteBatches != nil {
		*s.fetchedNoteBatches = append(*s.fetchedNoteBatches, len(refs))
	}
	notes := make([]model.ProviderNote, 0, len(refs))
	for _, ref := range refs {
		notes = append(notes, model.ProviderNote{NoteID: ref.RecordID, NoteContent: "正文"})
	}
	return notes, 0, nil
}

type providerDestinationStub struct {
	tables      []model.ProviderContentTable
	started     []string
	failed      []string
	saved       []string
	sourceRefs  []model.DocumentRef
	noteBatches []int
}

func (d *providerDestinationStub) ProviderContentTables(context.Context) ([]model.ProviderContentTable, error) {
	return d.tables, nil
}

func (d *providerDestinationStub) ProviderNotesToFetch(_ context.Context, refs []model.DocumentRef) ([]model.DocumentRef, error) {
	return refs, nil
}

func (d *providerDestinationStub) UpdateProviderNoteSources(_ context.Context, refs []model.DocumentRef) error {
	d.sourceRefs = append(d.sourceRefs, refs...)
	return nil
}

func (d *providerDestinationStub) MarkProviderContentSyncStarted(_ context.Context, providerCode string) error {
	d.started = append(d.started, providerCode)
	return nil
}

func (d *providerDestinationStub) MarkProviderContentSyncFailed(_ context.Context, providerCode string, _ error) error {
	d.failed = append(d.failed, providerCode)
	return nil
}

func (d *providerDestinationStub) UpsertProviderNotes(_ context.Context, notes []model.ProviderNote) error {
	d.noteBatches = append(d.noteBatches, len(notes))
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

func TestProviderRunProvidersOnlySyncsRequestedTargets(t *testing.T) {
	destination := &providerDestinationStub{tables: []model.ProviderContentTable{
		{ProviderCode: "manjie", ProviderName: "曼杰"},
		{ProviderCode: "youyiyouer", ProviderName: "有一有二"},
		{ProviderCode: "zhiyuan", ProviderName: "智元"},
	}}
	service := NewProvider(providerSourceStub{}, destination)

	result, err := service.RunProviders(context.Background(), []string{"zhiyuan", "manjie", "zhiyuan"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Providers != 2 {
		t.Fatalf("RunProviders() result = %+v", result)
	}
	if strings.Join(destination.saved, ",") != "manjie,zhiyuan" {
		t.Fatalf("saved providers = %v", destination.saved)
	}
}

func TestProviderRunProvidersRejectsUnknownTargetBeforeSync(t *testing.T) {
	destination := &providerDestinationStub{tables: []model.ProviderContentTable{
		{ProviderCode: "manjie", ProviderName: "曼杰"},
	}}
	service := NewProvider(providerSourceStub{}, destination)

	_, err := service.RunProviders(context.Background(), []string{"missing"})
	if !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("RunProviders() error = %v, want ErrUnknownProvider", err)
	}
	if len(destination.started) != 0 || len(destination.saved) != 0 {
		t.Fatalf("sync started for unknown target: started=%v saved=%v", destination.started, destination.saved)
	}
}

func TestProviderRunPersistsNotesInBoundedBatches(t *testing.T) {
	var fetchedBatches []int
	destination := &providerDestinationStub{tables: []model.ProviderContentTable{{
		ProviderCode: "zhiyuan", ProviderName: "智元",
	}}}
	service := NewProvider(providerSourceStub{
		noteCount: providerNoteBatchSize*2 + 1, fetchedNoteBatches: &fetchedBatches,
	}, destination)

	result, err := service.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []int{providerNoteBatchSize, providerNoteBatchSize, 1}
	if !slices.Equal(fetchedBatches, want) || !slices.Equal(destination.noteBatches, want) {
		t.Fatalf("fetched batches=%v persisted batches=%v want=%v", fetchedBatches, destination.noteBatches, want)
	}
	if len(destination.sourceRefs) != providerNoteBatchSize*2+1 {
		t.Fatalf("updated source refs=%d", len(destination.sourceRefs))
	}
	if result.Notes != providerNoteBatchSize*2+1 {
		t.Fatalf("Run() result = %+v", result)
	}
}

func containsError(err error, text string) bool {
	return err != nil && strings.Contains(err.Error(), text)
}
