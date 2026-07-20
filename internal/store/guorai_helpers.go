package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

func queueGuoraiNoteDimension(batch *pgx.Batch, record map[string]any, raw []byte) {
	batch.Queue(`INSERT INTO guorai_notes (
		note_id,note_name,note_type,note_author_name,account_name,note_publish_time,
		note_pic,spu_id,spu_name,tag,first_seen_at,last_seen_at,raw_dimension_payload
	) VALUES ($1,$2,$3,$4,$5,NULLIF($6,'')::timestamp,$7,$8,$9,$10,NOW(),NOW(),$11::jsonb)
	ON CONFLICT (note_id) DO UPDATE SET
		note_name=COALESCE(NULLIF(EXCLUDED.note_name,''),guorai_notes.note_name),
		note_type=COALESCE(EXCLUDED.note_type,guorai_notes.note_type),
		note_author_name=COALESCE(NULLIF(EXCLUDED.note_author_name,''),guorai_notes.note_author_name),
		account_name=COALESCE(NULLIF(EXCLUDED.account_name,''),guorai_notes.account_name),
		note_publish_time=COALESCE(EXCLUDED.note_publish_time,guorai_notes.note_publish_time),
		note_pic=COALESCE(NULLIF(EXCLUDED.note_pic,''),guorai_notes.note_pic),
		spu_id=COALESCE(NULLIF(EXCLUDED.spu_id,''),guorai_notes.spu_id),
		spu_name=COALESCE(NULLIF(EXCLUDED.spu_name,''),guorai_notes.spu_name),
		tag=COALESCE(NULLIF(EXCLUDED.tag,''),guorai_notes.tag),
		last_seen_at=NOW(),raw_dimension_payload=EXCLUDED.raw_dimension_payload`,
		guoraiString(record["noteId"]), guoraiString(record["noteName"]), guoraiInt(record["noteType"]),
		guoraiString(record["noteAuthorName"]), guoraiString(record["accountName"]),
		guoraiString(record["notePublishTime"]), guoraiString(record["notePic"]),
		guoraiString(record["spuId"]), guoraiString(record["spuName"]), guoraiString(record["tag"]), string(raw))
}

func queueGuoraiPlanDimension(batch *pgx.Batch, record map[string]any, raw []byte) {
	batch.Queue(`INSERT INTO guorai_plans (
		plan_id,plan_name,plan_type,plan_publish_time,account_name,tag,
		first_seen_at,last_seen_at,raw_dimension_payload
	) VALUES ($1,$2,$3,NULLIF($4,'')::timestamp,$5,$6,NOW(),NOW(),$7::jsonb)
	ON CONFLICT (plan_id) DO UPDATE SET
		plan_name=COALESCE(NULLIF(EXCLUDED.plan_name,''),guorai_plans.plan_name),
		plan_type=COALESCE(NULLIF(EXCLUDED.plan_type,''),guorai_plans.plan_type),
		plan_publish_time=COALESCE(EXCLUDED.plan_publish_time,guorai_plans.plan_publish_time),
		account_name=COALESCE(NULLIF(EXCLUDED.account_name,''),guorai_plans.account_name),
		tag=COALESCE(NULLIF(EXCLUDED.tag,''),guorai_plans.tag),
		last_seen_at=NOW(),raw_dimension_payload=EXCLUDED.raw_dimension_payload`,
		guoraiString(record["planId"]), guoraiString(record["planName"]), guoraiString(record["planType"]),
		guoraiString(record["planPublishTime"]), guoraiString(record["accountName"]),
		guoraiString(record["tag"]), string(raw))
}

func executeGuoraiBatch(ctx context.Context, tx pgx.Tx, batch *pgx.Batch) error {
	if batch.Len() == 0 {
		return nil
	}
	results := tx.SendBatch(ctx, batch)
	for index := 0; index < batch.Len(); index++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("execute Guorai batch item %d: %w", index+1, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close Guorai batch: %w", err)
	}
	return nil
}

func expandGuoraiMetrics(ctx context.Context, tx pgx.Tx, table string, fields []guoraiMetricField, fetchID int64) error {
	assignments := make([]string, 0, len(fields))
	for _, field := range fields {
		jsonType := "number"
		cast := field.cast
		if field.cast == "boolean" {
			jsonType = "boolean"
		}
		if field.cast == "bigint" {
			cast = "numeric::bigint"
		}
		assignments = append(assignments, fmt.Sprintf(
			"%s=CASE WHEN jsonb_typeof(raw_payload->'%s')='%s' THEN (raw_payload->>'%s')::%s END",
			field.column, field.json, jsonType, field.json, cast))
	}
	query := fmt.Sprintf("UPDATE %s SET %s WHERE fetch_id=$1", table, strings.Join(assignments, ","))
	if _, err := tx.Exec(ctx, query, fetchID); err != nil {
		return fmt.Errorf("expand Guorai raw metrics: %w", err)
	}
	return nil
}

func guoraiString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func guoraiInt(value any) any {
	raw := guoraiString(value)
	if raw == "" {
		return nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return parsed
}
