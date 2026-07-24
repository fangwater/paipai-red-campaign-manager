package xhssync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"paipai-red-campaign-manager/internal/store"
	"paipai-red-campaign-manager/internal/xhs"
)

type Mode string
type Target string

const (
	ModeIncremental Mode = "incremental"
	ModeFull        Mode = "full"

	TargetCampaigns    Target = "campaigns"
	TargetUnits        Target = "units"
	TargetCreativities Target = "creativities"
)

var ErrAlreadyRunning = errors.New("an XHS Spotlight sync is already running")

type Source interface {
	Status() xhs.ManagerStatus
	ListAllCampaigns(context.Context, xhs.CampaignListRequest) (xhs.CampaignCollection, error)
	ListAllUnits(context.Context, xhs.UnitListRequest) (xhs.UnitCollection, error)
	ListAllCreativities(context.Context, xhs.CreativityListRequest) (xhs.CreativityCollection, error)
}

type Destination interface {
	StartXHSJGSyncRun(context.Context, string, string, string, int64) (store.XHSJGSyncRun, error)
	FinishXHSJGSyncRun(context.Context, int64, string, int, int, int, int, int64, error) error
	ListXHSJGSyncRuns(context.Context, int) ([]store.XHSJGSyncRun, error)
	XHSJGIncrementalSince(context.Context, int64, string) (time.Time, error)
	MarkXHSJGIncrementalSynced(context.Context, int64, string, time.Time) error
	ReplaceXHSCampaignSnapshot(context.Context, xhs.Advertiser, []xhs.Campaign) (store.XHSCampaignStoreResult, error)
	UpsertXHSCampaigns(context.Context, xhs.Advertiser, []xhs.Campaign) (store.XHSCampaignStoreResult, error)
	ReplaceXHSUnitSnapshot(context.Context, xhs.Advertiser, []xhs.Unit) (store.XHSEntityStoreResult, error)
	UpsertXHSUnits(context.Context, xhs.Advertiser, []xhs.Unit) (store.XHSEntityStoreResult, error)
	ReplaceXHSCreativitySnapshot(context.Context, xhs.Advertiser, []xhs.Creativity) (store.XHSEntityStoreResult, error)
}

type Result struct {
	Advertisers  int   `json:"advertisers"`
	Campaigns    int   `json:"campaigns"`
	Units        int   `json:"units"`
	Creativities int   `json:"creativities"`
	Deactivated  int64 `json:"deactivated"`
}

type Status struct {
	Running bool                 `json:"running"`
	Current *store.XHSJGSyncRun  `json:"current,omitempty"`
	Recent  []store.XHSJGSyncRun `json:"recent"`
}

type Service struct {
	source      Source
	destination Destination
	logger      *slog.Logger
	timeout     time.Duration
	location    *time.Location
	now         func() time.Time
	gate        chan struct{}

	mu      sync.RWMutex
	current *store.XHSJGSyncRun
	wg      sync.WaitGroup
}

func New(source Source, destination Destination, logger *slog.Logger, timeout time.Duration, location *time.Location) (*Service, error) {
	if source == nil || destination == nil {
		return nil, errors.New("XHS Spotlight sync source and destination are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if timeout <= 0 {
		return nil, errors.New("XHS Spotlight sync timeout must be positive")
	}
	if location == nil {
		return nil, errors.New("XHS Spotlight sync location is required")
	}
	return &Service{
		source: source, destination: destination, logger: logger,
		timeout: timeout, location: location, now: time.Now, gate: make(chan struct{}, 1),
	}, nil
}

func (service *Service) Trigger(ctx context.Context, target Target, mode Mode, trigger string, advertiserID int64) (store.XHSJGSyncRun, error) {
	if err := validateRequest(target, mode, trigger, advertiserID); err != nil {
		return store.XHSJGSyncRun{}, err
	}
	select {
	case service.gate <- struct{}{}:
	default:
		return store.XHSJGSyncRun{}, ErrAlreadyRunning
	}
	run, err := service.destination.StartXHSJGSyncRun(ctx, string(mode), string(target), trigger, advertiserID)
	if err != nil {
		<-service.gate
		if errors.Is(err, store.ErrXHSJGSyncRunLocked) {
			return store.XHSJGSyncRun{}, ErrAlreadyRunning
		}
		return store.XHSJGSyncRun{}, err
	}
	service.mu.Lock()
	service.current = &run
	service.mu.Unlock()

	service.wg.Add(1)
	go service.execute(ctx, run, target, mode, advertiserID)
	return run, nil
}

func validateRequest(target Target, mode Mode, trigger string, advertiserID int64) error {
	if target != TargetCampaigns && target != TargetUnits && target != TargetCreativities {
		return fmt.Errorf("unsupported XHS Spotlight sync target %q", target)
	}
	if mode != ModeIncremental && mode != ModeFull {
		return fmt.Errorf("unsupported XHS Spotlight sync mode %q", mode)
	}
	if target == TargetCreativities && mode != ModeFull {
		return errors.New("XHS Spotlight creativity sync only supports full mode")
	}
	if trigger != "api" && trigger != "cli" {
		return fmt.Errorf("unsupported XHS Spotlight sync trigger %q", trigger)
	}
	if advertiserID < 0 {
		return errors.New("XHS Spotlight advertiser ID cannot be negative")
	}
	return nil
}

func (service *Service) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		service.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (service *Service) Status(ctx context.Context) (Status, error) {
	service.mu.RLock()
	var current *store.XHSJGSyncRun
	if service.current != nil {
		copy := *service.current
		current = &copy
	}
	service.mu.RUnlock()
	recent, err := service.destination.ListXHSJGSyncRuns(ctx, 10)
	if err != nil {
		return Status{}, err
	}
	return Status{Running: current != nil, Current: current, Recent: recent}, nil
}

func (service *Service) execute(parent context.Context, run store.XHSJGSyncRun, target Target, mode Mode, advertiserID int64) {
	defer service.wg.Done()
	ctx, cancel := context.WithTimeout(parent, service.timeout)
	defer cancel()
	startedAt := service.now()
	result, syncErr := service.run(ctx, target, mode, advertiserID, startedAt)
	status := "succeeded"
	if syncErr != nil {
		status = "failed"
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finishCancel()
	if err := service.destination.FinishXHSJGSyncRun(
		finishCtx, run.RunID, status, result.Advertisers, result.Campaigns,
		result.Units, result.Creativities, result.Deactivated, syncErr,
	); err != nil {
		service.logger.Error("finish XHS Spotlight sync run", "run_id", run.RunID, "error", err)
	}
	service.logger.LogAttrs(context.Background(), slog.LevelInfo, "XHS Spotlight sync finished",
		slog.Int64("run_id", run.RunID), slog.String("target", string(target)),
		slog.String("mode", string(mode)), slog.String("status", status),
		slog.Int("advertisers", result.Advertisers), slog.Int("campaigns", result.Campaigns),
		slog.Int("units", result.Units), slog.Int("creativities", result.Creativities),
		slog.Int64("deactivated", result.Deactivated), slog.Duration("duration", service.now().Sub(startedAt)),
		slog.Any("error", syncErr),
	)
	service.mu.Lock()
	service.current = nil
	service.mu.Unlock()
	<-service.gate
}

func (service *Service) run(ctx context.Context, target Target, mode Mode, requestedID int64, startedAt time.Time) (Result, error) {
	status := service.source.Status()
	if !status.Authorized || !status.AccessTokenValid {
		return Result{}, errors.New("XHS Spotlight does not have a valid access token")
	}
	advertisers, err := selectAdvertisers(status, requestedID)
	if err != nil {
		return Result{}, err
	}
	var result Result
	runErrors := make([]error, 0)
	for _, advertiser := range advertisers {
		var advertiserResult Result
		switch target {
		case TargetCampaigns:
			advertiserResult, err = service.syncCampaigns(ctx, advertiser, mode, startedAt)
		case TargetUnits:
			advertiserResult, err = service.syncUnits(ctx, advertiser, mode, startedAt)
		case TargetCreativities:
			advertiserResult, err = service.syncCreativities(ctx, advertiser, mode)
		default:
			err = fmt.Errorf("unsupported XHS Spotlight sync target %q", target)
		}
		if err != nil {
			runErrors = append(runErrors, fmt.Errorf("sync %s for advertiser %d (%s): %w", target, advertiser.ID, advertiser.Name, err))
			continue
		}
		result.Advertisers++
		result.Campaigns += advertiserResult.Campaigns
		result.Units += advertiserResult.Units
		result.Creativities += advertiserResult.Creativities
		result.Deactivated += advertiserResult.Deactivated
	}
	if len(runErrors) > 0 {
		return result, fmt.Errorf("XHS Spotlight %s %s sync completed with %d advertiser errors: %w", target, mode, len(runErrors), errors.Join(runErrors...))
	}
	return result, nil
}

func (service *Service) syncCampaigns(ctx context.Context, advertiser xhs.Advertiser, mode Mode, startedAt time.Time) (Result, error) {
	campaignStatus := 6
	request := xhs.CampaignListRequest{AdvertiserID: advertiser.ID, Status: &campaignStatus}
	if mode == ModeIncremental {
		startDate, endDate, err := service.incrementalWindow(ctx, advertiser.ID, TargetCampaigns, startedAt)
		if err != nil {
			return Result{}, err
		}
		request.UpdateStartDate = startDate
		request.UpdateEndDate = endDate
	}
	var collection xhs.CampaignCollection
	if err := retry(ctx, func() error {
		var fetchErr error
		collection, fetchErr = service.source.ListAllCampaigns(ctx, request)
		return fetchErr
	}); err != nil {
		return Result{}, fmt.Errorf("fetch campaigns: %w", err)
	}
	var stored store.XHSCampaignStoreResult
	var err error
	if mode == ModeFull {
		stored, err = service.destination.ReplaceXHSCampaignSnapshot(ctx, advertiser, collection.Campaigns)
	} else {
		stored, err = service.destination.UpsertXHSCampaigns(ctx, advertiser, collection.Campaigns)
	}
	if err != nil {
		return Result{}, fmt.Errorf("store campaigns: %w", err)
	}
	if err := service.destination.MarkXHSJGIncrementalSynced(ctx, advertiser.ID, string(TargetCampaigns), startedAt); err != nil {
		return Result{}, err
	}
	return Result{Campaigns: stored.Upserted, Deactivated: stored.Deactivated}, nil
}

func (service *Service) syncUnits(ctx context.Context, advertiser xhs.Advertiser, mode Mode, startedAt time.Time) (Result, error) {
	request := xhs.UnitListRequest{AdvertiserID: advertiser.ID}
	if mode == ModeIncremental {
		startDate, endDate, err := service.incrementalWindow(ctx, advertiser.ID, TargetUnits, startedAt)
		if err != nil {
			return Result{}, err
		}
		request.UpdateStartDate = startDate
		request.UpdateEndDate = endDate
	}
	var collection xhs.UnitCollection
	if err := retry(ctx, func() error {
		var fetchErr error
		collection, fetchErr = service.source.ListAllUnits(ctx, request)
		return fetchErr
	}); err != nil {
		return Result{}, fmt.Errorf("fetch units: %w", err)
	}
	activeUnits := collection.Units[:0]
	for _, unit := range collection.Units {
		if unit.UnitFilterState != 1 {
			activeUnits = append(activeUnits, unit)
		}
	}
	var stored store.XHSEntityStoreResult
	var err error
	if mode == ModeFull {
		stored, err = service.destination.ReplaceXHSUnitSnapshot(ctx, advertiser, activeUnits)
	} else {
		stored, err = service.destination.UpsertXHSUnits(ctx, advertiser, activeUnits)
	}
	if err != nil {
		return Result{}, fmt.Errorf("store units: %w", err)
	}
	if err := service.destination.MarkXHSJGIncrementalSynced(ctx, advertiser.ID, string(TargetUnits), startedAt); err != nil {
		return Result{}, err
	}
	return Result{Units: stored.Upserted, Deactivated: stored.Deactivated}, nil
}

func (service *Service) syncCreativities(ctx context.Context, advertiser xhs.Advertiser, mode Mode) (Result, error) {
	if mode != ModeFull {
		return Result{}, errors.New("XHS Spotlight creativity sync only supports full mode")
	}
	creativeStatus := 2
	var collection xhs.CreativityCollection
	if err := retry(ctx, func() error {
		var fetchErr error
		collection, fetchErr = service.source.ListAllCreativities(ctx, xhs.CreativityListRequest{
			AdvertiserID: advertiser.ID,
			Status:       &creativeStatus,
		})
		return fetchErr
	}); err != nil {
		return Result{}, fmt.Errorf("fetch creativities: %w", err)
	}
	stored, err := service.destination.ReplaceXHSCreativitySnapshot(ctx, advertiser, collection.Creativities)
	if err != nil {
		return Result{}, fmt.Errorf("store creativities: %w", err)
	}
	return Result{Creativities: stored.Upserted, Deactivated: stored.Deactivated}, nil
}

func (service *Service) incrementalWindow(ctx context.Context, advertiserID int64, target Target, startedAt time.Time) (string, string, error) {
	since, err := service.destination.XHSJGIncrementalSince(ctx, advertiserID, string(target))
	if err != nil {
		return "", "", err
	}
	startDate := since.In(service.location).AddDate(0, 0, -1).Format(time.DateOnly)
	endDate := startedAt.In(service.location).Format(time.DateOnly)
	return startDate, endDate, nil
}

func selectAdvertisers(status xhs.ManagerStatus, requestedID int64) ([]xhs.Advertiser, error) {
	advertisers := append([]xhs.Advertiser(nil), status.ApprovalAdvertisers...)
	if len(advertisers) == 0 && status.AdvertiserID > 0 {
		advertisers = append(advertisers, xhs.Advertiser{ID: status.AdvertiserID})
	}
	seen := make(map[int64]struct{}, len(advertisers))
	selected := make([]xhs.Advertiser, 0, len(advertisers))
	for _, advertiser := range advertisers {
		if advertiser.ID <= 0 || requestedID > 0 && advertiser.ID != requestedID {
			continue
		}
		if _, exists := seen[advertiser.ID]; exists {
			continue
		}
		seen[advertiser.ID] = struct{}{}
		selected = append(selected, advertiser)
	}
	if len(selected) == 0 {
		if requestedID > 0 {
			return nil, fmt.Errorf("advertiser %d is not present in the OAuth authorization", requestedID)
		}
		return nil, errors.New("OAuth authorization contains no advertisers")
	}
	sort.Slice(selected, func(left, right int) bool { return selected[left].ID < selected[right].ID })
	return selected, nil
}

func retry(ctx context.Context, operation func() error) error {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		err = operation()
		if err == nil {
			return nil
		}
		if attempt == 3 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("failed after 3 attempts: %w", err)
}
