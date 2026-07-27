package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func shouldApplyMaituoCurrentTables(ctx context.Context, tx pgx.Tx, reportDate time.Time) (bool, error) {
	var latest time.Time
	err := tx.QueryRow(ctx, `
		SELECT report_date
		FROM maituo_customer_daily_import_runs
		WHERE status='succeeded'
		ORDER BY report_date DESC,completed_at DESC
		LIMIT 1
	`).Scan(&latest)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("find latest Maituo report date: %w", err)
	}
	return !latest.After(reportDate), nil
}
