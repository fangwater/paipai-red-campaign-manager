package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/guorai"
	"paipai-red-campaign-manager/internal/model"
	"paipai-red-campaign-manager/internal/store"
)

func runGuoraiSync(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	session := flags.String("session", defaultSessionPath(), "path to the persistent cookie session")
	databaseURL := flags.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string; defaults to DATABASE_URL")
	businessType := flags.String("type", "all", "business type: note, plan, or all")
	snapshotDays := flags.Int("days", 1, "number of latest rolling snapshot dates to refresh")
	noteWindowDays := flags.Int("note-window-days", 14, "inclusive days in each note rolling window")
	planWindowDays := flags.Int("plan-window-days", 7, "inclusive days in each plan rolling window")
	windowDaysOverride := flags.Int("window-days", 0, "override both note and plan rolling windows")
	asOfRaw := flags.String("as-of", "", "latest snapshot date (YYYY-MM-DD); defaults to the platform cutoff")
	brandID := flags.String("brand-id", "", "XHS brand ID; defaults to the bound brand")
	merchantID := flags.String("merchant-id", os.Getenv("GUORAI_MERCHANT_ID"), "merchant ID; defaults to GUORAI_MERCHANT_ID")
	pageSize := flags.Int("page-size", 500, "records requested per API page (max 500)")
	timeout := flags.Duration("timeout", 30*time.Minute, "maximum time for the complete sync")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*databaseURL) == "" {
		return errors.New("--database-url or DATABASE_URL is required")
	}
	if *snapshotDays <= 0 || *snapshotDays > 365 {
		return errors.New("--days must be between 1 and 365")
	}
	if *noteWindowDays <= 0 || *noteWindowDays > 90 {
		return errors.New("--note-window-days must be between 1 and 90")
	}
	if *planWindowDays <= 0 || *planWindowDays > 90 {
		return errors.New("--plan-window-days must be between 1 and 90")
	}
	if *windowDaysOverride < 0 || *windowDaysOverride > 90 {
		return errors.New("--window-days must be 0 or between 1 and 90")
	}
	if *pageSize <= 0 || *pageSize > 500 {
		return errors.New("--page-size must be between 1 and 500")
	}
	if *timeout <= 0 {
		return errors.New("--timeout must be positive")
	}
	types, err := guoraiSyncTypes(*businessType)
	if err != nil {
		return err
	}

	syncCtx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	client, err := guorai.NewClient(*session)
	if err != nil {
		return err
	}
	destination, err := store.NewPostgres(syncCtx, *databaseURL, "guorai")
	if err != nil {
		return err
	}
	defer destination.Close()
	if err := destination.Migrate(syncCtx); err != nil {
		return err
	}
	releaseSyncLock, err := destination.AcquireGuoraiSyncLock(syncCtx)
	if err != nil {
		return err
	}
	defer releaseSyncLock()

	totalWindows := 0
	totalRows := 0
	for _, entityType := range types {
		entityWindowDays := guoraiWindowDays(entityType, *noteWindowDays, *planWindowDays, *windowDaysOverride)
		base, err := guoraiWithStoredLogin(syncCtx, client, destination, func() (guorai.ResolvedFilter, error) {
			return client.ResolveFilter(syncCtx, guorai.NotesFilter{
				BusinessType: entityType, BrandID: *brandID, MerchantID: *merchantID, PageSize: *pageSize,
			})
		})
		if err != nil {
			return fmt.Errorf("resolve %s sync context: %w", entityType, err)
		}
		cutoff, err := time.Parse(time.DateOnly, base.RuleEndDate)
		if err != nil {
			return fmt.Errorf("parse %s platform cutoff: %w", entityType, err)
		}
		asOf := cutoff
		if *asOfRaw != "" {
			asOf, err = time.Parse(time.DateOnly, *asOfRaw)
			if err != nil {
				return fmt.Errorf("parse --as-of: %w", err)
			}
			if asOf.After(cutoff) {
				return fmt.Errorf("--as-of %s is later than the %s platform cutoff %s", *asOfRaw, entityType, base.RuleEndDate)
			}
		}

		for _, window := range guoraiRollingWindows(asOf, *snapshotDays, entityWindowDays) {
			windowEnd := window.End
			windowStart := window.Start
			resolved := base
			resolved.BeginDate = windowStart.Format(time.DateOnly)
			resolved.EndDate = windowEnd.Format(time.DateOnly)
			resolved.PageSize = *pageSize
			resolved.Limit = 0

			queryResult, err := guoraiWithStoredLogin(syncCtx, client, destination, func() (guorai.NotesResult, error) {
				return client.QueryResolved(syncCtx, resolved)
			})
			if err != nil {
				return fmt.Errorf("query %s snapshot %s: %w", entityType, resolved.EndDate, err)
			}
			recordsJSON, err := json.Marshal(queryResult.Data)
			if err != nil {
				return fmt.Errorf("encode %s records: %w", entityType, err)
			}
			requestJSON, _ := json.Marshal(map[string]any{
				"businessType": entityType, "enterpriseId": resolved.EnterpriseID,
				"xhsBrandId": resolved.BrandID, "merchantId": resolved.MerchantID,
				"dateType": 1, "windowStart": resolved.BeginDate, "windowEnd": resolved.EndDate,
				"pageSize": resolved.PageSize, "sortColumn": resolved.SortField, "sortOrder": resolved.SortOrder,
			})
			responseJSON, err := json.Marshal(queryResult)
			if err != nil {
				return fmt.Errorf("encode %s raw response: %w", entityType, err)
			}
			stored, err := destination.SaveGuoraiSnapshot(syncCtx, model.GuoraiSnapshot{
				EntityType: entityType, EnterpriseID: resolved.EnterpriseID, XHSBrandID: resolved.BrandID,
				BrandName: resolved.BrandName, MerchantID: resolved.MerchantID,
				WindowStart: windowStart, WindowEnd: windowEnd, SnapshotDate: windowEnd, SourceCutoffDate: cutoff,
				AttributionType: resolved.Rule.EventType, AttributionModel: resolved.Rule.EventModel,
				AttributionWindowDays: guoraiAttributionDays(resolved.Rule.TradeDataPeriod),
				TrafficType:           guoraiTrafficType(entityType), RequestPayload: requestJSON,
				RawResponse: responseJSON, Records: recordsJSON,
			})
			if err != nil {
				return fmt.Errorf("store %s snapshot %s: %w", entityType, resolved.EndDate, err)
			}
			totalWindows++
			totalRows += stored.Rows
			fmt.Fprintf(os.Stderr, "stored %s snapshot %s (%s through %s): %d rows, fetch %d\n",
				entityType, resolved.EndDate, resolved.BeginDate, resolved.EndDate, stored.Rows, stored.FetchID)
		}
	}
	fmt.Printf("Guorai sync completed: %d windows, %d rows (note window %d days, plan window %d days)\n",
		totalWindows, totalRows,
		guoraiWindowDays(guorai.BusinessTypeNote, *noteWindowDays, *planWindowDays, *windowDaysOverride),
		guoraiWindowDays(guorai.BusinessTypePlan, *noteWindowDays, *planWindowDays, *windowDaysOverride),
	)
	return nil
}

type guoraiLoginClient interface {
	Login(context.Context, string, string) error
}

type guoraiCredentialsLoader interface {
	LoadGuoraiCredentials(context.Context) (store.GuoraiCredentials, error)
}

func guoraiWithStoredLogin[T any](
	ctx context.Context,
	client guoraiLoginClient,
	credentialsLoader guoraiCredentialsLoader,
	operation func() (T, error),
) (T, error) {
	result, err := operation()
	if !errors.Is(err, guorai.ErrSessionExpired) {
		return result, err
	}
	credentials, loadErr := credentialsLoader.LoadGuoraiCredentials(ctx)
	if loadErr != nil {
		var zero T
		return zero, fmt.Errorf("renew expired Guorai session: %w", loadErr)
	}
	if loginErr := client.Login(ctx, credentials.Username, credentials.Password); loginErr != nil {
		var zero T
		return zero, fmt.Errorf("renew expired Guorai session: %w", loginErr)
	}
	fmt.Fprintln(os.Stderr, "Guorai session renewed using stored PostgreSQL credentials")
	return operation()
}

func guoraiSyncTypes(value string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return []string{guorai.BusinessTypeNote, guorai.BusinessTypePlan}, nil
	case guorai.BusinessTypeNote:
		return []string{guorai.BusinessTypeNote}, nil
	case guorai.BusinessTypePlan:
		return []string{guorai.BusinessTypePlan}, nil
	default:
		return nil, errors.New("--type must be note, plan, or all")
	}
}

func guoraiWindowDays(entityType string, noteWindowDays, planWindowDays, override int) int {
	if override > 0 {
		return override
	}
	if entityType == guorai.BusinessTypeNote {
		return noteWindowDays
	}
	return planWindowDays
}

func guoraiAttributionDays(value any) int {
	raw := strings.TrimSpace(fmt.Sprint(value))
	days, _ := strconv.Atoi(raw)
	return days
}

func guoraiTrafficType(entityType string) string {
	if entityType == guorai.BusinessTypeNote {
		return "笔记整体流量效果"
	}
	return ""
}

type guoraiRollingWindow struct {
	Start time.Time
	End   time.Time
}

func guoraiRollingWindows(asOf time.Time, snapshotDays, windowDays int) []guoraiRollingWindow {
	windows := make([]guoraiRollingWindow, 0, snapshotDays)
	for offset := snapshotDays - 1; offset >= 0; offset-- {
		end := asOf.AddDate(0, 0, -offset)
		windows = append(windows, guoraiRollingWindow{Start: end.AddDate(0, 0, -(windowDays - 1)), End: end})
	}
	return windows
}
