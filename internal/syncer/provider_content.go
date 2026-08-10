package syncer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"paipai-red-campaign-manager/internal/model"
)

var ErrUnknownProvider = errors.New("unknown or disabled service provider")

const providerNoteBatchSize = 1

type ProviderSource interface {
	FetchProviderContent(context.Context, model.ProviderContentTable) (model.ProviderContentSnapshot, error)
	FetchProviderNotes(context.Context, []model.DocumentRef) ([]model.ProviderNote, int, error)
}

type ProviderDestination interface {
	ProviderContentTables(context.Context) ([]model.ProviderContentTable, error)
	ProviderNotesToFetch(context.Context, []model.DocumentRef) ([]model.DocumentRef, error)
	MarkProviderContentSyncStarted(context.Context, string) error
	MarkProviderContentSyncFailed(context.Context, string, error) error
	UpsertProviderNotes(context.Context, []model.ProviderNote) error
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
	return s.RunProviders(ctx, nil)
}

func (s *ProviderSyncer) RunProviders(ctx context.Context, providerCodes []string) (model.ProviderSyncResult, error) {
	if !s.running.CompareAndSwap(false, true) {
		return model.ProviderSyncResult{}, ErrAlreadyRunning
	}
	defer s.running.Store(false)

	tables, err := s.destination.ProviderContentTables(ctx)
	if err != nil {
		return model.ProviderSyncResult{}, err
	}
	tables, err = selectProviderContentTables(tables, providerCodes)
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
		noteRefs, fetchErr := s.destination.ProviderNotesToFetch(ctx, snapshot.NoteRefs)
		if fetchErr != nil {
			syncErr := fmt.Errorf("find incremental notes for provider %s: %w", table.ProviderName, fetchErr)
			if markErr := s.markFailed(table.ProviderCode, fetchErr); markErr != nil {
				syncErr = errors.Join(syncErr, markErr)
			}
			syncErrors = append(syncErrors, syncErr)
			continue
		}
		noteCount := 0
		noteErrors := snapshot.NoteErrors
		noteSyncFailed := false
		for start := 0; start < len(noteRefs); start += providerNoteBatchSize {
			end := min(start+providerNoteBatchSize, len(noteRefs))
			notes, batchErrors, fetchErr := s.source.FetchProviderNotes(ctx, noteRefs[start:end])
			if fetchErr != nil {
				syncErr := fmt.Errorf("fetch incremental notes for provider %s: %w", table.ProviderName, fetchErr)
				if markErr := s.markFailed(table.ProviderCode, fetchErr); markErr != nil {
					syncErr = errors.Join(syncErr, markErr)
				}
				syncErrors = append(syncErrors, syncErr)
				noteSyncFailed = true
				break
			}
			if persistErr := s.destination.UpsertProviderNotes(ctx, notes); persistErr != nil {
				syncErr := fmt.Errorf("persist incremental notes for provider %s: %w", table.ProviderName, persistErr)
				if markErr := s.markFailed(table.ProviderCode, persistErr); markErr != nil {
					syncErr = errors.Join(syncErr, markErr)
				}
				syncErrors = append(syncErrors, syncErr)
				noteSyncFailed = true
				break
			}
			noteCount += len(notes)
			noteErrors += batchErrors
		}
		if noteSyncFailed {
			continue
		}
		snapshot.NoteRefs = noteRefs
		snapshot.Notes = nil
		snapshot.NoteErrors = noteErrors

		providerResult, replaceErr := s.destination.ReplaceProviderContentSnapshot(ctx, snapshot)
		if replaceErr != nil {
			syncErr := fmt.Errorf("sync provider %s: %w", table.ProviderName, replaceErr)
			if markErr := s.markFailed(table.ProviderCode, replaceErr); markErr != nil {
				syncErr = errors.Join(syncErr, markErr)
			}
			syncErrors = append(syncErrors, syncErr)
			continue
		}
		providerResult.Notes += noteCount
		result.Providers += providerResult.Providers
		result.Fetched += providerResult.Fetched
		result.Upserted += providerResult.Upserted
		result.Deleted += providerResult.Deleted
		result.Notes += providerResult.Notes
		result.NoteErrors += providerResult.NoteErrors
	}
	return result, errors.Join(syncErrors...)
}

func selectProviderContentTables(tables []model.ProviderContentTable, providerCodes []string) ([]model.ProviderContentTable, error) {
	if len(providerCodes) == 0 {
		return tables, nil
	}
	requested := make(map[string]struct{}, len(providerCodes))
	for _, providerCode := range providerCodes {
		providerCode = strings.TrimSpace(providerCode)
		if providerCode == "" {
			return nil, fmt.Errorf("%w: provider code cannot be empty", ErrUnknownProvider)
		}
		requested[providerCode] = struct{}{}
	}
	selected := make([]model.ProviderContentTable, 0, len(requested))
	for _, table := range tables {
		if _, ok := requested[table.ProviderCode]; ok {
			selected = append(selected, table)
			delete(requested, table.ProviderCode)
		}
	}
	if len(requested) > 0 {
		unknown := make([]string, 0, len(requested))
		for providerCode := range requested {
			unknown = append(unknown, providerCode)
		}
		sort.Strings(unknown)
		return nil, fmt.Errorf("%w: %s", ErrUnknownProvider, strings.Join(unknown, ", "))
	}
	return selected, nil
}

func (s *ProviderSyncer) markFailed(providerCode string, syncErr error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.destination.MarkProviderContentSyncFailed(ctx, providerCode, syncErr)
}
