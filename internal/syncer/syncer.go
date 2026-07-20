package syncer

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"paipai-red-campaign-manager/internal/model"
)

var ErrAlreadyRunning = errors.New("sync is already running in this process")

type Source interface {
	Snapshot(ctx context.Context) (model.Snapshot, error)
	FetchDocuments(ctx context.Context, refs []model.DocumentRef) ([]model.Document, error)
}

type Destination interface {
	DocumentsToRefresh(ctx context.Context, refs []model.DocumentRef, staleBefore time.Time) ([]model.DocumentRef, error)
	ReplaceSnapshot(ctx context.Context, snapshot model.Snapshot, documents []model.Document) (model.SyncResult, error)
}

type Syncer struct {
	source          Source
	destination     Destination
	refreshInterval time.Duration
	running         atomic.Bool
}

func New(source Source, destination Destination, refreshInterval time.Duration) *Syncer {
	return &Syncer{
		source:          source,
		destination:     destination,
		refreshInterval: refreshInterval,
	}
}

func (s *Syncer) Run(ctx context.Context) (model.SyncResult, error) {
	if !s.running.CompareAndSwap(false, true) {
		return model.SyncResult{}, ErrAlreadyRunning
	}
	defer s.running.Store(false)

	snapshot, err := s.source.Snapshot(ctx)
	if err != nil {
		return model.SyncResult{}, err
	}
	refs, err := s.destination.DocumentsToRefresh(ctx, snapshot.DocumentRefs, time.Now().Add(-s.refreshInterval))
	if err != nil {
		return model.SyncResult{}, err
	}
	documents, err := s.source.FetchDocuments(ctx, refs)
	if err != nil {
		return model.SyncResult{}, err
	}
	return s.destination.ReplaceSnapshot(ctx, snapshot, documents)
}
