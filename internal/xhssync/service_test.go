package xhssync

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/store"
	"paipai-red-campaign-manager/internal/xhs"
)

type fakeSource struct {
	status            xhs.ManagerStatus
	campaignRequest   *xhs.CampaignListRequest
	unitRequest       *xhs.UnitListRequest
	creativityRequest *xhs.CreativityListRequest
}

func (source *fakeSource) Status() xhs.ManagerStatus { return source.status }

func (source *fakeSource) ListAllCampaigns(_ context.Context, request xhs.CampaignListRequest) (xhs.CampaignCollection, error) {
	source.campaignRequest = &request
	return xhs.CampaignCollection{TotalCount: 1, Campaigns: []xhs.Campaign{{CampaignID: 10}}}, nil
}

func (source *fakeSource) ListAllUnits(_ context.Context, request xhs.UnitListRequest) (xhs.UnitCollection, error) {
	source.unitRequest = &request
	return xhs.UnitCollection{TotalCount: 2, Units: []xhs.Unit{
		{UnitID: 20, CampaignID: 10, UnitFilterState: 10},
		{UnitID: 21, CampaignID: 10, UnitFilterState: 1},
	}}, nil
}

func (source *fakeSource) ListAllCreativities(_ context.Context, request xhs.CreativityListRequest) (xhs.CreativityCollection, error) {
	source.creativityRequest = &request
	return xhs.CreativityCollection{TotalCount: 1, Creativities: []xhs.Creativity{{CreativityID: 30, CampaignID: 10, UnitID: 20}}}, nil
}

type fakeDestination struct {
	since                time.Time
	cursorTarget         string
	markedTarget         string
	markedAt             time.Time
	replacedCampaigns    bool
	upsertedCampaigns    bool
	replacedUnits        bool
	upsertedUnits        bool
	replacedCreativities bool
	storedUnits          []xhs.Unit
}

func (destination *fakeDestination) StartXHSJGSyncRun(_ context.Context, mode, target, trigger string, _ int64) (store.XHSJGSyncRun, error) {
	return store.XHSJGSyncRun{RunID: 1, Mode: mode, Target: target, TriggerType: trigger}, nil
}

func (destination *fakeDestination) FinishXHSJGSyncRun(context.Context, int64, string, int, int, int, int, int64, error) error {
	return nil
}

func (destination *fakeDestination) ListXHSJGSyncRuns(context.Context, int) ([]store.XHSJGSyncRun, error) {
	return nil, nil
}

func (destination *fakeDestination) XHSJGIncrementalSince(_ context.Context, _ int64, target string) (time.Time, error) {
	destination.cursorTarget = target
	return destination.since, nil
}

func (destination *fakeDestination) MarkXHSJGIncrementalSynced(_ context.Context, _ int64, target string, syncedAt time.Time) error {
	destination.markedTarget = target
	destination.markedAt = syncedAt
	return nil
}

func (destination *fakeDestination) ReplaceXHSCampaignSnapshot(_ context.Context, _ xhs.Advertiser, campaigns []xhs.Campaign) (store.XHSCampaignStoreResult, error) {
	destination.replacedCampaigns = true
	return store.XHSCampaignStoreResult{Upserted: len(campaigns)}, nil
}

func (destination *fakeDestination) UpsertXHSCampaigns(_ context.Context, _ xhs.Advertiser, campaigns []xhs.Campaign) (store.XHSCampaignStoreResult, error) {
	destination.upsertedCampaigns = true
	return store.XHSCampaignStoreResult{Upserted: len(campaigns)}, nil
}

func (destination *fakeDestination) ReplaceXHSUnitSnapshot(_ context.Context, _ xhs.Advertiser, units []xhs.Unit) (store.XHSEntityStoreResult, error) {
	destination.replacedUnits = true
	destination.storedUnits = units
	return store.XHSEntityStoreResult{Upserted: len(units)}, nil
}

func (destination *fakeDestination) UpsertXHSUnits(_ context.Context, _ xhs.Advertiser, units []xhs.Unit) (store.XHSEntityStoreResult, error) {
	destination.upsertedUnits = true
	destination.storedUnits = units
	return store.XHSEntityStoreResult{Upserted: len(units)}, nil
}

func (destination *fakeDestination) ReplaceXHSCreativitySnapshot(_ context.Context, _ xhs.Advertiser, creativities []xhs.Creativity) (store.XHSEntityStoreResult, error) {
	destination.replacedCreativities = true
	return store.XHSEntityStoreResult{Upserted: len(creativities)}, nil
}

func newTestService(t *testing.T, source *fakeSource, destination *fakeDestination, location *time.Location) *Service {
	t.Helper()
	service, err := New(source, destination, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Minute, location)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func authorizedSource() *fakeSource {
	return &fakeSource{status: xhs.ManagerStatus{
		Authorized: true, AccessTokenValid: true,
		ApprovalAdvertisers: []xhs.Advertiser{{ID: 123, Name: "test"}},
	}}
}

func TestCampaignIncrementalRunOnlyRefreshesCampaigns(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	startedAt := time.Date(2026, 7, 23, 10, 0, 0, 0, location)
	source := authorizedSource()
	destination := &fakeDestination{since: time.Date(2026, 7, 22, 12, 0, 0, 0, location)}
	service := newTestService(t, source, destination, location)

	result, err := service.run(context.Background(), TargetCampaigns, ModeIncremental, 0, startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if source.campaignRequest == nil || source.campaignRequest.UpdateStartDate != "2026-07-21" || source.campaignRequest.UpdateEndDate != "2026-07-23" {
		t.Fatalf("campaign request = %+v", source.campaignRequest)
	}
	if source.unitRequest != nil || source.creativityRequest != nil {
		t.Fatalf("unrelated upstream calls: unit=%+v creativity=%+v", source.unitRequest, source.creativityRequest)
	}
	if !destination.upsertedCampaigns || destination.replacedCampaigns || destination.replacedUnits || destination.replacedCreativities {
		t.Fatalf("campaign storage = %+v", destination)
	}
	if destination.cursorTarget != "campaigns" || destination.markedTarget != "campaigns" || !destination.markedAt.Equal(startedAt) {
		t.Fatalf("campaign cursor target=%q marked=%q at=%v", destination.cursorTarget, destination.markedTarget, destination.markedAt)
	}
	if result.Advertisers != 1 || result.Campaigns != 1 || result.Units != 0 || result.Creativities != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestUnitIncrementalRunOnlyRefreshesUnits(t *testing.T) {
	location := time.UTC
	startedAt := time.Date(2026, 7, 23, 2, 0, 0, 0, location)
	source := authorizedSource()
	destination := &fakeDestination{since: time.Date(2026, 7, 22, 0, 0, 0, 0, location)}
	service := newTestService(t, source, destination, location)

	result, err := service.run(context.Background(), TargetUnits, ModeIncremental, 0, startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if source.unitRequest == nil || source.unitRequest.UpdateStartDate != "2026-07-21" || source.unitRequest.UpdateEndDate != "2026-07-23" {
		t.Fatalf("unit request = %+v", source.unitRequest)
	}
	if source.campaignRequest != nil || source.creativityRequest != nil {
		t.Fatalf("unrelated upstream calls: campaign=%+v creativity=%+v", source.campaignRequest, source.creativityRequest)
	}
	if !destination.upsertedUnits || destination.replacedUnits || len(destination.storedUnits) != 1 || destination.storedUnits[0].UnitID != 20 {
		t.Fatalf("unit storage = %+v", destination)
	}
	if destination.cursorTarget != "units" || destination.markedTarget != "units" {
		t.Fatalf("unit cursor target=%q marked=%q", destination.cursorTarget, destination.markedTarget)
	}
	if result.Advertisers != 1 || result.Units != 1 || result.Campaigns != 0 || result.Creativities != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestCreativityFullRunOnlyRefreshesCreativities(t *testing.T) {
	source := authorizedSource()
	destination := &fakeDestination{}
	service := newTestService(t, source, destination, time.UTC)

	result, err := service.run(context.Background(), TargetCreativities, ModeFull, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if source.creativityRequest == nil || source.creativityRequest.Status == nil || *source.creativityRequest.Status != 2 {
		t.Fatalf("creativity request = %+v", source.creativityRequest)
	}
	if source.campaignRequest != nil || source.unitRequest != nil {
		t.Fatalf("unrelated upstream calls: campaign=%+v unit=%+v", source.campaignRequest, source.unitRequest)
	}
	if !destination.replacedCreativities || destination.replacedCampaigns || destination.replacedUnits {
		t.Fatalf("creativity storage = %+v", destination)
	}
	if result.Advertisers != 1 || result.Creativities != 1 || result.Campaigns != 0 || result.Units != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestCreativityIncrementalIsRejected(t *testing.T) {
	if err := validateRequest(TargetCreativities, ModeIncremental, "api", 0); err == nil {
		t.Fatal("creativity incremental request was accepted")
	}
}
