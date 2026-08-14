package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/delivery"
)

func TestSelfServeDeliveryLifecycleIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "delivery-integration-test")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	advertiserID := int64(8_000_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	keySuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	spec := integrationDraftSpec(advertiserID)
	draft, err := postgres.CreateDraft(ctx, delivery.CreateDraftInput{
		Spec: spec, IdempotencyKey: "draft-" + keySuffix, ChangeReason: "integration create",
	}, delivery.Actor{ID: "integration-submitter", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = postgres.pool.Exec(cleanupCtx, `DELETE FROM delivery_publish_jobs WHERE draft_id=$1`, draft.ID)
		_, _ = postgres.pool.Exec(cleanupCtx, `DELETE FROM delivery_drafts WHERE id=$1`, draft.ID)
	})
	if draft.CurrentVersion != 1 || draft.Status != "draft" {
		t.Fatalf("created draft = %+v", draft)
	}

	idempotent, err := postgres.CreateDraft(ctx, delivery.CreateDraftInput{
		Spec: spec, IdempotencyKey: "draft-" + keySuffix, ChangeReason: "integration duplicate",
	}, delivery.Actor{ID: "integration-submitter", Role: "operator"})
	if err != nil || idempotent.ID != draft.ID {
		t.Fatalf("idempotent draft=%+v error=%v", idempotent, err)
	}

	spec.Campaign.Name = "integration-v2"
	updated, err := postgres.UpdateDraft(ctx, draft.ID, delivery.UpdateDraftInput{
		Spec: spec, ExpectedVersion: 1, ChangeReason: "integration version update",
	}, delivery.Actor{ID: "integration-submitter", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.CurrentVersion != 2 || updated.Spec.Campaign.Name != "integration-v2" {
		t.Fatalf("updated draft = %+v", updated)
	}
	_, err = postgres.UpdateDraft(ctx, draft.ID, delivery.UpdateDraftInput{
		Spec: spec, ExpectedVersion: 1, ChangeReason: "stale update",
	}, delivery.Actor{ID: "integration-submitter", Role: "operator"})
	if !errors.Is(err, delivery.ErrConflict) {
		t.Fatalf("stale UpdateDraft() error = %v", err)
	}

	validationID, _ := delivery.NewID("val")
	validation, err := postgres.SaveValidation(ctx, delivery.Validation{
		ID: validationID, DraftID: updated.ID, DraftVersion: updated.CurrentVersion,
		SpecHash: updated.SpecHash, RulesVersion: delivery.RulesVersion,
		ContractVersion: delivery.MediaContractVersion, Valid: true,
		Errors: []delivery.ValidationIssue{}, Warnings: []delivery.ValidationIssue{},
		CapabilitySnapshot: map[string]any{"authorized": true}, ValidUntil: time.Now().Add(time.Hour),
		CreatedBy: "integration-validator",
	})
	if err != nil || !validation.Valid {
		t.Fatalf("validation=%+v error=%v", validation, err)
	}
	latestValidation, err := postgres.LatestValidation(ctx, updated.ID, updated.CurrentVersion)
	if err != nil || latestValidation.ID != validation.ID {
		t.Fatalf("latest validation=%+v error=%v", latestValidation, err)
	}
	approveIntegrationDraft(t, ctx, postgres, updated)

	jobID, _ := delivery.NewID("job")
	job, err := postgres.CreatePublishJob(ctx, delivery.PublishJob{
		ID: jobID, DraftID: updated.ID, DraftVersion: updated.CurrentVersion,
		AdvertiserID: updated.AdvertiserID, Mode: "execute", Status: "queued",
		CurrentStep: "queued", IdempotencyKey: "publish-preflight-" + keySuffix,
		RequestPreview: map[string]any{"initial_status": "paused"}, Result: map[string]any{},
		RequestedBy: "integration-publisher", RequestedRole: "operator",
	})
	if err != nil || job.Status != "queued" {
		t.Fatalf("publish job=%+v error=%v", job, err)
	}
	jobs, err := postgres.PublishJobs(ctx, updated.ID, updated.CurrentVersion, 20)
	if err != nil || len(jobs) != 1 || jobs[0].ID != job.ID {
		t.Fatalf("publish jobs=%+v error=%v", jobs, err)
	}
	entities, err := postgres.MediaEntities(ctx, updated.ID)
	if err != nil || len(entities) != 0 {
		t.Fatalf("media entities=%+v error=%v", entities, err)
	}
	claimed, ok, err := postgres.ClaimPublishJob(ctx)
	if err != nil || !ok || claimed.ID != job.ID || claimed.Status != "publishing" {
		t.Fatalf("claimed=%+v ok=%v error=%v", claimed, ok, err)
	}
	now := time.Now().UTC()
	claimed.Status = "failed"
	claimed.CurrentStep = "preflight_failed"
	claimed.ErrorCode = "approval_expired"
	claimed.Result = map[string]any{"executed": false, "partial_failure": false}
	claimed.CompletedAt = &now
	if err := postgres.UpdatePublishJob(ctx, claimed); err != nil {
		t.Fatal(err)
	}
	afterFailure, err := postgres.Draft(ctx, updated.ID)
	if err != nil || afterFailure.Status != "pending_approval" {
		t.Fatalf("draft after preflight failure=%+v error=%v", afterFailure, err)
	}

	approveIntegrationDraft(t, ctx, postgres, afterFailure)
	retryID, _ := delivery.NewID("job")
	retry, err := postgres.CreatePublishJob(ctx, delivery.PublishJob{
		ID: retryID, DraftID: afterFailure.ID, DraftVersion: afterFailure.CurrentVersion,
		AdvertiserID: afterFailure.AdvertiserID, Mode: "execute", Status: "queued",
		CurrentStep: "queued", IdempotencyKey: "publish-retry-" + keySuffix,
		RequestPreview: map[string]any{"initial_status": "paused"}, Result: map[string]any{},
		RequestedBy: "integration-publisher", RequestedRole: "operator",
	})
	if err != nil || retry.ID != retryID || retry.Status != "queued" {
		t.Fatalf("retry job=%+v error=%v", retry, err)
	}
	idempotentJob, err := postgres.CreatePublishJob(ctx, retry)
	if err != nil || idempotentJob.ID != retry.ID {
		t.Fatalf("idempotent publish=%+v error=%v", idempotentJob, err)
	}
}

func approveIntegrationDraft(t *testing.T, ctx context.Context, postgres *Postgres, draft delivery.Draft) {
	t.Helper()
	for index, value := range []struct {
		role  string
		actor string
	}{{"operator", "integration-approver"}, {"budget_owner", "integration-budget-owner"}} {
		approvalID, _ := delivery.NewID("apr")
		_, err := postgres.SaveApproval(ctx, delivery.Approval{
			ID: approvalID, DraftID: draft.ID, DraftVersion: draft.CurrentVersion,
			SpecHash: draft.SpecHash, Role: value.role, Decision: "approved", Actor: value.actor,
			ApprovedBudgetFen: draft.Spec.Budget.TotalLimitFen,
			ExpiresAt:         time.Now().Add(time.Hour + time.Duration(index)*time.Minute),
		})
		if err != nil {
			t.Fatalf("approve %s: %v", value.role, err)
		}
	}
	approved, err := postgres.Draft(ctx, draft.ID)
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approved draft=%+v error=%v", approved, err)
	}
}

func integrationDraftSpec(advertiserID int64) delivery.DraftSpec {
	return delivery.DraftSpec{
		AdvertiserID: advertiserID, Objective: "integration", Placement: "search",
		Budget: delivery.BudgetPolicy{DailyLimitFen: 10_000, TotalLimitFen: 20_000},
		Campaign: delivery.CampaignSpec{
			LocalKey: "campaign", Name: "integration-v1", MarketingTarget: 4,
			Placement: 2, PromotionTarget: 1, Enable: 0, TimeType: 0,
			TimePeriodType: 0, BiddingStrategy: 2, LimitDayBudget: 1,
			DayBudgetFen: 10_000, OptimizeTarget: 1,
		},
		Units: []delivery.UnitSpec{{
			LocalKey: "unit", Name: "integration-unit", PromotionTarget: 1, TargetType: 1,
			Target: delivery.TargetSpec{}, Creativities: []delivery.CreativitySpec{{
				LocalKey: "creative", Name: "integration-creative", NoteID: "0123456789abcdef01234567",
			}},
		}},
		Experiment: delivery.ExperimentSpec{PrimaryMetric: "conversion"},
	}
}
