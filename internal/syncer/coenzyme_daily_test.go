package syncer

import (
	"context"
	"errors"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/coenzyme"
)

type coenzymeSourceStub struct {
	snapshot coenzyme.Snapshot
	err      error
}

func (stub *coenzymeSourceStub) FetchCoenzymeQ10Daily(context.Context, string, string, string) (coenzyme.Snapshot, error) {
	return stub.snapshot, stub.err
}

type coenzymeDestinationStub struct {
	started   int
	applied   int
	finished  int
	finishErr error
}

func (stub *coenzymeDestinationStub) StartCoenzymeQ10Sync(context.Context, string, string, string) (coenzyme.SyncResult, error) {
	stub.started++
	return coenzyme.SyncResult{RunID: 7}, nil
}

func (stub *coenzymeDestinationStub) ApplyCoenzymeQ10Daily(_ context.Context, runID int64, snapshot coenzyme.Snapshot) (coenzyme.SyncResult, error) {
	stub.applied++
	return coenzyme.SyncResult{RunID: runID, Fetched: len(snapshot.Records), Inserted: len(snapshot.Records)}, nil
}

func (stub *coenzymeDestinationStub) FinishCoenzymeQ10Sync(_ context.Context, _ coenzyme.SyncResult, runErr error) error {
	stub.finished++
	stub.finishErr = runErr
	return nil
}

func TestCoenzymeQ10SyncerAppliesSnapshot(t *testing.T) {
	source := &coenzymeSourceStub{snapshot: coenzyme.Snapshot{Records: []coenzyme.DailyRecord{{ReportDate: time.Now()}}}}
	destination := &coenzymeDestinationStub{}
	result, err := NewCoenzymeQ10(source, destination, "wiki", "sheet", coenzyme.SheetName).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.RunID != 7 || result.Inserted != 1 || destination.started != 1 || destination.applied != 1 || destination.finished != 1 {
		t.Fatalf("result=%+v destination=%+v", result, destination)
	}
}

func TestCoenzymeQ10SyncerPersistsFetchFailure(t *testing.T) {
	sourceErr := errors.New("permission denied")
	destination := &coenzymeDestinationStub{}
	_, err := NewCoenzymeQ10(&coenzymeSourceStub{err: sourceErr}, destination, "wiki", "sheet", coenzyme.SheetName).Run(context.Background())
	if !errors.Is(err, sourceErr) || destination.applied != 0 || destination.finished != 1 || !errors.Is(destination.finishErr, sourceErr) {
		t.Fatalf("error=%v destination=%+v", err, destination)
	}
}
