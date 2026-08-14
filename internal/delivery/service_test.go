package delivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type serviceTestStore struct {
	draft     Draft
	approvals []Approval
	jobs      map[string]PublishJob
	jobKeys   map[string]string
	entities  []MediaEntity
	attempts  []APIAttempt
	audits    int
}

func (store *serviceTestStore) CreateDraft(context.Context, CreateDraftInput, Actor) (Draft, error) {
	return Draft{}, errors.New("not implemented")
}
func (store *serviceTestStore) UpdateDraft(context.Context, string, UpdateDraftInput, Actor) (Draft, error) {
	return Draft{}, errors.New("not implemented")
}
func (store *serviceTestStore) Draft(_ context.Context, id string) (Draft, error) {
	if store.draft.ID != id {
		return Draft{}, ErrNotFound
	}
	return store.draft, nil
}
func (store *serviceTestStore) Drafts(context.Context, int64, int) ([]Draft, error) {
	return []Draft{store.draft}, nil
}
func (store *serviceTestStore) SaveRecommendation(_ context.Context, value Recommendation) (Recommendation, error) {
	return value, nil
}
func (store *serviceTestStore) LatestRecommendation(context.Context, string, int) (Recommendation, error) {
	return Recommendation{}, ErrNotFound
}
func (store *serviceTestStore) SaveValidation(_ context.Context, value Validation) (Validation, error) {
	if value.Valid && store.draft.Status != "approved" {
		store.draft.Status = "validated"
	}
	return value, nil
}
func (store *serviceTestStore) LatestValidation(context.Context, string, int) (Validation, error) {
	return Validation{}, ErrNotFound
}
func (store *serviceTestStore) SaveApproval(_ context.Context, value Approval) (Approval, error) {
	store.approvals = append(store.approvals, value)
	return value, nil
}
func (store *serviceTestStore) Approvals(context.Context, string, int) ([]Approval, error) {
	return append([]Approval(nil), store.approvals...), nil
}
func (store *serviceTestStore) CreatePublishJob(_ context.Context, value PublishJob) (PublishJob, error) {
	if store.jobs == nil {
		store.jobs = map[string]PublishJob{}
	}
	if store.jobKeys == nil {
		store.jobKeys = map[string]string{}
	}
	if existingID := store.jobKeys[value.IdempotencyKey]; existingID != "" {
		return store.jobs[existingID], nil
	}
	store.jobs[value.ID] = value
	store.jobKeys[value.IdempotencyKey] = value.ID
	return value, nil
}
func (store *serviceTestStore) PublishJobByIdempotency(_ context.Context, key string) (PublishJob, error) {
	if id := store.jobKeys[key]; id != "" {
		return store.jobs[id], nil
	}
	return PublishJob{}, ErrNotFound
}
func (store *serviceTestStore) PublishJob(_ context.Context, id string) (PublishJob, error) {
	job, ok := store.jobs[id]
	if !ok {
		return PublishJob{}, ErrNotFound
	}
	return job, nil
}
func (store *serviceTestStore) PublishJobs(context.Context, string, int, int) ([]PublishJob, error) {
	result := make([]PublishJob, 0, len(store.jobs))
	for _, job := range store.jobs {
		result = append(result, job)
	}
	return result, nil
}
func (store *serviceTestStore) ClaimPublishJob(context.Context) (PublishJob, bool, error) {
	for id, job := range store.jobs {
		if job.Mode == "execute" && job.Status == "queued" {
			now := time.Now().UTC()
			job.Status = "publishing"
			job.CurrentStep = "claimed"
			job.StartedAt = &now
			store.jobs[id] = job
			return job, true, nil
		}
	}
	return PublishJob{}, false, nil
}
func (store *serviceTestStore) UpdatePublishJob(_ context.Context, value PublishJob) error {
	store.jobs[value.ID] = value
	return nil
}
func (store *serviceTestStore) SaveMediaEntity(_ context.Context, value MediaEntity) (MediaEntity, error) {
	store.entities = append(store.entities, value)
	return value, nil
}
func (store *serviceTestStore) MediaEntity(_ context.Context, advertiserID int64, entityType string, mediaID int64) (MediaEntity, error) {
	for _, entity := range store.entities {
		if entity.AdvertiserID == advertiserID && entity.EntityType == entityType && entity.MediaID == mediaID {
			return entity, nil
		}
	}
	return MediaEntity{}, ErrNotFound
}
func (store *serviceTestStore) MediaEntities(context.Context, string) ([]MediaEntity, error) {
	return append([]MediaEntity(nil), store.entities...), nil
}
func (store *serviceTestStore) UpdateMediaEntityStatus(_ context.Context, entityID string, status string) error {
	for index := range store.entities {
		if store.entities[index].ID == entityID {
			store.entities[index].DesiredStatus = status
			store.entities[index].ObservedStatus = status
			return nil
		}
	}
	return ErrNotFound
}
func (store *serviceTestStore) SaveAPIAttempt(_ context.Context, value APIAttempt) error {
	store.attempts = append(store.attempts, value)
	return nil
}
func (store *serviceTestStore) SavePerformanceSnapshot(context.Context, PerformanceQuery, map[string]any, string) error {
	return nil
}
func (store *serviceTestStore) Assets(context.Context, AssetQuery) (Assets, error) {
	return Assets{}, nil
}
func (store *serviceTestStore) RecommendationCandidates(context.Context, []string) ([]CandidateNote, error) {
	return nil, nil
}
func (store *serviceTestStore) Audit(context.Context, Actor, string, string, string, int64, map[string]any) error {
	store.audits++
	return nil
}

type gatewayCall struct {
	operation string
	payload   map[string]any
}

type serviceTestGateway struct {
	calls       []gatewayCall
	write       bool
	failOn      string
	readbackBad bool
	readbackOn  bool
	reportData  map[string]any
}

func (gateway *serviceTestGateway) Advertisers(context.Context) ([]Advertiser, error) {
	return []Advertiser{{ID: 1234, Name: "测试广告主"}}, nil
}

func (gateway *serviceTestGateway) Capabilities(_ context.Context, advertiserID int64) (Capability, error) {
	return Capability{
		AdvertiserID: advertiserID, Authorized: true, AdvertiserAllowed: true,
		Scopes:             []string{"ad_manage", "ad_query", "report_service", "account_manage"},
		RequiredScopes:     []string{"ad_manage", "ad_query", "report_service", "account_manage"},
		MediaWritesEnabled: gateway.write, ContractVersion: MediaContractVersion,
		Operations: map[string]any{}, CheckedAt: time.Now(),
	}, nil
}

func (gateway *serviceTestGateway) Call(_ context.Context, operation string, payload map[string]any) (GatewayResponse, error) {
	copyPayload := map[string]any{}
	for key, value := range payload {
		copyPayload[key] = value
	}
	gateway.calls = append(gateway.calls, gatewayCall{operation: operation, payload: copyPayload})
	if operation == gateway.failOn {
		return GatewayResponse{}, fmt.Errorf("forced %s failure", operation)
	}
	data := map[string]any{}
	switch operation {
	case "account.balance":
		data = map[string]any{"available_balance": float64(1_000_000)}
	case "account.white_list":
		data = map[string]any{"in_note_force_bind_spu_white_list": false}
	case "delivery.name_check":
		data = map[string]any{"check_result": map[string]any{}}
	case "target.audience_estimate":
		data = map[string]any{"crowd_scope": float64(2), "raw_crowd_num": float64(100_000)}
	case "campaign.create":
		data = map[string]any{"campaign_id": float64(101)}
	case "unit.create":
		data = map[string]any{"unit_id": float64(201)}
	case "keyword.negative_add":
		data = map[string]any{}
	case "creativity.create":
		data = map[string]any{"creativity_id": float64(301)}
	case "campaign.list":
		enable := float64(0)
		if gateway.readbackOn {
			enable = 1
		}
		data = map[string]any{"base_campaign_dtos": []any{map[string]any{
			"campaign_id": float64(101), "campaign_enable": enable,
			"campaign_name": "测试计划", "campaign_day_budget": float64(30_000),
		}}}
	case "unit.list":
		data = map[string]any{"unit_infos": []any{map[string]any{
			"id": float64(201), "campaign_id": float64(101), "enable": float64(0),
			"name": "测试单元", "event_bid": float64(1_000),
		}}}
	case "creativity.search":
		id := float64(301)
		if gateway.readbackBad {
			id = 999
		}
		data = map[string]any{"creativity_dtos": []any{map[string]any{
			"creativity_id": id, "unit_id": float64(201), "creativity_enable": float64(0),
			"creativity_name": "测试创意", "note_id": "0123456789abcdef01234567",
		}}}
	case "campaign.status":
		data = map[string]any{}
	case "report.realtime.account":
		data = gateway.reportData
	default:
		return GatewayResponse{}, fmt.Errorf("unexpected operation %s", operation)
	}
	return GatewayResponse{Operation: operation, Data: data, RequestHash: fmt.Sprintf("%064d", len(gateway.calls)), LatencyMS: 1}, nil
}

func TestPublishExecutesPausedSagaAndReadback(t *testing.T) {
	service, store, gateway := newPublishTestService(t, false)
	job, err := service.Publish(context.Background(), store.draft.ID, PublishInput{Mode: "execute", IdempotencyKey: "publish-idempotency-1"}, Actor{ID: "operator-a", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "queued" || len(gateway.calls) != 5 {
		t.Fatalf("publish did not stop at the queue boundary: job=%+v calls=%v", job, gateway.calls)
	}
	processed, err := service.ProcessNextPublishJob(context.Background())
	if err != nil || !processed {
		t.Fatalf("ProcessNextPublishJob() processed=%v error=%v", processed, err)
	}
	job, err = service.Job(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "succeeded" || job.CurrentStep != "complete_paused" || job.Result["readback_verified"] != true {
		t.Fatalf("unexpected publish job: %+v", job)
	}
	operations := make([]string, len(gateway.calls))
	for index, call := range gateway.calls {
		operations[index] = call.operation
		if call.operation == "campaign.create" && numericInt64(call.payload["enable"]) != 0 {
			t.Fatalf("campaign create was not paused: %+v", call.payload)
		}
	}
	want := []string{
		"account.balance", "account.white_list", "delivery.name_check", "delivery.name_check",
		"target.audience_estimate", "campaign.create", "unit.create", "keyword.negative_add",
		"creativity.create", "campaign.list", "unit.list", "creativity.search",
	}
	if fmt.Sprint(operations) != fmt.Sprint(want) {
		t.Fatalf("operations = %v, want %v", operations, want)
	}
	if len(store.entities) != 3 || len(store.attempts) != len(gateway.calls) {
		t.Fatalf("entities=%d attempts=%d calls=%d", len(store.entities), len(store.attempts), len(gateway.calls))
	}
}

func TestPublishReadbackMismatchFailsWithPausedRecovery(t *testing.T) {
	service, store, gateway := newPublishTestService(t, true)
	job, err := service.Publish(context.Background(), store.draft.ID, PublishInput{Mode: "execute", IdempotencyKey: "publish-idempotency-2"}, Actor{ID: "operator-a", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	processed, processErr := service.ProcessNextPublishJob(context.Background())
	if processErr != nil || !processed {
		t.Fatalf("ProcessNextPublishJob() processed=%v error=%v", processed, processErr)
	}
	job, err = service.Job(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "failed" || job.ErrorCode != "readback_mismatch" || job.Result["partial_failure"] != true {
		t.Fatalf("unexpected failed job: %+v", job)
	}
	if gateway.calls[len(gateway.calls)-1].operation != "creativity.search" {
		t.Fatalf("last operation = %+v", gateway.calls[len(gateway.calls)-1])
	}
}

func TestPublishReadbackRejectsUnexpectedActivation(t *testing.T) {
	service, store, gateway := newPublishTestService(t, false)
	gateway.readbackOn = true
	job, err := service.Publish(context.Background(), store.draft.ID, PublishInput{Mode: "execute", IdempotencyKey: "publish-idempotency-active-readback"}, Actor{ID: "operator-a", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	processed, processErr := service.ProcessNextPublishJob(context.Background())
	if processErr != nil || !processed {
		t.Fatalf("ProcessNextPublishJob() processed=%v error=%v", processed, processErr)
	}
	job, err = service.Job(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "failed" || job.ErrorCode != "readback_mismatch" || !strings.Contains(job.ErrorMessage, "unexpectedly active") {
		t.Fatalf("unexpected job: %+v", job)
	}
}

func TestPublisherRechecksExpiredApprovalsBeforeMediaWrites(t *testing.T) {
	service, store, gateway := newPublishTestService(t, false)
	job, err := service.Publish(context.Background(), store.draft.ID, PublishInput{Mode: "execute", IdempotencyKey: "publish-idempotency-expiry"}, Actor{ID: "operator-a", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	for index := range store.approvals {
		store.approvals[index].ExpiresAt = service.now().Add(-time.Minute)
	}
	processed, processErr := service.ProcessNextPublishJob(context.Background())
	if processErr != nil || !processed {
		t.Fatalf("ProcessNextPublishJob() processed=%v error=%v", processed, processErr)
	}
	job, err = service.Job(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "failed" || job.ErrorCode != "approval_expired" || len(gateway.calls) != 5 {
		t.Fatalf("job=%+v calls=%v", job, gateway.calls)
	}
	if job.Result["executed"] != false || job.Result["partial_failure"] != false {
		t.Fatalf("preflight failure was marked as a partial media write: %+v", job.Result)
	}
}

func TestActivationRequiresSuccessfulPausedReadback(t *testing.T) {
	service, store, gateway := newPublishTestService(t, false)
	jobID := "job_3123456789abcdef0123456789abcdef"
	store.entities = append(store.entities, MediaEntity{
		ID: "ent_0123456789abcdef0123456789abcdef", JobID: jobID, DraftID: store.draft.ID,
		AdvertiserID: store.draft.AdvertiserID, EntityType: "campaign", MediaID: 101,
		DesiredStatus: "paused", ObservedStatus: "paused",
	})
	store.jobs[jobID] = PublishJob{
		ID: jobID, DraftID: store.draft.ID, DraftVersion: store.draft.CurrentVersion,
		AdvertiserID: store.draft.AdvertiserID, Mode: "execute", Status: "failed",
		CurrentStep: "readback_mismatch", Result: map[string]any{"readback_verified": false},
	}

	_, err := service.UpdateEntityStatus(context.Background(), store.draft.AdvertiserID, "campaign", 101, "active", Actor{ID: "operator-a", Role: "operator"})
	if !errors.Is(err, ErrConflict) || len(gateway.calls) != 0 {
		t.Fatalf("UpdateEntityStatus() error=%v calls=%v", err, gateway.calls)
	}

	job := store.jobs[jobID]
	job.Status = "succeeded"
	job.CurrentStep = "complete_paused"
	job.Result["readback_verified"] = true
	store.jobs[jobID] = job
	_, err = service.UpdateEntityStatus(context.Background(), store.draft.AdvertiserID, "campaign", 101, "active", Actor{ID: "operator-a", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if len(gateway.calls) != 1 || gateway.calls[0].operation != "campaign.status" || store.entities[0].ObservedStatus != "active" {
		t.Fatalf("calls=%v entity=%+v", gateway.calls, store.entities[0])
	}
}

func TestPerformanceUsesRealtimeAccountContractShape(t *testing.T) {
	service, store, gateway := newPublishTestService(t, false)
	gateway.reportData = map[string]any{"spend": float64(123)}
	result, err := service.Performance(context.Background(), PerformanceQuery{
		AdvertiserID: store.draft.AdvertiserID, Level: "account", Realtime: true,
		StartDate: "2026-08-12", EndDate: "2026-08-13", Page: 3, PageSize: 20,
	}, Actor{ID: "reader", Role: "viewer"})
	if err != nil {
		t.Fatal(err)
	}
	if result["spend"] != float64(123) {
		t.Fatalf("result = %+v", result)
	}
	call := gateway.calls[len(gateway.calls)-1]
	if call.operation != "report.realtime.account" || call.payload["page_num"] != nil || call.payload["split_columns"] != nil {
		t.Fatalf("report call = %+v", call)
	}
}

func TestPerformanceRejectsReservedFilter(t *testing.T) {
	service, store, _ := newPublishTestService(t, false)
	_, err := service.Performance(context.Background(), PerformanceQuery{
		AdvertiserID: store.draft.AdvertiserID, Level: "campaign",
		StartDate: "2026-08-12", EndDate: "2026-08-13",
		Filters: map[string]any{"advertiser_id": float64(999)},
	}, Actor{ID: "reader", Role: "viewer"})
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("Performance() error = %v", err)
	}
}

func TestPublishExecuteRequiresTwoDistinctApprovers(t *testing.T) {
	service, store, _ := newPublishTestService(t, false)
	store.approvals[1].Actor = store.approvals[0].Actor
	_, err := service.Publish(context.Background(), store.draft.ID, PublishInput{Mode: "execute", IdempotencyKey: "publish-idempotency-3"}, Actor{ID: "operator-a", Role: "operator"})
	if !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("Publish() error = %v, want approval required", err)
	}
}

func TestPublishExecuteRespectsGatewayWriteCapability(t *testing.T) {
	service, store, gateway := newPublishTestService(t, false)
	gateway.write = false
	_, err := service.Publish(context.Background(), store.draft.ID, PublishInput{Mode: "execute", IdempotencyKey: "publish-idempotency-4"}, Actor{ID: "operator-a", Role: "operator"})
	if !errors.Is(err, ErrWritesDisabled) {
		t.Fatalf("Publish() error = %v, want writes disabled", err)
	}
}

func TestPublishReturnsExistingQueuedJobWithoutExecuting(t *testing.T) {
	service, store, gateway := newPublishTestService(t, false)
	existing := PublishJob{
		ID: "job_0123456789abcdef0123456789abcdef", DraftID: store.draft.ID,
		DraftVersion: store.draft.CurrentVersion, AdvertiserID: store.draft.AdvertiserID,
		Mode: "execute", Status: "queued", CurrentStep: "queued", IdempotencyKey: "publish-idempotency-existing",
	}
	store.jobs[existing.ID] = existing
	store.jobKeys[existing.IdempotencyKey] = existing.ID

	job, err := service.Publish(context.Background(), store.draft.ID, PublishInput{Mode: "execute", IdempotencyKey: existing.IdempotencyKey}, Actor{ID: "operator-a", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != existing.ID || len(gateway.calls) != 0 {
		t.Fatalf("job=%+v gateway calls=%v", job, gateway.calls)
	}
}

func TestPublishRejectsIdempotencyKeyFromOlderDraftVersion(t *testing.T) {
	service, store, gateway := newPublishTestService(t, false)
	existing := PublishJob{
		ID: "job_1123456789abcdef0123456789abcdef", DraftID: store.draft.ID,
		DraftVersion: store.draft.CurrentVersion - 1, AdvertiserID: store.draft.AdvertiserID,
		Mode: "execute", Status: "succeeded", CurrentStep: "complete_paused", IdempotencyKey: "publish-idempotency-old-version",
	}
	store.jobs[existing.ID] = existing
	store.jobKeys[existing.IdempotencyKey] = existing.ID

	_, err := service.Publish(context.Background(), store.draft.ID, PublishInput{Mode: "execute", IdempotencyKey: existing.IdempotencyKey}, Actor{ID: "operator-a", Role: "operator"})
	if !errors.Is(err, ErrConflict) || len(gateway.calls) != 0 {
		t.Fatalf("Publish() error=%v calls=%v", err, gateway.calls)
	}
}

func TestApproveRejectsDraftSubmitter(t *testing.T) {
	service, store, _ := newPublishTestService(t, false)
	store.draft.Status = "validated"
	store.draft.UpdatedBy = "operator-a"
	_, err := service.Approve(context.Background(), store.draft.ID, ApprovalInput{
		Role: "operator", Decision: "approved", ApprovedBudgetFen: store.draft.Spec.Budget.TotalLimitFen,
	}, Actor{ID: "operator-a", Role: "operator"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Approve() error = %v, want forbidden", err)
	}
}

func TestValidateApprovalsUsesLatestDecisionForEachRole(t *testing.T) {
	service, store, _ := newPublishTestService(t, false)
	now := service.now()
	store.approvals[0].ID = "apr_0123456789abcdef0123456789abcdef"
	store.approvals[0].CreatedAt = now.Add(-2 * time.Minute)
	store.approvals[1].ID = "apr_1123456789abcdef0123456789abcdef"
	store.approvals[1].CreatedAt = now.Add(-2 * time.Minute)
	store.approvals = append(store.approvals, Approval{
		ID: "apr_2123456789abcdef0123456789abcdef", DraftID: store.draft.ID,
		DraftVersion: store.draft.CurrentVersion, SpecHash: store.draft.SpecHash,
		Role: "operator", Decision: "rejected", Actor: "operator-c",
		ApprovedBudgetFen: store.draft.Spec.Budget.TotalLimitFen,
		ExpiresAt:         now.Add(time.Hour), CreatedAt: now.Add(-time.Minute),
	})

	err := validateApprovals(store.draft, store.approvals, now)
	if !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("validateApprovals() error = %v, want approval required", err)
	}
}

func TestPlatformToolRejectsWriteOperation(t *testing.T) {
	service, _, gateway := newPublishTestService(t, false)
	_, err := service.PlatformTool(context.Background(), "keyword.negative_add", 1234, map[string]any{}, Actor{ID: "operator-a", Role: "operator"})
	if !errors.Is(err, ErrForbidden) || len(gateway.calls) != 0 {
		t.Fatalf("PlatformTool() error=%v calls=%v", err, gateway.calls)
	}
}

func TestPublishPayloadUsesContractFieldNames(t *testing.T) {
	spec := validTestDraftSpec()
	spec.Campaign.TimePeriodType = 1
	spec.Units[0].Keywords[0].KeywordSource = 11
	spec.Units[0].Keywords[0].FeedBidFen = 300
	draft := Draft{AdvertiserID: spec.AdvertiserID, Spec: spec}
	campaign := campaignPayload(draft)
	if campaign["time_period_type"] != 1 || campaign["time_peroid_type"] != nil {
		t.Fatalf("campaign payload = %+v", campaign)
	}
	unit := unitPayload(draft, spec.Units[0], 42)
	keywords, ok := unit["keyword_with_bid"].([]map[string]any)
	if !ok || len(keywords) != 1 {
		t.Fatalf("unit keyword payload = %#v", unit["keyword_with_bid"])
	}
	if keywords[0]["keyword_source"] != 11 || keywords[0]["phrase_match_type"] != 0 || keywords[0]["feed_bid"] != int64(300) {
		t.Fatalf("keyword payload = %+v", keywords[0])
	}
	target, ok := unit["target_config"].(map[string]any)
	if !ok || target["intelligent_expansion"] != 0 || target["intelligent_expension"] != nil {
		t.Fatalf("target payload = %#v", unit["target_config"])
	}
}

func newPublishTestService(t *testing.T, badReadback bool) (*Service, *serviceTestStore, *serviceTestGateway) {
	t.Helper()
	spec := validTestDraftSpec()
	spec.Units[0].NegativeKeywords = []NegativeKeyword{{Keyword: "批发"}}
	hash, _, err := HashSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	store := &serviceTestStore{
		draft: Draft{ID: "drf_0123456789abcdef0123456789abcdef", AdvertiserID: spec.AdvertiserID, Status: "approved", CurrentVersion: 1, Spec: spec, SpecHash: hash},
		approvals: []Approval{
			{DraftID: "drf_0123456789abcdef0123456789abcdef", DraftVersion: 1, SpecHash: hash, Role: "operator", Decision: "approved", Actor: "operator-a", ApprovedBudgetFen: spec.Budget.TotalLimitFen, ExpiresAt: now.Add(time.Hour)},
			{DraftID: "drf_0123456789abcdef0123456789abcdef", DraftVersion: 1, SpecHash: hash, Role: "budget_owner", Decision: "approved", Actor: "budget-b", ApprovedBudgetFen: spec.Budget.TotalLimitFen, ExpiresAt: now.Add(time.Hour)},
		},
		jobs:    map[string]PublishJob{},
		jobKeys: map[string]string{},
	}
	gateway := &serviceTestGateway{write: true, readbackBad: badReadback}
	service, err := NewService(store, gateway, RuleSemanticAdvisor{}, HeuristicRanker{}, true)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	return service, store, gateway
}
