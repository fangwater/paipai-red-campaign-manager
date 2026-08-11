package syncer

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"paipai-red-campaign-manager/internal/coenzyme"
)

type CoenzymeQ10Source interface {
	FetchCoenzymeQ10Daily(context.Context, string, string, string) (coenzyme.Snapshot, error)
}

type CoenzymeQ10Destination interface {
	StartCoenzymeQ10Sync(context.Context, string, string, string) (coenzyme.SyncResult, error)
	ApplyCoenzymeQ10Daily(context.Context, int64, coenzyme.Snapshot) (coenzyme.SyncResult, error)
	FinishCoenzymeQ10Sync(context.Context, coenzyme.SyncResult, error) error
}

type CoenzymeQ10Syncer struct {
	source      CoenzymeQ10Source
	destination CoenzymeQ10Destination
	wikiToken   string
	sheetID     string
	sheetName   string
	running     atomic.Bool
}

func NewCoenzymeQ10(source CoenzymeQ10Source, destination CoenzymeQ10Destination, wikiToken, sheetID, sheetName string) *CoenzymeQ10Syncer {
	return &CoenzymeQ10Syncer{
		source: source, destination: destination,
		wikiToken: wikiToken, sheetID: sheetID, sheetName: sheetName,
	}
}

func (s *CoenzymeQ10Syncer) Run(ctx context.Context) (result coenzyme.SyncResult, err error) {
	if !s.running.CompareAndSwap(false, true) {
		return coenzyme.SyncResult{}, ErrAlreadyRunning
	}
	defer s.running.Store(false)

	result, err = s.destination.StartCoenzymeQ10Sync(ctx, s.wikiToken, s.sheetID, s.sheetName)
	if err != nil {
		return result, err
	}
	defer func() {
		finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if finishErr := s.destination.FinishCoenzymeQ10Sync(finishCtx, result, err); finishErr != nil {
			err = errors.Join(err, finishErr)
		}
	}()

	snapshot, err := s.source.FetchCoenzymeQ10Daily(ctx, s.wikiToken, s.sheetID, s.sheetName)
	if err != nil {
		return result, fmt.Errorf("fetch coenzyme Q10 daily data: %w", err)
	}
	result, err = s.destination.ApplyCoenzymeQ10Daily(ctx, result.RunID, snapshot)
	return result, err
}
