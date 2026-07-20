package syncer

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"paipai-red-campaign-manager/internal/model"
)

type ProviderSource interface {
	FetchProviderContent(context.Context, model.ProviderContentTable) (model.ProviderContentSnapshot, error)
}

type ProviderDestination interface {
	ProviderContentTables(context.Context) ([]model.ProviderContentTable, error)
	MarkProviderContentSyncStarted(context.Context, string) error
	MarkProviderContentSyncFailed(context.Context, string, error) error
	ReplaceProviderContentSnapshot(context.Context, model.ProviderContentSnapshot) (model.ProviderSyncResult, error)
}

type ProviderSyncer struct {
	source      ProviderSource
	destination ProviderDestination
	running     atomic.Bool
}

func NewProvider(source ProviderSource, destination ProviderDestination) *ProviderSyncer {
	return &ProviderSyncer{source: source, destination: destination}
}

func (s *ProviderSyncer) Run(ctx context.Context) (model.ProviderSyncResult, error) {
	if !s.running.CompareAndSwap(false, true) {
		return model.ProviderSyncResult{}, ErrAlreadyRunning
	}
	defer s.running.Store(false)

	tables, err := s.destination.ProviderContentTables(ctx)
	if err != nil {
		return model.ProviderSyncResult{}, err
	}
	var result model.ProviderSyncResult
	var syncErrors []error
	for _, table := range tables {
		if err := ctx.Err(); err != nil {
			syncErrors = append(syncErrors, err)
			break
		}
		if err := s.destination.MarkProviderContentSyncStarted(ctx, table.ProviderCode); err != nil {
			syncErrors = append(syncErrors, fmt.Errorf("start provider %s sync: %w", table.ProviderName, err))
			continue
		}

		snapshot, fetchErr := s.source.FetchProviderContent(ctx, table)
		if fetchErr != nil {
			syncErr := fmt.Errorf("sync provider %s: %w", table.ProviderName, fetchErr)
			if markErr := s.markFailed(table.ProviderCode, fetchErr); markErr != nil {
				syncErr = errors.Join(syncErr, markErr)
			}
			syncErrors = append(syncErrors, syncErr)
			continue
		}

		providerResult, replaceErr := s.destination.ReplaceProviderContentSnapshot(ctx, snapshot)
		if replaceErr != nil {
			syncErr := fmt.Errorf("sync provider %s: %w", table.ProviderName, replaceErr)
			if markErr := s.markFailed(table.ProviderCode, replaceErr); markErr != nil {
				syncErr = errors.Join(syncErr, markErr)
			}
			syncErrors = append(syncErrors, syncErr)
			continue
		}
		result.Providers += providerResult.Providers
		result.Fetched += providerResult.Fetched
		result.Upserted += providerResult.Upserted
		result.Deleted += providerResult.Deleted
	}
	return result, errors.Join(syncErrors...)
}

func (s *ProviderSyncer) markFailed(providerCode string, syncErr error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.destination.MarkProviderContentSyncFailed(ctx, providerCode, syncErr)
}
