package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	store              Store
	gateway            Gateway
	semanticAdvisor    SemanticAdvisor
	ranker             Ranker
	mediaWritesEnabled bool
	now                func() time.Time
	publishWake        chan struct{}
}

func NewService(store Store, gateway Gateway, advisor SemanticAdvisor, ranker Ranker, mediaWritesEnabled bool) (*Service, error) {
	if store == nil || gateway == nil || advisor == nil || ranker == nil {
		return nil, errors.New("delivery store, gateway, semantic advisor, and ranker are required")
	}
	return &Service{
		store: store, gateway: gateway, semanticAdvisor: advisor, ranker: ranker,
		mediaWritesEnabled: mediaWritesEnabled, now: time.Now, publishWake: make(chan struct{}, 1),
	}, nil
}

func (service *Service) IntelligenceCapabilities() map[string]any {
	return map[string]any{
		"llm":       service.semanticAdvisor.Metadata(),
		"ranker":    service.ranker.Metadata(),
		"bayesian":  map[string]any{"method": "beta-binomial-normal-approximation/v1", "configured": true, "role": "uncertainty and sparse-segment shrinkage"},
		"optimizer": map[string]any{"method": "constrained-greedy-marginal-value/v1", "configured": true, "role": "budget allocation under hard caps", "executable": false},
		"bandit":    map[string]any{"method": "contextual-ucb-shadow/v1", "configured": true, "role": "shadow recommendation after measurement maturity", "shadow_only": true},
		"responsibility_boundary": map[string]any{
			"llm":                  "semantic extraction, candidate keywords, and evidence summaries only",
			"lightgbm_lambdamart":  "ranking over approved numeric features only",
			"bayesian":             "uncertainty intervals and shrinkage for sparse segments",
			"constraint_optimizer": "allocation suggestions inside operator-approved caps",
			"bandit":               "shadow suggestions only; never activates or changes media state",
			"rules":                "permissions, platform enums, budget caps, approvals, and safety checks",
			"human":                "final targeting, budget, publish, activation, and stop-loss decisions",
		},
	}
}

func (service *Service) Capabilities(ctx context.Context, advertiserID int64) (Capability, error) {
	capability, err := service.gateway.Capabilities(ctx, advertiserID)
	if err != nil {
		return Capability{}, err
	}
	capability.MediaWritesEnabled = capability.MediaWritesEnabled && service.mediaWritesEnabled
	return capability, nil
}

func (service *Service) Advertisers(ctx context.Context) ([]Advertiser, error) {
	return service.gateway.Advertisers(ctx)
}

func (service *Service) Assets(ctx context.Context, query AssetQuery) (Assets, error) {
	if query.AdvertiserID <= 0 {
		return Assets{}, errors.New("advertiser_id must be positive")
	}
	return service.store.Assets(ctx, query)
}

func (service *Service) PlatformTool(ctx context.Context, operation string, advertiserID int64, payload map[string]any, actor Actor) (GatewayResponse, error) {
	if advertiserID <= 0 {
		return GatewayResponse{}, errors.New("advertiser_id must be positive")
	}
	if !readOnlyPlatformOperations[operation] {
		return GatewayResponse{}, fmt.Errorf("%w: platform tool only permits allowlisted read operations", ErrForbidden)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["advertiser_id"] = advertiserID
	response, err := service.callGateway(ctx, "", advertiserID, operation, payload)
	detail := map[string]any{"operation": operation, "success": err == nil}
	if auditErr := service.store.Audit(ctx, actor, "platform_tool", "advertiser", strconv.FormatInt(advertiserID, 10), advertiserID, detail); auditErr != nil && err == nil {
		return GatewayResponse{}, auditErr
	}
	return response, err
}

func (service *Service) CreateDraft(ctx context.Context, input CreateDraftInput, actor Actor) (Draft, error) {
	input.Spec = NormalizeSpec(input.Spec)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 160 {
		return Draft{}, errors.New("idempotency_key must contain 8 to 160 characters")
	}
	if input.Spec.AdvertiserID <= 0 {
		return Draft{}, errors.New("advertiser_id must be positive")
	}
	result, err := service.store.CreateDraft(ctx, input, actor)
	if err != nil {
		return Draft{}, err
	}
	if err := service.store.Audit(ctx, actor, "create", "delivery_draft", result.ID, result.AdvertiserID, map[string]any{"version": result.CurrentVersion, "spec_hash": result.SpecHash}); err != nil {
		return Draft{}, err
	}
	return result, nil
}

func (service *Service) UpdateDraft(ctx context.Context, draftID string, input UpdateDraftInput, actor Actor) (Draft, error) {
	input.Spec = NormalizeSpec(input.Spec)
	if input.ExpectedVersion <= 0 || strings.TrimSpace(input.ChangeReason) == "" {
		return Draft{}, errors.New("expected_version and change_reason are required")
	}
	if input.Spec.AdvertiserID <= 0 {
		return Draft{}, errors.New("advertiser_id must be positive")
	}
	result, err := service.store.UpdateDraft(ctx, draftID, input, actor)
	if err != nil {
		return Draft{}, err
	}
	if err := service.store.Audit(ctx, actor, "update", "delivery_draft", result.ID, result.AdvertiserID, map[string]any{"version": result.CurrentVersion, "spec_hash": result.SpecHash, "reason": input.ChangeReason}); err != nil {
		return Draft{}, err
	}
	return result, nil
}

func (service *Service) Draft(ctx context.Context, draftID string) (Draft, error) {
	return service.store.Draft(ctx, draftID)
}

func (service *Service) Drafts(ctx context.Context, advertiserID int64, limit int) ([]Draft, error) {
	return service.store.Drafts(ctx, advertiserID, limit)
}

func (service *Service) Workflow(ctx context.Context, draftID string) (Workflow, error) {
	draft, err := service.store.Draft(ctx, draftID)
	if err != nil {
		return Workflow{}, err
	}
	result := Workflow{Draft: draft, Approvals: []Approval{}, Jobs: []PublishJob{}, Entities: []MediaEntity{}}
	if recommendation, recommendationErr := service.store.LatestRecommendation(ctx, draft.ID, draft.CurrentVersion); recommendationErr == nil {
		result.Recommendation = &recommendation
	} else if !errors.Is(recommendationErr, ErrNotFound) {
		return Workflow{}, recommendationErr
	}
	if validation, validationErr := service.store.LatestValidation(ctx, draft.ID, draft.CurrentVersion); validationErr == nil {
		result.Validation = &validation
	} else if !errors.Is(validationErr, ErrNotFound) {
		return Workflow{}, validationErr
	}
	result.Approvals, err = service.store.Approvals(ctx, draft.ID, draft.CurrentVersion)
	if err != nil {
		return Workflow{}, err
	}
	result.Jobs, err = service.store.PublishJobs(ctx, draft.ID, draft.CurrentVersion, 20)
	if err != nil {
		return Workflow{}, err
	}
	result.Entities, err = service.store.MediaEntities(ctx, draft.ID)
	if err != nil {
		return Workflow{}, err
	}
	return result, nil
}

func (service *Service) Recommend(ctx context.Context, draftID string, actor Actor) (Recommendation, error) {
	draft, err := service.store.Draft(ctx, draftID)
	if err != nil {
		return Recommendation{}, err
	}
	noteIDs := collectDraftNoteIDs(draft.Spec)
	if len(noteIDs) == 0 {
		return Recommendation{}, fmt.Errorf("%w: draft has no candidate notes", ErrValidation)
	}
	candidates, err := service.store.RecommendationCandidates(ctx, noteIDs)
	if err != nil {
		return Recommendation{}, err
	}
	if len(candidates) == 0 {
		return Recommendation{}, fmt.Errorf("%w: no draft notes exist in the manuscript catalog", ErrValidation)
	}
	semantic, semanticErr := service.semanticAdvisor.Advise(ctx, SemanticRequest{
		Objective: draft.Spec.Objective, Placement: draft.Spec.Placement, Candidates: candidates,
	})
	if semanticErr != nil {
		return Recommendation{}, semanticErr
	}
	ranking, rankErr := service.ranker.Rank(ctx, RankRequest{
		Objective: draft.Spec.Experiment.PrimaryMetric,
		Items:     BuildRankFeatures(candidates),
	})
	if rankErr != nil {
		return Recommendation{}, rankErr
	}
	keywordCandidates := semantic.KeywordSeeds
	platformKeywords := map[string]any{}
	if len(noteIDs) > 0 {
		response, toolErr := service.callGateway(ctx, "", draft.AdvertiserID, "keyword.recommend", map[string]any{
			"advertiser_id": draft.AdvertiserID, "request_type": "note", "item_ids": noteIDs,
			"promotion_target": draft.Spec.Campaign.PromotionTarget, "rank": 1,
		})
		if toolErr == nil {
			platformKeywords = response.Data
		} else {
			ranking.Warnings = append(ranking.Warnings, "平台推词不可用: "+limitRunes(toolErr.Error(), 240))
		}
	}
	recommendationID, err := NewID("rec")
	if err != nil {
		return Recommendation{}, err
	}
	warnings := append([]string{}, semantic.Uncertainties...)
	warnings = append(warnings, ranking.Warnings...)
	value := Recommendation{
		ID: recommendationID, DraftID: draft.ID, DraftVersion: draft.CurrentVersion,
		SchemaVersion: RecommendationSchemaVersion, LLMProvider: semantic.Provider,
		LLMModel: semantic.Model, RankerFamily: ranking.Family, RankerVersion: ranking.Version,
		RulesVersion: RulesVersion, Warnings: warnings, CreatedBy: actor.ID,
		Payload: map[string]any{
			"ranked_notes":                ranking.Items,
			"themes":                      semantic.Themes,
			"keyword_seeds":               keywordCandidates,
			"platform_keyword_candidates": platformKeywords,
			"negative_keywords":           semantic.NegativeKeywords,
			"audience_hypotheses":         semantic.AudienceHypotheses,
			"note_evidence":               semantic.NoteEvidence,
			"uncertainties":               warnings,
			"requires_human_review":       true,
			"executable":                  false,
		},
	}
	result, err := service.store.SaveRecommendation(ctx, value)
	if err != nil {
		return Recommendation{}, err
	}
	if err := service.store.Audit(ctx, actor, "recommend", "delivery_draft", draft.ID, draft.AdvertiserID, map[string]any{"recommendation_id": result.ID, "draft_version": draft.CurrentVersion}); err != nil {
		return Recommendation{}, err
	}
	return result, nil
}

func (service *Service) Validate(ctx context.Context, draftID string, actor Actor) (Validation, error) {
	draft, err := service.store.Draft(ctx, draftID)
	if err != nil {
		return Validation{}, err
	}
	localErrors, localWarnings := SplitIssues(ValidateDraftSpec(draft.Spec))
	capability, capabilityErr := service.Capabilities(ctx, draft.AdvertiserID)
	capabilityMap := map[string]any{}
	contractVersion := MediaContractVersion
	if capabilityErr != nil {
		localErrors = append(localErrors, ValidationIssue{Code: "capability_unavailable", Path: "advertiser_id", Message: capabilityErr.Error(), Severity: "error"})
	} else {
		capabilityMap = capabilityToMap(capability)
		contractVersion = capability.ContractVersion
		if !capability.Authorized || !capability.AdvertiserAllowed {
			localErrors = append(localErrors, ValidationIssue{Code: "advertiser_unauthorized", Path: "advertiser_id", Message: "advertiser is not available in the current OAuth authorization", Severity: "error"})
		}
		if len(capability.MissingScopes) > 0 {
			localErrors = append(localErrors, ValidationIssue{Code: "scope_missing", Path: "advertiser_id", Message: "missing OAuth scopes: " + strings.Join(capability.MissingScopes, ", "), Severity: "error"})
		}
		if !capability.MediaWritesEnabled {
			localWarnings = append(localWarnings, ValidationIssue{Code: "media_writes_disabled", Path: "advertiser_id", Message: "media writes are disabled; dry_run remains available", Severity: "warning"})
		}
	}
	if capabilityErr == nil && capability.Authorized && capability.AdvertiserAllowed {
		service.validateRemoteGuardrails(ctx, draft, &localErrors, &localWarnings, capabilityMap)
	}
	validationID, err := NewID("val")
	if err != nil {
		return Validation{}, err
	}
	value := Validation{
		ID: validationID, DraftID: draft.ID, DraftVersion: draft.CurrentVersion,
		SpecHash: draft.SpecHash, RulesVersion: RulesVersion, ContractVersion: contractVersion,
		Valid: len(localErrors) == 0, Errors: localErrors, Warnings: localWarnings,
		CapabilitySnapshot: capabilityMap, ValidUntil: service.now().UTC().Add(15 * time.Minute),
		CreatedBy: actor.ID,
	}
	result, err := service.store.SaveValidation(ctx, value)
	if err != nil {
		return Validation{}, err
	}
	if err := service.store.Audit(ctx, actor, "validate", "delivery_draft", draft.ID, draft.AdvertiserID, map[string]any{"valid": result.Valid, "errors": len(result.Errors), "warnings": len(result.Warnings), "validation_id": result.ID}); err != nil {
		return Validation{}, err
	}
	return result, nil
}

func (service *Service) Approve(ctx context.Context, draftID string, input ApprovalInput, actor Actor) (Approval, error) {
	if actor.ID == "" || actor.Role == "" {
		return Approval{}, ErrForbidden
	}
	if input.Role != "operator" && input.Role != "budget_owner" {
		return Approval{}, errors.New("role must be operator or budget_owner")
	}
	if actor.Role != input.Role && actor.Role != "admin" {
		return Approval{}, fmt.Errorf("%w: actor role cannot grant this approval", ErrForbidden)
	}
	if input.Decision != "approved" && input.Decision != "rejected" {
		return Approval{}, errors.New("decision must be approved or rejected")
	}
	draft, err := service.store.Draft(ctx, draftID)
	if err != nil {
		return Approval{}, err
	}
	if actor.ID == draft.UpdatedBy {
		return Approval{}, fmt.Errorf("%w: the draft submitter cannot approve their own version", ErrForbidden)
	}
	if input.Decision == "approved" && input.ApprovedBudgetFen < draft.Spec.Budget.TotalLimitFen {
		return Approval{}, errors.New("approved_budget_fen cannot be below the draft total budget")
	}
	minutes := input.ExpiresInMinutes
	if minutes == 0 {
		minutes = 60
	}
	if minutes < 5 || minutes > 24*60 {
		return Approval{}, errors.New("expires_in_minutes must be between 5 and 1440")
	}
	approvalID, err := NewID("apr")
	if err != nil {
		return Approval{}, err
	}
	value := Approval{
		ID: approvalID, DraftID: draft.ID, DraftVersion: draft.CurrentVersion,
		SpecHash: draft.SpecHash, Role: input.Role, Decision: input.Decision, Actor: actor.ID,
		Comment: strings.TrimSpace(input.Comment), ApprovedBudgetFen: input.ApprovedBudgetFen,
		ExpiresAt: service.now().UTC().Add(time.Duration(minutes) * time.Minute),
	}
	result, err := service.store.SaveApproval(ctx, value)
	if err != nil {
		return Approval{}, err
	}
	if err := service.store.Audit(ctx, actor, "approve", "delivery_draft", draft.ID, draft.AdvertiserID, map[string]any{"role": input.Role, "decision": input.Decision, "draft_version": draft.CurrentVersion, "approved_budget_fen": input.ApprovedBudgetFen}); err != nil {
		return Approval{}, err
	}
	return result, nil
}

func (service *Service) Publish(ctx context.Context, draftID string, input PublishInput, actor Actor) (PublishJob, error) {
	input.Mode = strings.TrimSpace(input.Mode)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.Mode == "" {
		input.Mode = "dry_run"
	}
	if input.Mode != "dry_run" && input.Mode != "execute" {
		return PublishJob{}, errors.New("mode must be dry_run or execute")
	}
	if len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 200 {
		return PublishJob{}, errors.New("idempotency_key must contain 8 to 200 characters")
	}
	draft, err := service.store.Draft(ctx, draftID)
	if err != nil {
		return PublishJob{}, err
	}
	existing, existingErr := service.store.PublishJobByIdempotency(ctx, input.IdempotencyKey)
	if existingErr == nil {
		if existing.DraftID != draft.ID || existing.DraftVersion != draft.CurrentVersion || existing.AdvertiserID != draft.AdvertiserID || existing.Mode != input.Mode {
			return PublishJob{}, fmt.Errorf("%w: publish idempotency key belongs to another request", ErrConflict)
		}
		return existing, nil
	}
	if !errors.Is(existingErr, ErrNotFound) {
		return PublishJob{}, existingErr
	}
	validation, validationErr := service.Validate(ctx, draftID, actor)
	if validationErr != nil {
		return PublishJob{}, validationErr
	}
	if !validation.Valid {
		return PublishJob{}, fmt.Errorf("%w: publish validation has %d errors", ErrValidation, len(validation.Errors))
	}
	preview := service.buildPublishPreview(draft)
	if input.Mode == "execute" {
		if !service.mediaWritesEnabled {
			return PublishJob{}, ErrWritesDisabled
		}
		if enabled, _ := validation.CapabilitySnapshot["media_writes_enabled"].(bool); !enabled {
			return PublishJob{}, ErrWritesDisabled
		}
		approvals, err := service.store.Approvals(ctx, draft.ID, draft.CurrentVersion)
		if err != nil {
			return PublishJob{}, err
		}
		if err := validateApprovals(draft, approvals, service.now().UTC()); err != nil {
			return PublishJob{}, err
		}
	}
	jobID, err := NewID("job")
	if err != nil {
		return PublishJob{}, err
	}
	job := PublishJob{
		ID: jobID, DraftID: draft.ID, DraftVersion: draft.CurrentVersion, AdvertiserID: draft.AdvertiserID,
		Mode: input.Mode, Status: "queued", CurrentStep: "queued", IdempotencyKey: input.IdempotencyKey,
		RequestPreview: preview, Result: map[string]any{}, RequestedBy: actor.ID, RequestedRole: actor.Role,
	}
	job, err = service.store.CreatePublishJob(ctx, job)
	if err != nil {
		return PublishJob{}, err
	}
	// A different ID means another request won the idempotency race. Never execute
	// its queued job in this request, because that could create duplicate media.
	if job.ID != jobID {
		service.signalPublisher()
		return job, nil
	}
	if job.Status != "queued" {
		return job, nil
	}
	if input.Mode == "dry_run" {
		now := service.now().UTC()
		job.Status = "succeeded"
		job.CurrentStep = "dry_run_complete"
		job.StartedAt = &now
		job.CompletedAt = &now
		job.Result = map[string]any{
			"executed": false, "campaigns": 1, "units": len(draft.Spec.Units),
			"creativities": countCreativities(draft.Spec), "forced_initial_status": "paused",
		}
		if err := service.store.UpdatePublishJob(ctx, job); err != nil {
			return PublishJob{}, err
		}
		return service.store.PublishJob(ctx, job.ID)
	}
	if err := service.store.Audit(ctx, actor, "publish_queued", "delivery_draft", draft.ID, draft.AdvertiserID, map[string]any{"job_id": job.ID, "mode": job.Mode, "status": job.Status}); err != nil {
		return PublishJob{}, err
	}
	service.signalPublisher()
	return job, nil
}

func (service *Service) RunPublisher(ctx context.Context, onError func(error)) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		for {
			processed, err := service.ProcessNextPublishJob(ctx)
			if err != nil && !errors.Is(err, context.Canceled) && onError != nil {
				onError(err)
			}
			if err != nil || !processed {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-service.publishWake:
		case <-ticker.C:
		}
	}
}

func (service *Service) ProcessNextPublishJob(ctx context.Context) (bool, error) {
	job, claimed, err := service.store.ClaimPublishJob(ctx)
	if err != nil || !claimed {
		return claimed, err
	}
	draft, err := service.store.Draft(ctx, job.DraftID)
	if err != nil || draft.CurrentVersion != job.DraftVersion || draft.AdvertiserID != job.AdvertiserID {
		if err == nil {
			err = ErrApprovalStale
		}
		failed := failPreflightJob(job, "draft_version_mismatch", err, service.now().UTC())
		return true, service.store.UpdatePublishJob(ctx, failed)
	}
	if !job.CreatedAt.IsZero() && service.now().UTC().Sub(job.CreatedAt) > 15*time.Minute {
		failed := failPreflightJob(job, "validation_expired", ErrCapabilityExpired, service.now().UTC())
		return true, service.store.UpdatePublishJob(ctx, failed)
	}
	if !service.mediaWritesEnabled {
		failed := failPreflightJob(job, "media_writes_disabled", ErrWritesDisabled, service.now().UTC())
		return true, service.store.UpdatePublishJob(ctx, failed)
	}
	capability, err := service.Capabilities(ctx, job.AdvertiserID)
	if err != nil {
		failed := failPreflightJob(job, "capability_unavailable", err, service.now().UTC())
		return true, service.store.UpdatePublishJob(ctx, failed)
	}
	if !capability.Authorized || !capability.AdvertiserAllowed || len(capability.MissingScopes) > 0 || !capability.MediaWritesEnabled {
		failed := failPreflightJob(job, "capability_revoked", ErrCapabilityExpired, service.now().UTC())
		return true, service.store.UpdatePublishJob(ctx, failed)
	}
	approvals, err := service.store.Approvals(ctx, draft.ID, draft.CurrentVersion)
	if err != nil {
		failed := failPreflightJob(job, "approval_lookup_failed", err, service.now().UTC())
		return true, service.store.UpdatePublishJob(ctx, failed)
	}
	if err := validateApprovals(draft, approvals, service.now().UTC()); err != nil {
		failed := failPreflightJob(job, "approval_expired", err, service.now().UTC())
		return true, service.store.UpdatePublishJob(ctx, failed)
	}
	job = service.executePublish(ctx, draft, job)
	if err := service.store.UpdatePublishJob(ctx, job); err != nil {
		return true, err
	}
	actor := Actor{ID: job.RequestedBy, Role: job.RequestedRole}
	if err := service.store.Audit(ctx, actor, "publish_complete", "delivery_draft", draft.ID, draft.AdvertiserID, map[string]any{"job_id": job.ID, "mode": job.Mode, "status": job.Status}); err != nil {
		return true, err
	}
	return true, nil
}

func (service *Service) signalPublisher() {
	select {
	case service.publishWake <- struct{}{}:
	default:
	}
}

func (service *Service) Job(ctx context.Context, jobID string) (PublishJob, error) {
	return service.store.PublishJob(ctx, jobID)
}

func (service *Service) UpdateCampaignStatus(ctx context.Context, input CampaignStatusInput, actor Actor) (CampaignStatusResult, error) {
	input, err := normalizeCampaignStatusInput(input)
	if err != nil {
		return CampaignStatusResult{}, err
	}
	if actor.Role != "operator" && actor.Role != "admin" {
		return CampaignStatusResult{}, fmt.Errorf("%w: operator or admin role is required", ErrForbidden)
	}
	if !service.mediaWritesEnabled {
		return CampaignStatusResult{}, ErrWritesDisabled
	}
	response, err := service.callGateway(ctx, "", input.AdvertiserID, "campaign.status", map[string]any{
		"advertiser_id": input.AdvertiserID,
		"campaign_ids":  input.CampaignIDs,
		"action_type":   input.ActionType,
	})
	if err != nil {
		return CampaignStatusResult{}, err
	}
	updated := campaignIDsFromGateway(response.Data)
	if len(updated) == 0 {
		updated = append([]int64(nil), input.CampaignIDs...)
	}
	status := campaignStatusName(input.ActionType)
	localUpdated := make([]int64, 0)
	for _, campaignID := range updated {
		entity, lookupErr := service.store.MediaEntity(ctx, input.AdvertiserID, "campaign", campaignID)
		if errors.Is(lookupErr, ErrNotFound) {
			continue
		}
		if lookupErr != nil {
			return CampaignStatusResult{}, lookupErr
		}
		if err := service.store.UpdateMediaEntityStatus(ctx, entity.ID, status); err != nil {
			return CampaignStatusResult{}, err
		}
		localUpdated = append(localUpdated, campaignID)
	}
	if err := service.store.Audit(ctx, actor, "campaign_status_update", "campaign", strconv.FormatInt(input.AdvertiserID, 10), input.AdvertiserID, map[string]any{
		"action_type":          input.ActionType,
		"campaign_ids":         input.CampaignIDs,
		"updated_campaign_ids": updated,
		"local_entity_ids":     localUpdated,
	}); err != nil {
		return CampaignStatusResult{}, err
	}
	return CampaignStatusResult{
		AdvertiserID:         input.AdvertiserID,
		ActionType:           input.ActionType,
		RequestedCampaignIDs: input.CampaignIDs,
		CampaignIDs:          updated,
		LocalEntityIDs:       localUpdated,
		Gateway:              response,
	}, nil
}

func (service *Service) UpdateEntityStatus(ctx context.Context, advertiserID int64, entityType string, mediaID int64, status string, actor Actor) (GatewayResponse, error) {
	if status != "paused" && status != "active" {
		return GatewayResponse{}, errors.New("status must be paused or active")
	}
	if !service.mediaWritesEnabled {
		return GatewayResponse{}, ErrWritesDisabled
	}
	entity, err := service.store.MediaEntity(ctx, advertiserID, entityType, mediaID)
	if err != nil {
		return GatewayResponse{}, err
	}
	if actor.Role != "operator" && actor.Role != "admin" {
		return GatewayResponse{}, fmt.Errorf("%w: operator or admin role is required", ErrForbidden)
	}
	if status == "active" {
		job, jobErr := service.store.PublishJob(ctx, entity.JobID)
		readbackVerified, _ := job.Result["readback_verified"].(bool)
		if jobErr != nil {
			return GatewayResponse{}, jobErr
		}
		if job.Mode != "execute" || job.Status != "succeeded" || job.CurrentStep != "complete_paused" || !readbackVerified {
			return GatewayResponse{}, fmt.Errorf("%w: entity cannot activate before a successful paused-state publish and readback", ErrConflict)
		}
		draft, loadErr := service.store.Draft(ctx, entity.DraftID)
		if loadErr != nil {
			return GatewayResponse{}, loadErr
		}
		approvals, approvalErr := service.store.Approvals(ctx, draft.ID, draft.CurrentVersion)
		if approvalErr != nil {
			return GatewayResponse{}, approvalErr
		}
		if approvalErr := validateApprovals(draft, approvals, service.now().UTC()); approvalErr != nil {
			return GatewayResponse{}, approvalErr
		}
	}
	action := 2
	if status == "active" {
		action = 1
	}
	operation, idField, actionField := "", "", ""
	switch entityType {
	case "campaign":
		operation, idField, actionField = "campaign.status", "campaign_ids", "action_type"
	case "unit":
		operation, idField, actionField = "unit.status", "unit_ids", "status"
	case "creativity":
		operation, idField, actionField = "creativity.status", "creativity_ids", "action_type"
	default:
		return GatewayResponse{}, errors.New("entity type must be campaign, unit, or creativity")
	}
	response, err := service.callGateway(ctx, entity.JobID, advertiserID, operation, map[string]any{
		"advertiser_id": advertiserID, idField: []int64{mediaID}, actionField: action,
	})
	if err != nil {
		return GatewayResponse{}, err
	}
	if err := service.store.UpdateMediaEntityStatus(ctx, entity.ID, status); err != nil {
		return GatewayResponse{}, err
	}
	if err := service.store.Audit(ctx, actor, "status_update", "delivery_media_entity", entity.ID, advertiserID, map[string]any{"status": status, "media_id": mediaID, "entity_type": entityType}); err != nil {
		return GatewayResponse{}, err
	}
	return response, nil
}

func (service *Service) Performance(ctx context.Context, query PerformanceQuery, actor Actor) (map[string]any, error) {
	if query.AdvertiserID <= 0 || !oneOfString(query.Level, "account", "campaign", "unit", "creativity", "keyword") {
		return nil, errors.New("advertiser_id and a valid level are required")
	}
	start, startErr := time.Parse(time.DateOnly, query.StartDate)
	end, endErr := time.Parse(time.DateOnly, query.EndDate)
	if startErr != nil || endErr != nil || end.Before(start) || end.Sub(start) > 93*24*time.Hour {
		return nil, errors.New("start_date and end_date must form a valid range of no more than 93 days")
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 100
	}
	if query.PageSize > 1000 {
		return nil, errors.New("page_size cannot exceed 1000")
	}
	if len(query.SplitColumns) > 50 {
		return nil, errors.New("split_columns cannot contain more than 50 values")
	}
	if err := validateReportFilters(query.Filters); err != nil {
		return nil, err
	}
	prefix := "report.offline."
	if query.Realtime {
		prefix = "report.realtime."
	}
	payload := map[string]any{
		"advertiser_id": query.AdvertiserID, "start_date": query.StartDate, "end_date": query.EndDate,
	}
	if !query.Realtime || query.Level != "account" {
		payload["page_num"] = query.Page
		payload["page_size"] = query.PageSize
	}
	if !query.Realtime && len(query.SplitColumns) > 0 {
		payload["split_columns"] = query.SplitColumns
	}
	for key, value := range query.Filters {
		payload[key] = value
	}
	response, err := service.callGateway(ctx, "", query.AdvertiserID, prefix+query.Level, payload)
	if err != nil {
		return nil, err
	}
	if err := service.store.SavePerformanceSnapshot(ctx, query, response.Data, MediaContractVersion); err != nil {
		return nil, err
	}
	_ = service.store.Audit(ctx, actor, "read_performance", "advertiser", strconv.FormatInt(query.AdvertiserID, 10), query.AdvertiserID, map[string]any{"level": query.Level, "realtime": query.Realtime, "start_date": query.StartDate, "end_date": query.EndDate})
	return response.Data, nil
}

func (service *Service) validateRemoteGuardrails(ctx context.Context, draft Draft, errorsOut, warningsOut *[]ValidationIssue, capabilityMap map[string]any) {
	if balance, err := service.callGateway(ctx, "", draft.AdvertiserID, "account.balance", map[string]any{"advertiser_id": draft.AdvertiserID}); err != nil {
		*errorsOut = append(*errorsOut, ValidationIssue{Code: "balance_unavailable", Path: "budget", Message: limitRunes(err.Error(), 300), Severity: "error"})
	} else {
		capabilityMap["balance"] = balance.Data
		available := numericInt64(balance.Data["available_balance"])
		if available > 0 && available < draft.Spec.Budget.DailyLimitFen {
			*errorsOut = append(*errorsOut, ValidationIssue{Code: "balance_insufficient", Path: "budget.daily_limit_fen", Message: "available balance is below one day of approved budget", Severity: "error"})
		}
	}
	if whitelist, err := service.callGateway(ctx, "", draft.AdvertiserID, "account.white_list", map[string]any{"advertiser_id": draft.AdvertiserID}); err != nil {
		*warningsOut = append(*warningsOut, ValidationIssue{Code: "white_list_unavailable", Path: "advertiser_id", Message: limitRunes(err.Error(), 300), Severity: "warning"})
	} else {
		capabilityMap["white_list"] = whitelist.Data
	}
	campaignNames := []string{draft.Spec.Campaign.Name}
	unitNames := make([]string, 0, len(draft.Spec.Units))
	for _, unit := range draft.Spec.Units {
		unitNames = append(unitNames, unit.Name)
	}
	for _, check := range []struct {
		names  []string
		typeID int
		path   string
	}{{campaignNames, 1, "campaign.name"}, {unitNames, 2, "units"}} {
		if len(check.names) == 0 {
			continue
		}
		if duplicate, err := service.callGateway(ctx, "", draft.AdvertiserID, "delivery.name_check", map[string]any{
			"advertiser_id": draft.AdvertiserID, "name": check.names, "type": check.typeID,
		}); err != nil {
			*warningsOut = append(*warningsOut, ValidationIssue{Code: "name_check_unavailable", Path: check.path, Message: limitRunes(err.Error(), 300), Severity: "warning"})
		} else if result, ok := duplicate.Data["check_result"].(map[string]any); ok {
			for name, raw := range result {
				if duplicated, _ := raw.(bool); duplicated {
					*errorsOut = append(*errorsOut, ValidationIssue{Code: "name_duplicate", Path: check.path, Message: "platform reports duplicate name: " + name, Severity: "error"})
				}
			}
		}
	}
	for index, unit := range draft.Spec.Units {
		if unit.TargetType != 3 {
			continue
		}
		response, err := service.callGateway(ctx, "", draft.AdvertiserID, "target.audience_estimate", map[string]any{
			"advertiser_id": draft.AdvertiserID, "marketing_target": draft.Spec.Campaign.MarketingTarget,
			"placement": draft.Spec.Campaign.Placement, "optimize_target": draft.Spec.Campaign.OptimizeTarget,
			"target_type": unit.TargetType, "target_config": targetPayload(unit.Target),
		})
		if err != nil {
			*warningsOut = append(*warningsOut, ValidationIssue{Code: "audience_estimate_unavailable", Path: fmt.Sprintf("units[%d].target", index), Message: limitRunes(err.Error(), 300), Severity: "warning"})
			continue
		}
		scope := numericInt64(response.Data["crowd_scope"])
		if scope == 1 {
			*errorsOut = append(*errorsOut, ValidationIssue{Code: "audience_too_narrow", Path: fmt.Sprintf("units[%d].target", index), Message: "platform audience estimate is too narrow", Severity: "error"})
		}
	}
}

func (service *Service) executePublish(ctx context.Context, draft Draft, job PublishJob) PublishJob {
	started := service.now().UTC()
	job.Status = "publishing"
	job.StartedAt = &started
	job.CurrentStep = "campaign.create"
	job.Result = map[string]any{"created_entities": []map[string]any{}, "forced_initial_status": "paused"}
	if err := service.store.UpdatePublishJob(ctx, job); err != nil {
		return failJob(job, "persist_job", err, service.now().UTC())
	}
	campaignPayload := campaignPayload(draft)
	campaignResponse, err := service.callGateway(ctx, job.ID, draft.AdvertiserID, "campaign.create", campaignPayload)
	if err != nil {
		return failJob(job, "campaign_create_failed", err, service.now().UTC())
	}
	campaignID := findMediaID(campaignResponse.Data, "campaign_id")
	if campaignID <= 0 {
		return failJob(job, "campaign_response_invalid", errors.New("campaign create returned no positive campaign_id"), service.now().UTC())
	}
	if _, err := service.saveEntity(ctx, job, draft, "campaign", draft.Spec.Campaign.LocalKey, "", campaignID, 0, campaignResponse.Data); err != nil {
		return failJob(job, "campaign_mapping_failed", err, service.now().UTC())
	}
	appendCreatedEntity(job.Result, "campaign", draft.Spec.Campaign.LocalKey, campaignID, 0)
	for unitIndex, unit := range draft.Spec.Units {
		job.CurrentStep = fmt.Sprintf("unit.create[%d]", unitIndex)
		if err := service.store.UpdatePublishJob(ctx, job); err != nil {
			return failJob(job, "persist_job", err, service.now().UTC())
		}
		unitResponse, err := service.callGateway(ctx, job.ID, draft.AdvertiserID, "unit.create", unitPayload(draft, unit, campaignID))
		if err != nil {
			return failJob(job, "unit_create_failed", err, service.now().UTC())
		}
		unitID := findMediaID(unitResponse.Data, "unit_id")
		if unitID <= 0 {
			return failJob(job, "unit_response_invalid", errors.New("unit create returned no positive unit_id"), service.now().UTC())
		}
		if _, err := service.saveEntity(ctx, job, draft, "unit", unit.LocalKey, draft.Spec.Campaign.LocalKey, unitID, campaignID, unitResponse.Data); err != nil {
			return failJob(job, "unit_mapping_failed", err, service.now().UTC())
		}
		appendCreatedEntity(job.Result, "unit", unit.LocalKey, unitID, campaignID)
		if len(unit.NegativeKeywords) > 0 {
			job.CurrentStep = fmt.Sprintf("keyword.negative_add[%d]", unitIndex)
			if err := service.store.UpdatePublishJob(ctx, job); err != nil {
				return failJob(job, "persist_job", err, service.now().UTC())
			}
			negativeKeywords := make([]map[string]any, 0, len(unit.NegativeKeywords))
			for _, keyword := range unit.NegativeKeywords {
				negativeKeywords = append(negativeKeywords, map[string]any{"keyword": keyword.Keyword, "phrase_match_type": keyword.PhraseMatchType})
			}
			if _, err := service.callGateway(ctx, job.ID, draft.AdvertiserID, "keyword.negative_add", map[string]any{
				"advertiser_id": draft.AdvertiserID, "unit_id": unitID, "keywords": negativeKeywords,
			}); err != nil {
				return failJob(job, "negative_keyword_add_failed", err, service.now().UTC())
			}
		}
		for creativeIndex, creative := range unit.Creativities {
			job.CurrentStep = fmt.Sprintf("creativity.create[%d][%d]", unitIndex, creativeIndex)
			if err := service.store.UpdatePublishJob(ctx, job); err != nil {
				return failJob(job, "persist_job", err, service.now().UTC())
			}
			creativeResponse, err := service.callGateway(ctx, job.ID, draft.AdvertiserID, "creativity.create", creativityPayload(draft, creative, unitID))
			if err != nil {
				return failJob(job, "creativity_create_failed", err, service.now().UTC())
			}
			creativeID := findMediaID(creativeResponse.Data, "creativity_id")
			if creativeID <= 0 {
				return failJob(job, "creativity_response_invalid", errors.New("creativity create returned no positive creativity_id"), service.now().UTC())
			}
			if _, err := service.saveEntity(ctx, job, draft, "creativity", creative.LocalKey, unit.LocalKey, creativeID, unitID, creativeResponse.Data); err != nil {
				return failJob(job, "creativity_mapping_failed", err, service.now().UTC())
			}
			appendCreatedEntity(job.Result, "creativity", creative.LocalKey, creativeID, unitID)
		}
	}
	job.CurrentStep = "read_back"
	if err := service.store.UpdatePublishJob(ctx, job); err != nil {
		return failJob(job, "persist_job", err, service.now().UTC())
	}
	created, _ := job.Result["created_entities"].([]map[string]any)
	for _, entity := range created {
		entityType, _ := entity["type"].(string)
		mediaID := numericInt64(entity["media_id"])
		operation, idField := "", ""
		payload := map[string]any{"advertiser_id": draft.AdvertiserID}
		switch entityType {
		case "campaign":
			operation, idField = "campaign.list", "campaign_ids"
		case "unit":
			operation, idField = "unit.list", "unit_ids"
		case "creativity":
			operation, idField = "creativity.search", "creativity_ids"
		default:
			return failJob(job, "readback_entity_invalid", fmt.Errorf("unknown entity type %q", entityType), service.now().UTC())
		}
		payload[idField] = []int64{mediaID}
		readBack, err := service.callGateway(ctx, job.ID, draft.AdvertiserID, operation, payload)
		if err != nil {
			return failJob(job, "readback_failed", err, service.now().UTC())
		}
		if err := verifyReadbackEntity(draft, entity, readBack.Data); err != nil {
			return failJob(job, "readback_mismatch", err, service.now().UTC())
		}
	}
	completed := service.now().UTC()
	job.Status = "succeeded"
	job.CurrentStep = "complete_paused"
	job.CompletedAt = &completed
	job.Result["executed"] = true
	job.Result["activation_required"] = true
	job.Result["readback_verified"] = true
	return job
}

func (service *Service) saveEntity(ctx context.Context, job PublishJob, draft Draft, entityType, localKey, parentLocalKey string, mediaID, parentMediaID int64, payload map[string]any) (MediaEntity, error) {
	entityID, err := NewID("ent")
	if err != nil {
		return MediaEntity{}, err
	}
	return service.store.SaveMediaEntity(ctx, MediaEntity{
		ID: entityID, JobID: job.ID, DraftID: draft.ID, AdvertiserID: draft.AdvertiserID,
		EntityType: entityType, LocalKey: localKey, ParentLocalKey: parentLocalKey,
		MediaID: mediaID, ParentMediaID: parentMediaID, DesiredStatus: "paused",
		ObservedStatus: "created_paused", UpstreamPayload: payload,
	})
}

func (service *Service) callGateway(ctx context.Context, jobID string, advertiserID int64, operation string, payload map[string]any) (GatewayResponse, error) {
	started := time.Now()
	response, err := service.gateway.Call(ctx, operation, payload)
	attempt := APIAttempt{
		JobID: jobID, AdvertiserID: advertiserID, Operation: operation,
		ContractVersion: MediaContractVersion, RequestSummary: sanitizePayload(payload),
		ResponseSummary: map[string]any{}, Success: err == nil,
		LatencyMS: time.Since(started).Milliseconds(),
	}
	if response.RequestHash != "" {
		attempt.RequestHash = response.RequestHash
	} else {
		encoded := fmt.Sprintf("%v", attempt.RequestSummary)
		digest := sha256.Sum256([]byte(encoded))
		attempt.RequestHash = hex.EncodeToString(digest[:])
	}
	attempt.UpstreamRequestID = response.RequestID
	attempt.LatencyMS = maxInt64(attempt.LatencyMS, response.LatencyMS)
	if err != nil {
		attempt.ErrorCode = "upstream_error"
		attempt.ErrorMessage = limitRunes(err.Error(), 1000)
	} else {
		attempt.ResponseSummary = summarizeResponse(response.Data)
	}
	if auditErr := service.store.SaveAPIAttempt(ctx, attempt); auditErr != nil && err == nil {
		return GatewayResponse{}, auditErr
	}
	return response, err
}

func (service *Service) buildPublishPreview(draft Draft) map[string]any {
	return map[string]any{
		"contract_version":             MediaContractVersion,
		"campaign":                     campaignPayload(draft),
		"units":                        len(draft.Spec.Units),
		"creativities":                 countCreativities(draft.Spec),
		"operations":                   []string{"campaign.create", "unit.create", "creativity.create", "read_back"},
		"initial_status":               "paused",
		"requires_separate_activation": true,
	}
}

var readOnlyPlatformOperations = map[string]bool{
	"account.balance":          true,
	"account.white_list":       true,
	"asset.event_list":         true,
	"asset.note_list":          true,
	"asset.qualification_list": true,
	"asset.spu_list":           true,
	"campaign.list":            true,
	"creativity.search":        true,
	"delivery.name_check":      true,
	"keyword.negative_list":    true,
	"keyword.recommend":        true,
	"keyword.word_bags":        true,
	"target.audience_estimate": true,
	"target.options":           true,
	"unit.list":                true,
}

func validateApprovals(draft Draft, approvals []Approval, now time.Time) error {
	latest := map[string]Approval{}
	for _, approval := range approvals {
		if approval.DraftVersion != draft.CurrentVersion || approval.SpecHash != draft.SpecHash {
			continue
		}
		current, exists := latest[approval.Role]
		if !exists || approval.CreatedAt.After(current.CreatedAt) || (approval.CreatedAt.Equal(current.CreatedAt) && approval.ID > current.ID) {
			latest[approval.Role] = approval
		}
	}

	roles := map[string]Approval{}
	actors := map[string]struct{}{}
	for _, approval := range latest {
		if approval.Decision != "approved" || !approval.ExpiresAt.After(now) {
			continue
		}
		if approval.ApprovedBudgetFen < draft.Spec.Budget.TotalLimitFen {
			return ErrApprovalStale
		}
		roles[approval.Role] = approval
		actors[approval.Actor] = struct{}{}
	}
	if _, ok := roles["operator"]; !ok {
		return ErrApprovalRequired
	}
	if _, ok := roles["budget_owner"]; !ok {
		return ErrApprovalRequired
	}
	if len(actors) < 2 {
		return fmt.Errorf("%w: operator and budget owner must be different people", ErrApprovalRequired)
	}
	return nil
}

func campaignPayload(draft Draft) map[string]any {
	campaign := draft.Spec.Campaign
	result := map[string]any{
		"advertiser_id": draft.AdvertiserID, "marketing_target": campaign.MarketingTarget,
		"campaign_name": campaign.Name, "placement": campaign.Placement,
		"promotion_target": campaign.PromotionTarget, "enable": 0,
		"time_type": campaign.TimeType, "start_time": campaign.StartTime,
		"expire_time": campaign.ExpireTime, "time_period_type": campaign.TimePeriodType,
		"time_period": campaign.TimePeriod, "bidding_strategy": campaign.BiddingStrategy,
		"limit_day_budget": 1, "campaign_day_budget": campaign.DayBudgetFen,
		"optimize_target": campaign.OptimizeTarget, "constraint_type": campaign.ConstraintType,
		"smart_switch": campaign.SmartSwitch, "pacing_mode": campaign.PacingMode,
		"feed_flag": campaign.FeedFlag, "build_type": campaign.BuildType,
		"event_asset_id": campaign.EventAssetID, "asset_event": campaign.AssetEvent,
		"asset_event_id": campaign.AssetEventID, "page_category": campaign.PageCategory,
		"search_flag": campaign.SearchFlag, "target_extension_switch": campaign.TargetExtensionSwitch,
		"search_bid_ratio": campaign.SearchBidRatio, "deeplink_id": campaign.DeeplinkID,
		"universal_link_id": campaign.UniversalLinkID, "detect_url_link": campaign.DetectURLLink,
	}
	return compactPayload(result)
}

func unitPayload(draft Draft, unit UnitSpec, campaignID int64) map[string]any {
	spuNotes := make([]map[string]any, 0, len(unit.SPUNotes))
	for _, value := range unit.SPUNotes {
		spuNotes = append(spuNotes, map[string]any{"spu_id": value.SPUID, "note_ids": value.NoteIDs})
	}
	keywords := make([]map[string]any, 0, len(unit.Keywords))
	for _, keyword := range unit.Keywords {
		keywords = append(keywords, compactPayload(map[string]any{
			"keyword": keyword.Keyword, "bid": keyword.BidFen, "feed_bid": keyword.FeedBidFen,
			"keyword_source": keyword.KeywordSource, "phrase_match_type": keyword.PhraseMatchType,
		}))
	}
	return compactPayload(map[string]any{
		"advertiser_id": draft.AdvertiserID, "campaign_id": campaignID,
		"unit_name": unit.Name, "event_bid": unit.EventBidFen, "note_ids": unit.NoteIDs,
		"promotion_target": unit.PromotionTarget, "target_type": unit.TargetType,
		"target_config": targetPayload(unit.Target), "keyword_target_period": unit.KeywordTargetPeriod,
		"keyword_target_action": unit.KeywordTargetAction, "business_tree_name": unit.BusinessTreeName,
		"spu_note_info": spuNotes, "keyword_with_bid": keywords,
		"substituted_user_id": unit.SubstitutedUserID, "keyword_gen_type": unit.KeywordGenType,
		"page_id": unit.PageID, "landing_page_url": unit.LandingPageURL,
		"unit_external_page_url": unit.ExternalPageURL, "unit_landing_page_desc": unit.LandingPageDesc,
		"target_template_id": unit.TargetTemplateID,
	})
}

func targetPayload(target TargetSpec) map[string]any {
	content := make([]map[string]string, 0, len(target.ContentInterests))
	for _, value := range target.ContentInterests {
		content = append(content, map[string]string{"code": value.Code, "name": value.Name})
	}
	shopping := make([]map[string]string, 0, len(target.ShoppingInterests))
	for _, value := range target.ShoppingInterests {
		shopping = append(shopping, map[string]string{"code": value.Code, "name": value.Name})
	}
	crowds := make([]map[string]any, 0, len(target.CrowdPackages))
	for _, value := range target.CrowdPackages {
		crowds = append(crowds, map[string]any{"value": value.Value, "name": value.Name, "group_id": value.GroupID, "type": value.Type, "sync_status": value.SyncState, "status": value.Status})
	}
	return compactPayload(map[string]any{
		"target_gender": target.Gender, "target_age": target.Age,
		"target_device": target.Device, "target_city": target.Cities,
		"industry_interest_target": compactPayload(map[string]any{"content_interest": content, "shopping_interest": shopping}),
		"crowd_target":             compactPayload(map[string]any{"crowd_pkg": crowds}),
		"keywords":                 target.BehaviorKeywords, "interest_keywords": target.InterestKeywords,
		"keyword_target_period":                 target.KeywordTargetPeriod,
		"keyword_target_action":                 target.KeywordTargetActions,
		"intelligent_expansion":                 target.IntelligentExpansion,
		"have_reverse_blogger_fan_target":       target.ExcludeBloggerFans,
		"have_reverse_blogger_purchased_target": target.ExcludeBloggerPurchasers,
		"have_brand_recognition_group":          target.IncludeBrandRecognition,
		"have_category_interest_group":          target.IncludeCategoryInterested,
	})
}

func creativityPayload(draft Draft, creative CreativitySpec, unitID int64) map[string]any {
	qualification := map[string]any{}
	if creative.Qualification != nil {
		qualification = map[string]any{
			"apply_id":             creative.Qualification.ApplyID,
			"product_qual_id_list": creative.Qualification.ProductQualIDList,
			"brand_qual_id_list":   creative.Qualification.BrandQualIDList,
		}
	}
	return compactPayload(map[string]any{
		"advertiser_id": draft.AdvertiserID, "unit_id": unitID,
		"creativity_name": creative.Name, "note_id": creative.NoteID,
		"click_urls": creative.ClickURLs, "expo_urls": creative.ExpoURLs,
		"mask_perfer": creative.MaskPrefer, "title_mask_perfer": creative.TitleMaskPrefer,
		"conversion_type": creative.ConversionType, "jump_url": creative.JumpURL,
		"landing_page_type": creative.LandingPageType, "bar_content": creative.BarContent,
		"conversion_component_types": creative.ConversionComponentTypes,
		"comment":                    creative.Comment, "app_comp_icon": creative.AppComponentIcon,
		"fall_back_jump_url": creative.FallbackJumpURL, "qual_info": qualification,
	})
}

func collectDraftNoteIDs(spec DraftSpec) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	for _, noteID := range spec.Notes {
		add(noteID)
	}
	for _, unit := range spec.Units {
		for _, noteID := range unit.NoteIDs {
			add(noteID)
		}
		for _, creative := range unit.Creativities {
			add(creative.NoteID)
		}
	}
	return result
}

func capabilityToMap(value Capability) map[string]any {
	return map[string]any{
		"advertiser_id": value.AdvertiserID, "authorized": value.Authorized,
		"advertiser_allowed": value.AdvertiserAllowed, "scopes": value.Scopes,
		"required_scopes": value.RequiredScopes, "missing_scopes": value.MissingScopes,
		"advertiser_count": value.AdvertiserCount, "media_writes_enabled": value.MediaWritesEnabled,
		"contract_version": value.ContractVersion, "operations": value.Operations,
		"checked_at": value.CheckedAt,
	}
}

func numericInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	default:
		return 0
	}
}

func campaignStatusName(actionType int) string {
	switch actionType {
	case 1:
		return "active"
	case 2:
		return "paused"
	case 3:
		return "deleted"
	default:
		return ""
	}
}

func normalizeCampaignStatusInput(input CampaignStatusInput) (CampaignStatusInput, error) {
	if input.AdvertiserID <= 0 {
		return CampaignStatusInput{}, errors.New("advertiser_id must be positive")
	}
	if len(input.CampaignIDs) == 0 {
		return CampaignStatusInput{}, errors.New("campaign_ids must contain at least one value")
	}
	if len(input.CampaignIDs) > 20 {
		return CampaignStatusInput{}, errors.New("campaign_ids cannot contain more than 20 values")
	}
	seen := make(map[int64]struct{}, len(input.CampaignIDs))
	ids := make([]int64, 0, len(input.CampaignIDs))
	for _, campaignID := range input.CampaignIDs {
		if campaignID <= 0 {
			return CampaignStatusInput{}, errors.New("campaign_ids must contain only positive values")
		}
		if _, exists := seen[campaignID]; exists {
			return CampaignStatusInput{}, errors.New("campaign_ids cannot contain duplicates")
		}
		seen[campaignID] = struct{}{}
		ids = append(ids, campaignID)
	}
	if input.ActionType < 1 || input.ActionType > 3 {
		return CampaignStatusInput{}, errors.New("action_type must be 1, 2, or 3")
	}
	input.CampaignIDs = ids
	return input, nil
}

func campaignIDsFromGateway(data map[string]any) []int64 {
	if len(data) == 0 {
		return nil
	}
	return int64Slice(data["campaign_ids"])
}

func int64Slice(value any) []int64 {
	switch typed := value.(type) {
	case []int64:
		return append([]int64(nil), typed...)
	case []any:
		result := make([]int64, 0, len(typed))
		for _, item := range typed {
			if id := numericInt64(item); id > 0 {
				result = append(result, id)
			}
		}
		return result
	default:
		return nil
	}
}

func findMediaID(data map[string]any, field string) int64 {
	if value := numericInt64(data[field]); value > 0 {
		return value
	}
	if nested, ok := data["data"].(map[string]any); ok {
		return numericInt64(nested[field])
	}
	return 0
}

func countCreativities(spec DraftSpec) int {
	count := 0
	for _, unit := range spec.Units {
		count += len(unit.Creativities)
	}
	return count
}

func appendCreatedEntity(result map[string]any, entityType, localKey string, mediaID, parentMediaID int64) {
	created, _ := result["created_entities"].([]map[string]any)
	created = append(created, map[string]any{"type": entityType, "local_key": localKey, "media_id": mediaID, "parent_media_id": parentMediaID, "status": "paused"})
	result["created_entities"] = created
}

func failJob(job PublishJob, code string, err error, completed time.Time) PublishJob {
	if job.Result == nil {
		job.Result = map[string]any{}
	}
	job.Status = "failed"
	job.ErrorCode = code
	job.ErrorMessage = limitRunes(err.Error(), 2000)
	job.CompletedAt = &completed
	job.Result["executed"] = true
	job.Result["partial_failure"] = true
	job.Result["recovery"] = "created entities remain paused; reconcile upstream state before any new publish request"
	return job
}

func failPreflightJob(job PublishJob, code string, err error, completed time.Time) PublishJob {
	if job.Result == nil {
		job.Result = map[string]any{}
	}
	job.Status = "failed"
	job.CurrentStep = "preflight_failed"
	job.ErrorCode = code
	job.ErrorMessage = limitRunes(err.Error(), 2000)
	job.CompletedAt = &completed
	job.Result["executed"] = false
	job.Result["partial_failure"] = false
	job.Result["recovery"] = "renew capability and approvals, then submit a new idempotent publish request"
	return job
}

func validateReportFilters(filters map[string]any) error {
	if len(filters) > 64 {
		return errors.New("filters cannot contain more than 64 fields")
	}
	reserved := map[string]bool{
		"advertiser_id": true, "start_date": true, "end_date": true,
		"page": true, "page_num": true, "page_size": true, "split_columns": true,
	}
	for key, value := range filters {
		key = strings.TrimSpace(key)
		if key == "" || len(key) > 80 {
			return errors.New("filter names must contain 1 to 80 characters")
		}
		if reserved[key] {
			return fmt.Errorf("filter %q is reserved", key)
		}
		if err := validateReportFilterValue(value, 0); err != nil {
			return fmt.Errorf("filter %q: %w", key, err)
		}
	}
	return nil
}

func validateReportFilterValue(value any, depth int) error {
	if depth > 4 {
		return errors.New("nested values cannot exceed four levels")
	}
	switch typed := value.(type) {
	case nil, bool, string, float64, json.Number:
		return nil
	case []any:
		if len(typed) > 500 {
			return errors.New("arrays cannot contain more than 500 values")
		}
		for _, item := range typed {
			if err := validateReportFilterValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		if len(typed) > 64 {
			return errors.New("objects cannot contain more than 64 fields")
		}
		for key, item := range typed {
			if strings.TrimSpace(key) == "" || len(key) > 80 {
				return errors.New("nested field names must contain 1 to 80 characters")
			}
			if err := validateReportFilterValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported JSON value type %T", value)
	}
}

func compactPayload(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		switch typed := item.(type) {
		case nil:
			continue
		case string:
			if typed == "" {
				continue
			}
		case []string:
			if len(typed) == 0 {
				continue
			}
		case []int:
			if len(typed) == 0 {
				continue
			}
		case []int64:
			if len(typed) == 0 {
				continue
			}
		case []map[string]any:
			if len(typed) == 0 {
				continue
			}
		case []map[string]string:
			if len(typed) == 0 {
				continue
			}
		case map[string]any:
			if len(typed) == 0 {
				continue
			}
		}
		result[key] = item
	}
	return result
}

func sanitizePayload(payload map[string]any) map[string]any {
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		result[key] = sanitizeAuditValue(key, value)
	}
	return result
}

func sanitizeAuditValue(key string, value any) any {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "authorization") || strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") {
		return "[REDACTED]"
	}
	if strings.Contains(lower, "url") || strings.Contains(lower, "link") {
		return "[URL_REDACTED]"
	}
	switch typed := value.(type) {
	case map[string]any:
		return sanitizePayload(typed)
	case []any:
		result := make([]any, len(typed))
		for index, nested := range typed {
			result[index] = sanitizeAuditValue("", nested)
		}
		return result
	case []map[string]any:
		result := make([]map[string]any, len(typed))
		for index, nested := range typed {
			result[index] = sanitizePayload(nested)
		}
		return result
	default:
		return value
	}
}

func summarizeResponse(data map[string]any) map[string]any {
	result := map[string]any{"keys": make([]string, 0, len(data))}
	keys := result["keys"].([]string)
	for key := range data {
		keys = append(keys, key)
		if strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "_ids") || key == "total_count" || key == "crowd_scope" || key == "raw_crowd_num" {
			result[key] = data[key]
		}
	}
	sort.Strings(keys)
	result["keys"] = keys
	return result
}

func verifyReadbackEntity(draft Draft, entity map[string]any, data map[string]any) error {
	entityType, _ := entity["type"].(string)
	localKey, _ := entity["local_key"].(string)
	mediaID := numericInt64(entity["media_id"])
	parentMediaID := numericInt64(entity["parent_media_id"])
	record, ok := findReadbackEntity(data, entityType, mediaID)
	if !ok {
		return fmt.Errorf("%s %d was not found in the upstream readback", entityType, mediaID)
	}
	var statusFields []string
	switch entityType {
	case "campaign":
		statusFields = []string{"campaign_enable", "enable"}
		if err := compareReadbackString(record, []string{"campaign_name", "name"}, draft.Spec.Campaign.Name); err != nil {
			return fmt.Errorf("campaign %d: %w", mediaID, err)
		}
		if err := compareReadbackNumeric(record, []string{"campaign_day_budget"}, draft.Spec.Campaign.DayBudgetFen); err != nil {
			return fmt.Errorf("campaign %d: %w", mediaID, err)
		}
	case "unit":
		statusFields = []string{"enable", "unit_enable"}
		if actual, exists := lookupNumericField(record, "campaign_id"); !exists || actual != parentMediaID {
			return fmt.Errorf("unit %d belongs to campaign %d, expected %d", mediaID, actual, parentMediaID)
		}
		unit, exists := draftUnitByLocalKey(draft.Spec, localKey)
		if !exists {
			return fmt.Errorf("unit %d has unknown local key %q", mediaID, localKey)
		}
		if err := compareReadbackString(record, []string{"unit_name", "name"}, unit.Name); err != nil {
			return fmt.Errorf("unit %d: %w", mediaID, err)
		}
		if err := compareReadbackNumeric(record, []string{"event_bid"}, unit.EventBidFen); err != nil {
			return fmt.Errorf("unit %d: %w", mediaID, err)
		}
	case "creativity":
		statusFields = []string{"creativity_enable", "enable"}
		if actual, exists := lookupNumericField(record, "unit_id"); !exists || actual != parentMediaID {
			return fmt.Errorf("creativity %d belongs to unit %d, expected %d", mediaID, actual, parentMediaID)
		}
		creative, exists := draftCreativityByLocalKey(draft.Spec, localKey)
		if !exists {
			return fmt.Errorf("creativity %d has unknown local key %q", mediaID, localKey)
		}
		if err := compareReadbackString(record, []string{"creativity_name", "creative_name", "name"}, creative.Name); err != nil {
			return fmt.Errorf("creativity %d: %w", mediaID, err)
		}
		if err := compareReadbackString(record, []string{"note_id"}, creative.NoteID); err != nil {
			return fmt.Errorf("creativity %d: %w", mediaID, err)
		}
	default:
		return fmt.Errorf("unknown readback entity type %q", entityType)
	}
	status, exists := lookupNumericField(record, statusFields...)
	if !exists {
		return fmt.Errorf("%s %d readback omitted its enable status", entityType, mediaID)
	}
	if status != 0 {
		return fmt.Errorf("%s %d was unexpectedly active after paused-state creation", entityType, mediaID)
	}
	return nil
}

func findReadbackEntity(value any, entityType string, expected int64) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		identityFields := map[string][]string{
			"campaign":   {"campaign_id"},
			"unit":       {"id", "unit_id"},
			"creativity": {"creativity_id", "creative_id"},
		}[entityType]
		if actual, exists := lookupNumericField(typed, identityFields...); exists && actual == expected {
			return typed, true
		}
		for _, nested := range typed {
			if record, found := findReadbackEntity(nested, entityType, expected); found {
				return record, true
			}
		}
	case []any:
		for _, nested := range typed {
			if record, found := findReadbackEntity(nested, entityType, expected); found {
				return record, true
			}
		}
	}
	return nil, false
}

func lookupNumericField(record map[string]any, fields ...string) (int64, bool) {
	for _, field := range fields {
		value, exists := record[field]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int64(typed), true
		case float32:
			return int64(typed), true
		case int:
			return int64(typed), true
		case int64:
			return typed, true
		case int32:
			return int64(typed), true
		case json.Number:
			parsed, err := typed.Int64()
			if err == nil {
				return parsed, true
			}
		case string:
			parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func compareReadbackNumeric(record map[string]any, fields []string, expected int64) error {
	actual, exists := lookupNumericField(record, fields...)
	if !exists {
		return nil
	}
	if actual != expected {
		return fmt.Errorf("field %s is %d, expected %d", strings.Join(fields, "/"), actual, expected)
	}
	return nil
}

func compareReadbackString(record map[string]any, fields []string, expected string) error {
	for _, field := range fields {
		value, exists := record[field]
		if !exists || value == nil {
			continue
		}
		actual, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %s has a non-string value", field)
		}
		if actual != expected {
			return fmt.Errorf("field %s is %q, expected %q", field, actual, expected)
		}
		return nil
	}
	return nil
}

func draftUnitByLocalKey(spec DraftSpec, localKey string) (UnitSpec, bool) {
	for _, unit := range spec.Units {
		if unit.LocalKey == localKey {
			return unit, true
		}
	}
	return UnitSpec{}, false
}

func draftCreativityByLocalKey(spec DraftSpec, localKey string) (CreativitySpec, bool) {
	for _, unit := range spec.Units {
		for _, creative := range unit.Creativities {
			if creative.LocalKey == localKey {
				return creative, true
			}
		}
	}
	return CreativitySpec{}, false
}
