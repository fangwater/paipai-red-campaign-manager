package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/delivery"

	"github.com/jackc/pgx/v5"
)

func (p *Postgres) CreateDraft(ctx context.Context, input delivery.CreateDraftInput, actor delivery.Actor) (delivery.Draft, error) {
	spec := delivery.NormalizeSpec(input.Spec)
	hash, encoded, err := delivery.HashSpec(spec)
	if err != nil {
		return delivery.Draft{}, err
	}
	draftID, err := delivery.NewID("drf")
	if err != nil {
		return delivery.Draft{}, err
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return delivery.Draft{}, fmt.Errorf("begin delivery draft transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	command, err := tx.Exec(ctx, `
		INSERT INTO delivery_drafts (
			id, advertiser_id, status, current_version, spec, spec_hash,
			idempotency_key, created_by, updated_by
		) VALUES ($1,$2,'draft',1,$3::jsonb,$4,$5,$6,$6)
		ON CONFLICT (advertiser_id, idempotency_key) DO NOTHING
	`, draftID, spec.AdvertiserID, string(encoded), hash, input.IdempotencyKey, actor.ID)
	if err != nil {
		return delivery.Draft{}, fmt.Errorf("insert delivery draft: %w", err)
	}
	if command.RowsAffected() == 0 {
		existing, loadErr := draftByIdempotencyTx(ctx, tx, spec.AdvertiserID, input.IdempotencyKey)
		if loadErr != nil {
			return delivery.Draft{}, loadErr
		}
		if existing.SpecHash != hash {
			return delivery.Draft{}, fmt.Errorf("%w: idempotency key belongs to a different draft payload", delivery.ErrConflict)
		}
		if err := tx.Commit(ctx); err != nil {
			return delivery.Draft{}, fmt.Errorf("commit delivery draft lookup: %w", err)
		}
		return existing, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO delivery_draft_versions (
			draft_id, version, spec, spec_hash, change_reason, created_by
		) VALUES ($1,1,$2::jsonb,$3,$4,$5)
	`, draftID, string(encoded), hash, input.ChangeReason, actor.ID); err != nil {
		return delivery.Draft{}, fmt.Errorf("insert delivery draft version: %w", err)
	}
	result, err := draftTx(ctx, tx, draftID, false)
	if err != nil {
		return delivery.Draft{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return delivery.Draft{}, fmt.Errorf("commit delivery draft: %w", err)
	}
	return result, nil
}

func (p *Postgres) UpdateDraft(ctx context.Context, draftID string, input delivery.UpdateDraftInput, actor delivery.Actor) (delivery.Draft, error) {
	spec := delivery.NormalizeSpec(input.Spec)
	hash, encoded, err := delivery.HashSpec(spec)
	if err != nil {
		return delivery.Draft{}, err
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return delivery.Draft{}, fmt.Errorf("begin delivery draft update: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, err := draftTx(ctx, tx, draftID, true)
	if err != nil {
		return delivery.Draft{}, err
	}
	if current.CurrentVersion != input.ExpectedVersion {
		return delivery.Draft{}, fmt.Errorf("%w: expected version %d, current version is %d", delivery.ErrConflict, input.ExpectedVersion, current.CurrentVersion)
	}
	if current.Status == "publishing" || current.Status == "paused" || current.Status == "active" || current.Status == "failed" || current.Status == "cancelled" {
		return delivery.Draft{}, fmt.Errorf("%w: draft in status %s cannot be edited", delivery.ErrConflict, current.Status)
	}
	if current.AdvertiserID != spec.AdvertiserID {
		return delivery.Draft{}, fmt.Errorf("%w: advertiser_id is immutable", delivery.ErrConflict)
	}
	version := current.CurrentVersion + 1
	if _, err := tx.Exec(ctx, `
		INSERT INTO delivery_draft_versions (
			draft_id, version, spec, spec_hash, change_reason, created_by
		) VALUES ($1,$2,$3::jsonb,$4,$5,$6)
	`, draftID, version, string(encoded), hash, input.ChangeReason, actor.ID); err != nil {
		return delivery.Draft{}, fmt.Errorf("insert delivery draft version: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE delivery_drafts
		SET status='draft', current_version=$2, spec=$3::jsonb, spec_hash=$4,
			updated_by=$5, updated_at=NOW()
		WHERE id=$1
	`, draftID, version, string(encoded), hash, actor.ID); err != nil {
		return delivery.Draft{}, fmt.Errorf("update delivery draft: %w", err)
	}
	result, err := draftTx(ctx, tx, draftID, false)
	if err != nil {
		return delivery.Draft{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return delivery.Draft{}, fmt.Errorf("commit delivery draft update: %w", err)
	}
	return result, nil
}

func (p *Postgres) Draft(ctx context.Context, draftID string) (delivery.Draft, error) {
	return scanDeliveryDraft(p.pool.QueryRow(ctx, deliveryDraftSelect+" WHERE id=$1", draftID))
}

func (p *Postgres) Drafts(ctx context.Context, advertiserID int64, limit int) ([]delivery.Draft, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := p.pool.Query(ctx, deliveryDraftSelect+`
		WHERE ($1::bigint = 0 OR advertiser_id=$1)
		ORDER BY updated_at DESC LIMIT $2
	`, advertiserID, limit)
	if err != nil {
		return nil, fmt.Errorf("query delivery drafts: %w", err)
	}
	defer rows.Close()
	result := make([]delivery.Draft, 0)
	for rows.Next() {
		draft, err := scanDeliveryDraft(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, draft)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery drafts: %w", err)
	}
	return result, nil
}

func (p *Postgres) SaveRecommendation(ctx context.Context, value delivery.Recommendation) (delivery.Recommendation, error) {
	payload, err := json.Marshal(value.Payload)
	if err != nil {
		return delivery.Recommendation{}, fmt.Errorf("encode delivery recommendation: %w", err)
	}
	err = p.pool.QueryRow(ctx, `
		INSERT INTO delivery_recommendations (
			id, draft_id, draft_version, schema_version, llm_provider, llm_model,
			ranker_family, ranker_version, rules_version, payload, warnings, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12)
		RETURNING created_at
	`, value.ID, value.DraftID, value.DraftVersion, value.SchemaVersion, value.LLMProvider,
		value.LLMModel, value.RankerFamily, value.RankerVersion, value.RulesVersion,
		string(payload), value.Warnings, value.CreatedBy).Scan(&value.CreatedAt)
	if err != nil {
		return delivery.Recommendation{}, fmt.Errorf("save delivery recommendation: %w", err)
	}
	return value, nil
}

func (p *Postgres) LatestRecommendation(ctx context.Context, draftID string, version int) (delivery.Recommendation, error) {
	return scanDeliveryRecommendation(p.pool.QueryRow(ctx, `
		SELECT id,draft_id,draft_version,schema_version,llm_provider,llm_model,
			ranker_family,ranker_version,rules_version,payload,warnings,created_by,created_at
		FROM delivery_recommendations
		WHERE draft_id=$1 AND draft_version=$2
		ORDER BY created_at DESC,id DESC LIMIT 1
	`, draftID, version))
}

func (p *Postgres) SaveValidation(ctx context.Context, value delivery.Validation) (delivery.Validation, error) {
	errorsJSON, err := json.Marshal(value.Errors)
	if err != nil {
		return delivery.Validation{}, err
	}
	warningsJSON, err := json.Marshal(value.Warnings)
	if err != nil {
		return delivery.Validation{}, err
	}
	capabilityJSON, err := json.Marshal(value.CapabilitySnapshot)
	if err != nil {
		return delivery.Validation{}, err
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return delivery.Validation{}, fmt.Errorf("begin delivery validation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	err = tx.QueryRow(ctx, `
		INSERT INTO delivery_validations (
			id, draft_id, draft_version, spec_hash, rules_version, contract_version,
			valid, errors, warnings, capability_snapshot, valid_until, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb,$11,$12)
		RETURNING created_at
	`, value.ID, value.DraftID, value.DraftVersion, value.SpecHash, value.RulesVersion,
		value.ContractVersion, value.Valid, string(errorsJSON), string(warningsJSON),
		string(capabilityJSON), value.ValidUntil, value.CreatedBy).Scan(&value.CreatedAt)
	if err != nil {
		return delivery.Validation{}, fmt.Errorf("save delivery validation: %w", err)
	}
	status := "draft"
	if value.Valid {
		status = "validated"
	}
	command, err := tx.Exec(ctx, `
			UPDATE delivery_drafts SET
			status=CASE
				WHEN $2='validated' AND status IN ('pending_approval','approved') THEN status
				ELSE $2
			END,
			updated_at=NOW()
			WHERE id=$1 AND current_version=$3 AND spec_hash=$4 AND status NOT IN ('publishing','paused','active','failed','cancelled')
		`, value.DraftID, status, value.DraftVersion, value.SpecHash)
	if err != nil {
		return delivery.Validation{}, fmt.Errorf("update validated delivery draft: %w", err)
	}
	if command.RowsAffected() != 1 {
		return delivery.Validation{}, fmt.Errorf("%w: draft changed while validation was running", delivery.ErrConflict)
	}
	if err := tx.Commit(ctx); err != nil {
		return delivery.Validation{}, fmt.Errorf("commit delivery validation: %w", err)
	}
	return value, nil
}

func (p *Postgres) LatestValidation(ctx context.Context, draftID string, version int) (delivery.Validation, error) {
	return scanDeliveryValidation(p.pool.QueryRow(ctx, `
		SELECT id,draft_id,draft_version,spec_hash,rules_version,contract_version,
			valid,errors,warnings,capability_snapshot,valid_until,created_by,created_at
		FROM delivery_validations
		WHERE draft_id=$1 AND draft_version=$2
		ORDER BY created_at DESC,id DESC LIMIT 1
	`, draftID, version))
}

func (p *Postgres) SaveApproval(ctx context.Context, value delivery.Approval) (delivery.Approval, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return delivery.Approval{}, fmt.Errorf("begin delivery approval: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, err := draftTx(ctx, tx, value.DraftID, true)
	if err != nil {
		return delivery.Approval{}, err
	}
	if current.CurrentVersion != value.DraftVersion || current.SpecHash != value.SpecHash {
		return delivery.Approval{}, delivery.ErrApprovalStale
	}
	if current.Status != "validated" && current.Status != "pending_approval" && current.Status != "approved" {
		return delivery.Approval{}, fmt.Errorf("%w: draft must be validated before approval", delivery.ErrConflict)
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO delivery_approvals (
			id, draft_id, draft_version, spec_hash, role, decision, actor,
			comment, approved_budget_fen, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING created_at
	`, value.ID, value.DraftID, value.DraftVersion, value.SpecHash, value.Role,
		value.Decision, value.Actor, value.Comment, value.ApprovedBudgetFen, value.ExpiresAt).Scan(&value.CreatedAt)
	if err != nil {
		return delivery.Approval{}, fmt.Errorf("save delivery approval: %w", err)
	}
	status := "pending_approval"
	if value.Decision == "rejected" {
		status = "draft"
	} else {
		var approvalRoleCount, approvalActorCount int
		if err := tx.QueryRow(ctx, `
			WITH latest AS (
				SELECT DISTINCT ON (role) role,actor,decision,expires_at
				FROM delivery_approvals
				WHERE draft_id=$1 AND draft_version=$2 AND spec_hash=$3
				ORDER BY role,created_at DESC,id DESC
			)
			SELECT COUNT(DISTINCT role), COUNT(DISTINCT actor) FROM latest
			WHERE decision='approved' AND expires_at > NOW()
		`, value.DraftID, value.DraftVersion, value.SpecHash).Scan(&approvalRoleCount, &approvalActorCount); err != nil {
			return delivery.Approval{}, fmt.Errorf("count delivery approvals: %w", err)
		}
		if approvalRoleCount == 2 && approvalActorCount == 2 {
			status = "approved"
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE delivery_drafts SET status=$2,updated_at=NOW() WHERE id=$1`, value.DraftID, status); err != nil {
		return delivery.Approval{}, fmt.Errorf("update delivery approval status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return delivery.Approval{}, fmt.Errorf("commit delivery approval: %w", err)
	}
	return value, nil
}

func (p *Postgres) Approvals(ctx context.Context, draftID string, version int) ([]delivery.Approval, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,draft_id,draft_version,spec_hash,role,decision,actor,comment,
			approved_budget_fen,expires_at,created_at
		FROM (
			SELECT DISTINCT ON (role) id,draft_id,draft_version,spec_hash,role,
				decision,actor,comment,approved_budget_fen,expires_at,created_at
			FROM delivery_approvals
			WHERE draft_id=$1 AND draft_version=$2
			ORDER BY role,created_at DESC,id DESC
		) latest
		ORDER BY role
	`, draftID, version)
	if err != nil {
		return nil, fmt.Errorf("query delivery approvals: %w", err)
	}
	defer rows.Close()
	result := make([]delivery.Approval, 0)
	for rows.Next() {
		var value delivery.Approval
		if err := rows.Scan(&value.ID, &value.DraftID, &value.DraftVersion, &value.SpecHash,
			&value.Role, &value.Decision, &value.Actor, &value.Comment,
			&value.ApprovedBudgetFen, &value.ExpiresAt, &value.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan delivery approval: %w", err)
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) CreatePublishJob(ctx context.Context, value delivery.PublishJob) (delivery.PublishJob, error) {
	preview, err := json.Marshal(value.RequestPreview)
	if err != nil {
		return delivery.PublishJob{}, err
	}
	resultJSON, err := json.Marshal(value.Result)
	if err != nil {
		return delivery.PublishJob{}, err
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return delivery.PublishJob{}, fmt.Errorf("begin delivery publish job: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, err := draftTx(ctx, tx, value.DraftID, true)
	if err != nil {
		return delivery.PublishJob{}, err
	}
	if current.CurrentVersion != value.DraftVersion || current.AdvertiserID != value.AdvertiserID {
		return delivery.PublishJob{}, fmt.Errorf("%w: draft changed before the publish job was created", delivery.ErrConflict)
	}
	existingByKey, existingByKeyErr := publishJobByIdempotencyTx(ctx, tx, value.IdempotencyKey)
	if existingByKeyErr == nil {
		if existingByKey.DraftID != value.DraftID || existingByKey.DraftVersion != value.DraftVersion || existingByKey.AdvertiserID != value.AdvertiserID || existingByKey.Mode != value.Mode {
			return delivery.PublishJob{}, fmt.Errorf("%w: publish idempotency key belongs to another request", delivery.ErrConflict)
		}
		if err := tx.Commit(ctx); err != nil {
			return delivery.PublishJob{}, fmt.Errorf("commit delivery publish lookup: %w", err)
		}
		return existingByKey, nil
	}
	if !errors.Is(existingByKeyErr, delivery.ErrNotFound) {
		return delivery.PublishJob{}, existingByKeyErr
	}
	if value.Mode == "execute" && current.Status != "approved" {
		return delivery.PublishJob{}, fmt.Errorf("%w: execute publish requires an approved draft", delivery.ErrApprovalRequired)
	}
	if value.Mode == "execute" {
		existing, existingErr := publishJobForDraftVersionTx(ctx, tx, value.DraftID, value.DraftVersion)
		if existingErr == nil {
			if existing.IdempotencyKey == value.IdempotencyKey {
				if err := tx.Commit(ctx); err != nil {
					return delivery.PublishJob{}, fmt.Errorf("commit delivery publish lookup: %w", err)
				}
				return existing, nil
			}
			return delivery.PublishJob{}, fmt.Errorf("%w: draft version already has an execute publish job", delivery.ErrConflict)
		}
		if !errors.Is(existingErr, delivery.ErrNotFound) {
			return delivery.PublishJob{}, existingErr
		}
	}
	command, err := tx.Exec(ctx, `
		INSERT INTO delivery_publish_jobs (
			id,draft_id,draft_version,advertiser_id,mode,status,current_step,
			idempotency_key,request_preview,result,requested_by,requested_role
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11,$12)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, value.ID, value.DraftID, value.DraftVersion, value.AdvertiserID, value.Mode,
		value.Status, value.CurrentStep, value.IdempotencyKey, string(preview), string(resultJSON), value.RequestedBy, value.RequestedRole)
	if err != nil {
		return delivery.PublishJob{}, fmt.Errorf("create delivery publish job: %w", err)
	}
	if command.RowsAffected() == 0 {
		existing, err := publishJobByIdempotencyTx(ctx, tx, value.IdempotencyKey)
		if err != nil {
			return delivery.PublishJob{}, err
		}
		if existing.DraftID != value.DraftID || existing.DraftVersion != value.DraftVersion || existing.AdvertiserID != value.AdvertiserID || existing.Mode != value.Mode {
			return delivery.PublishJob{}, fmt.Errorf("%w: publish idempotency key belongs to another request", delivery.ErrConflict)
		}
		if err := tx.Commit(ctx); err != nil {
			return delivery.PublishJob{}, fmt.Errorf("commit delivery publish lookup: %w", err)
		}
		return existing, nil
	}
	if value.Mode == "execute" {
		if _, err := tx.Exec(ctx, `UPDATE delivery_drafts SET status='publishing',updated_at=NOW() WHERE id=$1`, value.DraftID); err != nil {
			return delivery.PublishJob{}, fmt.Errorf("lock delivery draft for publishing: %w", err)
		}
	}
	result, err := scanPublishJob(tx.QueryRow(ctx, deliveryPublishJobSelect+" WHERE id=$1", value.ID))
	if err != nil {
		return delivery.PublishJob{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return delivery.PublishJob{}, fmt.Errorf("commit delivery publish job: %w", err)
	}
	return result, nil
}

func (p *Postgres) PublishJob(ctx context.Context, jobID string) (delivery.PublishJob, error) {
	return scanPublishJob(p.pool.QueryRow(ctx, deliveryPublishJobSelect+" WHERE id=$1", jobID))
}

func (p *Postgres) PublishJobs(ctx context.Context, draftID string, version, limit int) ([]delivery.PublishJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := p.pool.Query(ctx, deliveryPublishJobSelect+`
		WHERE draft_id=$1 AND draft_version=$2 ORDER BY created_at DESC,id DESC LIMIT $3
	`, draftID, version, limit)
	if err != nil {
		return nil, fmt.Errorf("query delivery publish jobs: %w", err)
	}
	defer rows.Close()
	result := make([]delivery.PublishJob, 0)
	for rows.Next() {
		value, scanErr := scanPublishJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) PublishJobByIdempotency(ctx context.Context, key string) (delivery.PublishJob, error) {
	return scanPublishJob(p.pool.QueryRow(ctx, deliveryPublishJobSelect+" WHERE idempotency_key=$1", key))
}

func (p *Postgres) ClaimPublishJob(ctx context.Context) (delivery.PublishJob, bool, error) {
	row := p.pool.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id FROM delivery_publish_jobs
			WHERE mode='execute' AND status='queued'
			ORDER BY created_at,id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE delivery_publish_jobs AS jobs
		SET status='publishing',current_step='claimed',started_at=COALESCE(started_at,NOW()),updated_at=NOW()
		FROM candidate
		WHERE jobs.id=candidate.id
		RETURNING jobs.id,jobs.draft_id,jobs.draft_version,jobs.advertiser_id,jobs.mode,
			jobs.status,jobs.current_step,jobs.idempotency_key,jobs.request_preview,jobs.result,
			jobs.error_code,jobs.error_message,jobs.retry_count,jobs.requested_by,jobs.requested_role,
			jobs.created_at,jobs.started_at,jobs.completed_at,jobs.updated_at
	`)
	value, err := scanPublishJob(row)
	if errors.Is(err, delivery.ErrNotFound) {
		return delivery.PublishJob{}, false, nil
	}
	if err != nil {
		return delivery.PublishJob{}, false, fmt.Errorf("claim delivery publish job: %w", err)
	}
	return value, true, nil
}

func (p *Postgres) FailInterruptedPublishJobs(ctx context.Context, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "delivery service restarted before publishing completed"
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin interrupted delivery publish cleanup: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `
		UPDATE delivery_publish_jobs
		SET status='failed',current_step='interrupted',error_code='worker_interrupted',
			error_message=$1,completed_at=NOW(),updated_at=NOW(),
			result=result || '{"executed":true,"partial_failure":true,"recovery":"created entities remain paused; reconcile upstream state before any new publish request"}'::jsonb
		WHERE mode='execute' AND status='publishing'
		RETURNING draft_id
	`, reason)
	if err != nil {
		return fmt.Errorf("fail interrupted delivery publish jobs: %w", err)
	}
	draftIDs := make([]string, 0)
	for rows.Next() {
		var draftID string
		if err := rows.Scan(&draftID); err != nil {
			rows.Close()
			return fmt.Errorf("scan interrupted delivery draft: %w", err)
		}
		draftIDs = append(draftIDs, draftID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate interrupted delivery drafts: %w", err)
	}
	rows.Close()
	if len(draftIDs) > 0 {
		if _, err := tx.Exec(ctx, `UPDATE delivery_drafts SET status='failed',updated_at=NOW() WHERE id=ANY($1::text[])`, draftIDs); err != nil {
			return fmt.Errorf("fail interrupted delivery drafts: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit interrupted delivery publish cleanup: %w", err)
	}
	return nil
}

func publishJobByIdempotencyTx(ctx context.Context, tx pgx.Tx, key string) (delivery.PublishJob, error) {
	return scanPublishJob(tx.QueryRow(ctx, deliveryPublishJobSelect+" WHERE idempotency_key=$1", key))
}

func publishJobForDraftVersionTx(ctx context.Context, tx pgx.Tx, draftID string, version int) (delivery.PublishJob, error) {
	return scanPublishJob(tx.QueryRow(ctx, deliveryPublishJobSelect+`
		WHERE draft_id=$1 AND draft_version=$2 AND mode='execute'
		  AND (status IN ('queued','publishing','succeeded')
		       OR (status='failed' AND COALESCE((result->>'executed')::boolean,true)))
		ORDER BY created_at DESC LIMIT 1
	`, draftID, version))
}

func (p *Postgres) UpdatePublishJob(ctx context.Context, value delivery.PublishJob) error {
	resultJSON, err := json.Marshal(value.Result)
	if err != nil {
		return err
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin delivery publish job update: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	command, err := tx.Exec(ctx, `
		UPDATE delivery_publish_jobs SET status=$2,current_step=$3,result=$4::jsonb,
			error_code=$5,error_message=$6,retry_count=$7,
			started_at=$8,completed_at=$9,updated_at=NOW()
		WHERE id=$1
	`, value.ID, value.Status, value.CurrentStep, string(resultJSON), value.ErrorCode,
		value.ErrorMessage, value.RetryCount, value.StartedAt, value.CompletedAt)
	if err != nil {
		return fmt.Errorf("update delivery publish job: %w", err)
	}
	if command.RowsAffected() != 1 {
		return delivery.ErrNotFound
	}
	if value.Mode == "execute" {
		draftStatus := ""
		switch value.Status {
		case "queued", "publishing":
			draftStatus = "publishing"
		case "succeeded":
			draftStatus = "paused"
		case "failed":
			executed, _ := value.Result["executed"].(bool)
			if executed {
				draftStatus = "failed"
			} else {
				draftStatus = "pending_approval"
			}
		}
		if draftStatus != "" {
			if _, err := tx.Exec(ctx, `UPDATE delivery_drafts SET status=$2,updated_at=NOW() WHERE id=$1`, value.DraftID, draftStatus); err != nil {
				return fmt.Errorf("update delivery draft publish status: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delivery publish job update: %w", err)
	}
	return nil
}

func (p *Postgres) SaveMediaEntity(ctx context.Context, value delivery.MediaEntity) (delivery.MediaEntity, error) {
	payload, err := json.Marshal(value.UpstreamPayload)
	if err != nil {
		return delivery.MediaEntity{}, err
	}
	err = p.pool.QueryRow(ctx, `
		INSERT INTO delivery_media_entities (
			id,job_id,draft_id,advertiser_id,entity_type,local_key,parent_local_key,
			media_id,parent_media_id,desired_status,observed_status,upstream_payload
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,0),$10,$11,$12::jsonb)
		ON CONFLICT (job_id,entity_type,local_key) DO UPDATE SET
			media_id=EXCLUDED.media_id,parent_media_id=EXCLUDED.parent_media_id,
			observed_status=EXCLUDED.observed_status,upstream_payload=EXCLUDED.upstream_payload,updated_at=NOW()
		RETURNING created_at,updated_at
	`, value.ID, value.JobID, value.DraftID, value.AdvertiserID, value.EntityType,
		value.LocalKey, value.ParentLocalKey, value.MediaID, value.ParentMediaID,
		value.DesiredStatus, value.ObservedStatus, string(payload)).Scan(&value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return delivery.MediaEntity{}, fmt.Errorf("save delivery media entity: %w", err)
	}
	return value, nil
}

func (p *Postgres) MediaEntity(ctx context.Context, advertiserID int64, entityType string, mediaID int64) (delivery.MediaEntity, error) {
	var value delivery.MediaEntity
	var payload []byte
	err := p.pool.QueryRow(ctx, `
		SELECT id,job_id,draft_id,advertiser_id,entity_type,local_key,parent_local_key,
			media_id,COALESCE(parent_media_id,0),desired_status,observed_status,
			upstream_payload,created_at,updated_at
		FROM delivery_media_entities
		WHERE advertiser_id=$1 AND entity_type=$2 AND media_id=$3
	`, advertiserID, entityType, mediaID).Scan(
		&value.ID, &value.JobID, &value.DraftID, &value.AdvertiserID, &value.EntityType,
		&value.LocalKey, &value.ParentLocalKey, &value.MediaID, &value.ParentMediaID,
		&value.DesiredStatus, &value.ObservedStatus, &payload, &value.CreatedAt, &value.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return delivery.MediaEntity{}, delivery.ErrNotFound
	}
	if err != nil {
		return delivery.MediaEntity{}, fmt.Errorf("query delivery media entity: %w", err)
	}
	if err := json.Unmarshal(payload, &value.UpstreamPayload); err != nil {
		return delivery.MediaEntity{}, fmt.Errorf("decode delivery media entity: %w", err)
	}
	return value, nil
}

func (p *Postgres) MediaEntities(ctx context.Context, draftID string) ([]delivery.MediaEntity, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,job_id,draft_id,advertiser_id,entity_type,local_key,parent_local_key,
			media_id,COALESCE(parent_media_id,0),desired_status,observed_status,
			upstream_payload,created_at,updated_at
		FROM delivery_media_entities
		WHERE draft_id=$1 ORDER BY created_at,entity_type,local_key
	`, draftID)
	if err != nil {
		return nil, fmt.Errorf("query delivery media entities: %w", err)
	}
	defer rows.Close()
	result := make([]delivery.MediaEntity, 0)
	for rows.Next() {
		value, scanErr := scanMediaEntity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateMediaEntityStatus(ctx context.Context, entityID, status string) error {
	if status != "paused" && status != "active" && status != "deleted" {
		return errors.New("invalid delivery media status")
	}
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin delivery media status update: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var draftID, entityType string
	err = tx.QueryRow(ctx, `
		UPDATE delivery_media_entities
		SET desired_status=$2,observed_status=$2,updated_at=NOW()
		WHERE id=$1
		RETURNING draft_id,entity_type
	`, entityID, status).Scan(&draftID, &entityType)
	if errors.Is(err, pgx.ErrNoRows) {
		return delivery.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("update delivery media status: %w", err)
	}
	if entityType == "campaign" && status != "deleted" {
		if _, err := tx.Exec(ctx, `UPDATE delivery_drafts SET status=$2,updated_at=NOW() WHERE id=$1`, draftID, status); err != nil {
			return fmt.Errorf("update delivery draft activation status: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delivery media status update: %w", err)
	}
	return nil
}

func (p *Postgres) SaveAPIAttempt(ctx context.Context, value delivery.APIAttempt) error {
	requestJSON, err := json.Marshal(value.RequestSummary)
	if err != nil {
		return err
	}
	responseJSON, err := json.Marshal(value.ResponseSummary)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO delivery_api_attempts (
			job_id,advertiser_id,operation,contract_version,request_hash,
			request_summary,response_summary,upstream_request_id,success,
			error_code,error_message,latency_ms
		) VALUES (NULLIF($1,''),$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8,$9,$10,$11,$12)
	`, value.JobID, value.AdvertiserID, value.Operation, value.ContractVersion,
		value.RequestHash, string(requestJSON), string(responseJSON), value.UpstreamRequestID,
		value.Success, value.ErrorCode, value.ErrorMessage, value.LatencyMS)
	if err != nil {
		return fmt.Errorf("save delivery API attempt: %w", err)
	}
	return nil
}

func (p *Postgres) SavePerformanceSnapshot(ctx context.Context, query delivery.PerformanceQuery, payload map[string]any, contractVersion string) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO delivery_performance_snapshots (
			advertiser_id,level,realtime,start_date,end_date,contract_version,payload
		) VALUES ($1,$2,$3,$4::date,$5::date,$6,$7::jsonb)
	`, query.AdvertiserID, query.Level, query.Realtime, query.StartDate, query.EndDate, contractVersion, string(encoded))
	if err != nil {
		return fmt.Errorf("save delivery performance snapshot: %w", err)
	}
	return nil
}

func (p *Postgres) Assets(ctx context.Context, query delivery.AssetQuery) (delivery.Assets, error) {
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 50
	}
	result := delivery.Assets{AdvertiserID: query.AdvertiserID, Notes: []delivery.CandidateNote{}, GeneratedAt: time.Now().UTC()}
	rows, err := p.pool.Query(ctx, `
		WITH execution_tags AS (
			SELECT note_id,
				COALESCE(ARRAY_AGG(DISTINCT BTRIM(audience)) FILTER (WHERE NULLIF(BTRIM(audience),'') IS NOT NULL),'{}'::text[]) audience,
				COALESCE(ARRAY_AGG(DISTINCT BTRIM(user_scenario)) FILTER (WHERE NULLIF(BTRIM(user_scenario),'') IS NOT NULL),'{}'::text[]) scenarios,
				COALESCE(ARRAY_AGG(DISTINCT BTRIM(note_type)) FILTER (WHERE NULLIF(BTRIM(note_type),'') IS NOT NULL),'{}'::text[]) note_types
			FROM service_provider_note_executions WHERE deleted_at IS NULL AND note_id IS NOT NULL GROUP BY note_id
		), performance AS (
			SELECT note_id,SUM(spend)::double precision spend,SUM(search_users)::bigint users,
				CASE WHEN SUM(search_users)>0 THEN SUM(spend)/SUM(search_users) END::double precision cost
			FROM maituo_customer_daily_notes WHERE deleted_at IS NULL GROUP BY note_id
		), media AS (
			SELECT note_id,COUNT(*)::integer creativity_count
			FROM xhs_jg_creativities WHERE advertiser_id=$1 AND deleted_at IS NULL GROUP BY note_id
		)
		SELECT notes.note_id,COALESCE(NULLIF(BTRIM(notes.source_title),''),LEFT(notes.note_content,80)),
			notes.note_content,COALESCE(tags.audience,'{}'),COALESCE(tags.scenarios,'{}'),
			COALESCE(tags.note_types,'{}'),COALESCE(performance.spend,0),
			COALESCE(performance.users,0),performance.cost,
			COALESCE(media.creativity_count,0)>0,COALESCE(media.creativity_count,0),
			COUNT(*) OVER()::integer
		FROM service_provider_notes notes
		LEFT JOIN execution_tags tags USING(note_id)
		LEFT JOIN performance USING(note_id)
		LEFT JOIN media USING(note_id)
		WHERE ($2='' OR notes.note_id ILIKE $3 OR notes.note_content ILIKE $3 OR notes.source_title ILIKE $3)
		ORDER BY COALESCE(performance.users,0) DESC,COALESCE(performance.spend,0) DESC,notes.note_id
		LIMIT $4
	`, query.AdvertiserID, strings.TrimSpace(query.Search), "%"+strings.TrimSpace(query.Search)+"%", query.Limit)
	if err != nil {
		return result, fmt.Errorf("query delivery assets: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var note delivery.CandidateNote
		if err := rows.Scan(&note.NoteID, &note.Title, &note.Content, &note.Audience, &note.Scenarios,
			&note.NoteTypes, &note.HistoricalSpend, &note.HistoricalUsers, &note.HistoricalCost,
			&note.Published, &note.CreativityCount, &result.Count); err != nil {
			return result, fmt.Errorf("scan delivery asset: %w", err)
		}
		result.Notes = append(result.Notes, note)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate delivery assets: %w", err)
	}
	return result, nil
}

func (p *Postgres) RecommendationCandidates(ctx context.Context, noteIDs []string) ([]delivery.CandidateNote, error) {
	rows, err := p.pool.Query(ctx, `
		WITH performance AS (
			SELECT note_id,SUM(spend)::double precision spend,SUM(search_users)::bigint users,
				CASE WHEN SUM(search_users)>0 THEN SUM(spend)/SUM(search_users) END::double precision cost
			FROM maituo_customer_daily_notes WHERE deleted_at IS NULL GROUP BY note_id
		), tags AS (
			SELECT note_id,
				COALESCE(ARRAY_AGG(DISTINCT BTRIM(audience)) FILTER (WHERE NULLIF(BTRIM(audience),'') IS NOT NULL),'{}'::text[]) audience,
				COALESCE(ARRAY_AGG(DISTINCT BTRIM(user_scenario)) FILTER (WHERE NULLIF(BTRIM(user_scenario),'') IS NOT NULL),'{}'::text[]) scenarios,
				COALESCE(ARRAY_AGG(DISTINCT BTRIM(note_type)) FILTER (WHERE NULLIF(BTRIM(note_type),'') IS NOT NULL),'{}'::text[]) note_types
			FROM service_provider_note_executions WHERE deleted_at IS NULL GROUP BY note_id
		)
		SELECT notes.note_id,COALESCE(NULLIF(BTRIM(notes.source_title),''),LEFT(notes.note_content,80)),notes.note_content,
			COALESCE(tags.audience,'{}'),COALESCE(tags.scenarios,'{}'),COALESCE(tags.note_types,'{}'),
			COALESCE(performance.spend,0),COALESCE(performance.users,0),performance.cost
		FROM service_provider_notes notes
		LEFT JOIN performance USING(note_id) LEFT JOIN tags USING(note_id)
		WHERE notes.note_id=ANY($1::text[])
		ORDER BY notes.note_id
	`, noteIDs)
	if err != nil {
		return nil, fmt.Errorf("query recommendation candidates: %w", err)
	}
	defer rows.Close()
	result := make([]delivery.CandidateNote, 0, len(noteIDs))
	for rows.Next() {
		var note delivery.CandidateNote
		if err := rows.Scan(&note.NoteID, &note.Title, &note.Content, &note.Audience, &note.Scenarios,
			&note.NoteTypes, &note.HistoricalSpend, &note.HistoricalUsers, &note.HistoricalCost); err != nil {
			return nil, fmt.Errorf("scan recommendation candidate: %w", err)
		}
		result = append(result, note)
	}
	return result, rows.Err()
}

func (p *Postgres) Audit(ctx context.Context, actor delivery.Actor, action, resourceType, resourceID string, advertiserID int64, detail map[string]any) error {
	encoded, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO delivery_audit_log (actor,role,action,resource_type,resource_id,advertiser_id,detail)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,0),$7::jsonb)
	`, actor.ID, actor.Role, action, resourceType, resourceID, advertiserID, string(encoded))
	if err != nil {
		return fmt.Errorf("save delivery audit log: %w", err)
	}
	return nil
}

const deliveryDraftSelect = `
	SELECT id,advertiser_id,status,current_version,spec,spec_hash,idempotency_key,
		created_by,updated_by,created_at,updated_at
	FROM delivery_drafts`

type rowScanner interface {
	Scan(...any) error
}

func scanDeliveryRecommendation(row rowScanner) (delivery.Recommendation, error) {
	var value delivery.Recommendation
	var payload []byte
	err := row.Scan(&value.ID, &value.DraftID, &value.DraftVersion, &value.SchemaVersion,
		&value.LLMProvider, &value.LLMModel, &value.RankerFamily, &value.RankerVersion,
		&value.RulesVersion, &payload, &value.Warnings, &value.CreatedBy, &value.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return delivery.Recommendation{}, delivery.ErrNotFound
	}
	if err != nil {
		return delivery.Recommendation{}, fmt.Errorf("scan delivery recommendation: %w", err)
	}
	if err := json.Unmarshal(payload, &value.Payload); err != nil {
		return delivery.Recommendation{}, fmt.Errorf("decode delivery recommendation: %w", err)
	}
	return value, nil
}

func scanDeliveryValidation(row rowScanner) (delivery.Validation, error) {
	var value delivery.Validation
	var errorsJSON, warningsJSON, capabilityJSON []byte
	err := row.Scan(&value.ID, &value.DraftID, &value.DraftVersion, &value.SpecHash,
		&value.RulesVersion, &value.ContractVersion, &value.Valid, &errorsJSON, &warningsJSON,
		&capabilityJSON, &value.ValidUntil, &value.CreatedBy, &value.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return delivery.Validation{}, delivery.ErrNotFound
	}
	if err != nil {
		return delivery.Validation{}, fmt.Errorf("scan delivery validation: %w", err)
	}
	if err := json.Unmarshal(errorsJSON, &value.Errors); err != nil {
		return delivery.Validation{}, fmt.Errorf("decode delivery validation errors: %w", err)
	}
	if err := json.Unmarshal(warningsJSON, &value.Warnings); err != nil {
		return delivery.Validation{}, fmt.Errorf("decode delivery validation warnings: %w", err)
	}
	if err := json.Unmarshal(capabilityJSON, &value.CapabilitySnapshot); err != nil {
		return delivery.Validation{}, fmt.Errorf("decode delivery validation capability: %w", err)
	}
	return value, nil
}

func scanMediaEntity(row rowScanner) (delivery.MediaEntity, error) {
	var value delivery.MediaEntity
	var payload []byte
	err := row.Scan(&value.ID, &value.JobID, &value.DraftID, &value.AdvertiserID,
		&value.EntityType, &value.LocalKey, &value.ParentLocalKey, &value.MediaID,
		&value.ParentMediaID, &value.DesiredStatus, &value.ObservedStatus, &payload,
		&value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return delivery.MediaEntity{}, delivery.ErrNotFound
	}
	if err != nil {
		return delivery.MediaEntity{}, fmt.Errorf("scan delivery media entity: %w", err)
	}
	if err := json.Unmarshal(payload, &value.UpstreamPayload); err != nil {
		return delivery.MediaEntity{}, fmt.Errorf("decode delivery media entity: %w", err)
	}
	return value, nil
}

func scanDeliveryDraft(row rowScanner) (delivery.Draft, error) {
	var value delivery.Draft
	var specJSON []byte
	err := row.Scan(&value.ID, &value.AdvertiserID, &value.Status, &value.CurrentVersion,
		&specJSON, &value.SpecHash, &value.IdempotencyKey, &value.CreatedBy, &value.UpdatedBy,
		&value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return delivery.Draft{}, delivery.ErrNotFound
	}
	if err != nil {
		return delivery.Draft{}, fmt.Errorf("scan delivery draft: %w", err)
	}
	if err := json.Unmarshal(specJSON, &value.Spec); err != nil {
		return delivery.Draft{}, fmt.Errorf("decode delivery draft: %w", err)
	}
	return value, nil
}

func draftTx(ctx context.Context, tx pgx.Tx, draftID string, forUpdate bool) (delivery.Draft, error) {
	query := deliveryDraftSelect + " WHERE id=$1"
	if forUpdate {
		query += " FOR UPDATE"
	}
	return scanDeliveryDraft(tx.QueryRow(ctx, query, draftID))
}

func draftByIdempotencyTx(ctx context.Context, tx pgx.Tx, advertiserID int64, key string) (delivery.Draft, error) {
	return scanDeliveryDraft(tx.QueryRow(ctx, deliveryDraftSelect+" WHERE advertiser_id=$1 AND idempotency_key=$2", advertiserID, key))
}

const deliveryPublishJobSelect = `
	SELECT id,draft_id,draft_version,advertiser_id,mode,status,current_step,
		idempotency_key,request_preview,result,error_code,error_message,retry_count,
		requested_by,requested_role,created_at,started_at,completed_at,updated_at
	FROM delivery_publish_jobs`

func scanPublishJob(row rowScanner) (delivery.PublishJob, error) {
	var value delivery.PublishJob
	var previewJSON, resultJSON []byte
	err := row.Scan(&value.ID, &value.DraftID, &value.DraftVersion, &value.AdvertiserID,
		&value.Mode, &value.Status, &value.CurrentStep, &value.IdempotencyKey,
		&previewJSON, &resultJSON, &value.ErrorCode, &value.ErrorMessage, &value.RetryCount,
		&value.RequestedBy, &value.RequestedRole, &value.CreatedAt, &value.StartedAt, &value.CompletedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return delivery.PublishJob{}, delivery.ErrNotFound
	}
	if err != nil {
		return delivery.PublishJob{}, fmt.Errorf("scan delivery publish job: %w", err)
	}
	if err := json.Unmarshal(previewJSON, &value.RequestPreview); err != nil {
		return delivery.PublishJob{}, fmt.Errorf("decode delivery publish preview: %w", err)
	}
	if err := json.Unmarshal(resultJSON, &value.Result); err != nil {
		return delivery.PublishJob{}, fmt.Errorf("decode delivery publish result: %w", err)
	}
	return value, nil
}
