package store

import (
	"context"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/migrations"
)

func TestMaituoNoteSubaccountMigrationRestoresArchivedRowsIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "maituo-subaccount-migration-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()

	if _, err := postgres.pool.Exec(ctx, migrations.InitSQL+"\n"+migrations.MaituoCustomerDailySQL); err != nil {
		t.Fatalf("create pre-migration schema: %v", err)
	}
	if _, err := postgres.pool.Exec(ctx, `
		ALTER TABLE maituo_customer_daily_notes
			ADD COLUMN subaccount TEXT NOT NULL DEFAULT '',
			ADD COLUMN campaign_name TEXT NOT NULL DEFAULT '';
		ALTER TABLE maituo_customer_daily_notes
			DROP CONSTRAINT maituo_customer_daily_notes_pkey;
		ALTER TABLE maituo_customer_daily_notes
			ADD PRIMARY KEY (report_date, note_id, subaccount, campaign_name, placement);
	`); err != nil {
		t.Fatalf("restore pre-collapse note key: %v", err)
	}

	var runID int64
	if err := postgres.pool.QueryRow(ctx, `
		INSERT INTO maituo_customer_daily_import_runs (file_name,file_sha256,status)
		VALUES ('2099-02-01-Maituo.xlsx','subaccount-migration','succeeded')
		RETURNING id
	`).Scan(&runID); err != nil {
		t.Fatalf("create import run: %v", err)
	}
	if _, err := postgres.pool.Exec(ctx, `
		INSERT INTO maituo_customer_daily_notes (
			report_date,note_id,note_url,category,subaccount,campaign_name,placement,
			spend,search_users,search_cost,cpc,ctr_pct,source_row_number,content_hash,import_run_id
		) VALUES
			('2099-02-01','shared-note','https://example.com/note','测评','账户A','计划A','搜索',10,1,10,1,1,2,'account-a',$1),
			('2099-02-01','shared-note','https://example.com/note','测评','账户B','计划B','搜索',20,2,10,1,1,3,'account-b',$1)
	`, runID); err != nil {
		t.Fatalf("seed account-level rows: %v", err)
	}

	if err := postgres.Migrate(ctx); err != nil {
		t.Fatalf("migrate account-level rows: %v", err)
	}
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}

	rows, err := postgres.pool.Query(ctx, `
		SELECT subaccount,campaign_name,spend::DOUBLE PRECISION,estimated_postback_cost::DOUBLE PRECISION
		FROM maituo_customer_daily_notes
		WHERE note_id='shared-note' AND deleted_at IS NULL
		ORDER BY subaccount
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var restored []struct {
		subaccount string
		campaign   string
		spend      float64
		cost       float64
	}
	for rows.Next() {
		var item struct {
			subaccount string
			campaign   string
			spend      float64
			cost       float64
		}
		if err := rows.Scan(&item.subaccount, &item.campaign, &item.spend, &item.cost); err != nil {
			t.Fatal(err)
		}
		restored = append(restored, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(restored) != 2 || restored[0].subaccount != "账户A" || restored[0].campaign != "计划A" || restored[0].spend != 10 || restored[0].cost != 6.3 || restored[1].subaccount != "账户B" || restored[1].campaign != "计划B" || restored[1].spend != 20 || restored[1].cost != 6.3 {
		t.Fatalf("restored rows = %+v", restored)
	}
}
