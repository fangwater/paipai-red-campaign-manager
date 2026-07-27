package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type LarkSyncRun struct {
	RunID            int64      `json:"run_id"`
	Status           string     `json:"status"`
	FetchedCount     int        `json:"fetched_count"`
	UpsertedCount    int        `json:"upserted_count"`
	DeletedCount     int64      `json:"deleted_count"`
	TablesCount      int        `json:"tables_count"`
	DocumentsFetched int        `json:"documents_fetched"`
	DocumentErrors   int        `json:"document_errors"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	StartedAt        time.Time  `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

func (p *Postgres) LarkSyncRuns(ctx context.Context, limit int) ([]LarkSyncRun, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("Lark sync run limit must be between 1 and 100")
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id, status, fetched_count, upserted_count, deleted_count, tables_count,
			documents_fetched, document_errors, COALESCE(error_message, ''), started_at, completed_at
		FROM sync_runs
		WHERE app_token = $1
		ORDER BY started_at DESC
		LIMIT $2
	`, p.appToken, limit)
	if err != nil {
		return nil, fmt.Errorf("list Lark sync runs: %w", err)
	}
	defer rows.Close()
	runs := make([]LarkSyncRun, 0, limit)
	for rows.Next() {
		var run LarkSyncRun
		if err := rows.Scan(
			&run.RunID, &run.Status, &run.FetchedCount, &run.UpsertedCount, &run.DeletedCount,
			&run.TablesCount, &run.DocumentsFetched, &run.DocumentErrors, &run.ErrorMessage,
			&run.StartedAt, &run.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan Lark sync run: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Lark sync runs: %w", err)
	}
	return runs, nil
}
