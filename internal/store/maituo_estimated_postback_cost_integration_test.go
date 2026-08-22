package store

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

func TestMaituoEstimatedPostbackCostMigrationIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "maituo-estimated-postback-cost-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	prefix := "estimated-postback-integration-" + time.Now().Format("20060102150405.000000000")
	fileName := prefix + ".xlsx"
	reportDate := time.Date(2097, 3, 1, 0, 0, 0, 0, time.UTC)
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, `UPDATE maituo_customer_daily_notes SET estimated_postback_cost=ROUND(ROUND(search_cost,2)*0.63,2) WHERE note_id=$1`, prefix)
		_, _ = postgres.pool.Exec(cleanup, `UPDATE maituo_customer_daily_subaccounts SET estimated_postback_cost=ROUND(ROUND(search_cost,2)*0.63,2) WHERE spu=$1`, prefix)
		_ = postgres.Migrate(cleanup)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_notes WHERE note_id=$1", prefix)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_subaccounts WHERE spu=$1", prefix)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM maituo_customer_daily_import_runs WHERE file_name=$1", fileName)
	}()

	noteSearchCost := 1.005
	subaccountSearchCost := 10.075
	wrongSourceCost := 999.0
	snapshot := maituo.Snapshot{
		FileName: fileName, FileSHA256: prefix, ReportDate: reportDate,
		PresentSheets: []string{maituo.SheetNotes, maituo.SheetSubaccount},
		Notes: []maituo.NoteDetail{{
			NoteID: prefix, NoteURL: "https://example.com/note", Category: "测评", Placement: "搜索",
			Spend: 1, SearchUsers: 1, SearchCost: &noteSearchCost, EstimatedPostbackCost: &wrongSourceCost,
			CPC: 1, CTRPct: 1, RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: prefix + "-note"},
		}},
		Subaccounts: []maituo.SubaccountOverview{{
			SPU: prefix, Subaccount: prefix, Placement: "信息流",
			SearchCost: &subaccountSearchCost, EstimatedPostbackCost: &wrongSourceCost,
			Spend: 1, SearchUsers: 1, NoteCount: 1,
			RowMetadata: maituo.RowMetadata{SourceRow: 2, ContentHash: prefix + "-subaccount"},
		}},
	}
	if _, err := postgres.ImportMaituoCustomerDaily(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	assertStoredPostbackCosts(t, ctx, postgres, prefix, 0.64, 6.35)

	if _, err := postgres.pool.Exec(ctx, `
		ALTER TABLE maituo_customer_daily_notes
			DROP CONSTRAINT maituo_notes_estimated_postback_cost_formula;
		ALTER TABLE maituo_customer_daily_subaccounts
			DROP CONSTRAINT maituo_subaccounts_estimated_postback_cost_formula
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.pool.Exec(ctx, `UPDATE maituo_customer_daily_notes SET estimated_postback_cost=0.01 WHERE note_id=$1`, prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.pool.Exec(ctx, `UPDATE maituo_customer_daily_subaccounts SET estimated_postback_cost=NULL WHERE spu=$1`, prefix); err != nil {
		t.Fatal(err)
	}

	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	assertStoredPostbackCosts(t, ctx, postgres, prefix, 0.64, 6.35)
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}

	var constraints int
	if err := postgres.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM pg_constraint
		WHERE conname IN (
			'maituo_notes_estimated_postback_cost_formula',
			'maituo_subaccounts_estimated_postback_cost_formula'
		)
		  AND convalidated
	`).Scan(&constraints); err != nil {
		t.Fatal(err)
	}
	if constraints != 2 {
		t.Fatalf("validated formula constraints = %d, want 2", constraints)
	}
	if _, err := postgres.pool.Exec(ctx, `
		UPDATE maituo_customer_daily_notes
		SET estimated_postback_cost=0.63
		WHERE note_id=$1
	`, prefix); err == nil {
		t.Fatal("formula constraint accepted an invalid estimated postback cost")
	}
}

func assertStoredPostbackCosts(t *testing.T, ctx context.Context, postgres *Postgres, prefix string, wantNote, wantSubaccount float64) {
	t.Helper()
	var noteCost, subaccountCost float64
	if err := postgres.pool.QueryRow(ctx, `
		SELECT notes.estimated_postback_cost::DOUBLE PRECISION,
		       subaccounts.estimated_postback_cost::DOUBLE PRECISION
		FROM maituo_customer_daily_notes notes
		CROSS JOIN maituo_customer_daily_subaccounts subaccounts
		WHERE notes.note_id=$1 AND subaccounts.spu=$1
	`, prefix).Scan(&noteCost, &subaccountCost); err != nil {
		t.Fatal(err)
	}
	if math.Abs(noteCost-wantNote) > 1e-9 || math.Abs(subaccountCost-wantSubaccount) > 1e-9 {
		t.Fatalf("stored costs = (%v, %v), want (%v, %v)", noteCost, subaccountCost, wantNote, wantSubaccount)
	}
}
