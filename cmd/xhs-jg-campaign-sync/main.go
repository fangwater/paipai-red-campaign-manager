package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"paipai-red-campaign-manager/internal/store"
	"paipai-red-campaign-manager/internal/xhs"
)

const defaultAuthdURL = "http://127.0.0.1:18080"

type daemonClient struct {
	baseURL    string
	httpClient *http.Client
}

type daemonResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("xhs-jg-campaign-sync", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	authdURL := flags.String("url", envOrDefault("XHS_JG_AUTHD_URL", defaultAuthdURL), "XHS Spotlight auth daemon base URL")
	databaseURL := flags.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string; defaults to DATABASE_URL")
	advertiserID := flags.Int64("advertiser-id", 0, "sync only this authorized advertiser; defaults to all")
	includeUnits := flags.Bool("include-units", false, "also sync all non-deleted units")
	includeCreativities := flags.Bool("include-creativities", false, "also sync all non-deleted creativities")
	timeout := flags.Duration("timeout", 30*time.Minute, "maximum time for the complete full sync")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*databaseURL) == "" {
		return errors.New("--database-url or DATABASE_URL is required")
	}
	if *advertiserID < 0 {
		return errors.New("--advertiser-id cannot be negative")
	}
	if *timeout <= 0 {
		return errors.New("--timeout must be positive")
	}

	syncCtx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	source, err := newDaemonClient(*authdURL)
	if err != nil {
		return err
	}
	status, err := source.status(syncCtx)
	if err != nil {
		return err
	}
	if !status.Authorized || !status.AccessTokenValid {
		return errors.New("XHS Spotlight auth daemon does not have a valid access token")
	}
	advertisers, err := selectAdvertisers(status, *advertiserID)
	if err != nil {
		return err
	}

	destination, err := store.NewPostgres(syncCtx, *databaseURL, "xhs-jg-campaigns")
	if err != nil {
		return err
	}
	defer destination.Close()
	if err := destination.Migrate(syncCtx); err != nil {
		return err
	}

	totalCampaigns := 0
	totalUnits := 0
	totalCreativities := 0
	totalDeactivated := int64(0)
	syncErrors := make([]error, 0)
	for _, advertiser := range advertisers {
		campaigns, err := source.listAllActiveCampaigns(syncCtx, advertiser.ID)
		if err != nil {
			syncErrors = append(syncErrors, fmt.Errorf("fetch advertiser %d (%s): %w", advertiser.ID, advertiser.Name, err))
			continue
		}
		stored, err := destination.ReplaceXHSCampaignSnapshot(syncCtx, advertiser, campaigns)
		if err != nil {
			syncErrors = append(syncErrors, fmt.Errorf("store advertiser %d (%s): %w", advertiser.ID, advertiser.Name, err))
			continue
		}
		totalCampaigns += stored.Upserted
		totalDeactivated += stored.Deactivated
		fmt.Fprintf(os.Stderr, "stored advertiser %d (%s): %d active campaigns, %d deactivated\n",
			advertiser.ID, advertiser.Name, stored.Upserted, stored.Deactivated)

		if *includeUnits {
			units, err := source.listAllActiveUnits(syncCtx, advertiser.ID)
			if err != nil {
				syncErrors = append(syncErrors, fmt.Errorf("fetch units for advertiser %d (%s): %w", advertiser.ID, advertiser.Name, err))
			} else {
				unitStored, err := destination.ReplaceXHSUnitSnapshot(syncCtx, advertiser, units)
				if err != nil {
					syncErrors = append(syncErrors, fmt.Errorf("store units for advertiser %d (%s): %w", advertiser.ID, advertiser.Name, err))
				} else {
					totalUnits += unitStored.Upserted
					totalDeactivated += unitStored.Deactivated
					fmt.Fprintf(os.Stderr, "stored advertiser %d (%s): %d active units, %d deactivated\n",
						advertiser.ID, advertiser.Name, unitStored.Upserted, unitStored.Deactivated)
				}
			}
		}

		if *includeCreativities {
			creativities, err := source.listAllActiveCreativities(syncCtx, advertiser.ID)
			if err != nil {
				syncErrors = append(syncErrors, fmt.Errorf("fetch creativities for advertiser %d (%s): %w", advertiser.ID, advertiser.Name, err))
			} else {
				creativityStored, err := destination.ReplaceXHSCreativitySnapshot(syncCtx, advertiser, creativities)
				if err != nil {
					syncErrors = append(syncErrors, fmt.Errorf("store creativities for advertiser %d (%s): %w", advertiser.ID, advertiser.Name, err))
				} else {
					totalCreativities += creativityStored.Upserted
					totalDeactivated += creativityStored.Deactivated
					fmt.Fprintf(os.Stderr, "stored advertiser %d (%s): %d active creativities, %d deactivated\n",
						advertiser.ID, advertiser.Name, creativityStored.Upserted, creativityStored.Deactivated)
				}
			}
		}
	}
	if len(syncErrors) > 0 {
		return fmt.Errorf("full campaign sync completed with %d advertiser errors: %w", len(syncErrors), errors.Join(syncErrors...))
	}
	fmt.Printf("XHS Spotlight full sync completed: %d advertisers, %d active campaigns, %d active units, %d active creativities, %d deactivated\n",
		len(advertisers), totalCampaigns, totalUnits, totalCreativities, totalDeactivated)
	return nil
}

func newDaemonClient(baseURL string) (*daemonClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("XHS Spotlight auth daemon URL is required")
	}
	return &daemonClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 45 * time.Second},
	}, nil
}

func (client *daemonClient) status(ctx context.Context) (xhs.ManagerStatus, error) {
	var status xhs.ManagerStatus
	if err := client.do(ctx, http.MethodGet, "/v1/oauth/status", nil, &status); err != nil {
		return xhs.ManagerStatus{}, fmt.Errorf("query XHS Spotlight auth daemon status: %w", err)
	}
	return status, nil
}

func (client *daemonClient) listAllActiveCampaigns(ctx context.Context, advertiserID int64) ([]xhs.Campaign, error) {
	status := 6
	pageIndex := 1
	campaigns := make([]xhs.Campaign, 0)
	for {
		request := xhs.CampaignListRequest{
			AdvertiserID: advertiserID,
			Status:       &status,
			Page:         &xhs.CampaignPageRequest{PageIndex: pageIndex, PageSize: 100},
		}
		var page xhs.CampaignListData
		if err := client.doCampaignPage(ctx, request, &page); err != nil {
			return nil, fmt.Errorf("query campaign page %d: %w", pageIndex, err)
		}
		if page.Page.PageIndex != pageIndex {
			return nil, fmt.Errorf("campaign API returned page_index %d for requested page %d", page.Page.PageIndex, pageIndex)
		}
		campaigns = append(campaigns, page.Campaigns...)
		if len(campaigns) >= page.Page.TotalCount {
			return campaigns, nil
		}
		if len(page.Campaigns) == 0 {
			return nil, fmt.Errorf("campaign pagination stopped at page %d before total_count %d", pageIndex, page.Page.TotalCount)
		}
		pageIndex++
	}
}

func (client *daemonClient) doCampaignPage(ctx context.Context, request xhs.CampaignListRequest, page *xhs.CampaignListData) error {
	return client.doWithRetry(ctx, func() error {
		*page = xhs.CampaignListData{}
		return client.do(ctx, http.MethodPost, "/v1/campaigns/list", request, page)
	})
}

func (client *daemonClient) listAllActiveUnits(ctx context.Context, advertiserID int64) ([]xhs.Unit, error) {
	pageIndex := 1
	units := make([]xhs.Unit, 0)
	seen := make(map[int64]struct{})
	for {
		request := xhs.UnitListRequest{AdvertiserID: advertiserID, Page: pageIndex, PageSize: 100}
		var page xhs.UnitListData
		if err := client.doWithRetry(ctx, func() error {
			page = xhs.UnitListData{}
			return client.do(ctx, http.MethodPost, "/v1/units/list", request, &page)
		}); err != nil {
			return nil, fmt.Errorf("query unit page %d: %w", pageIndex, err)
		}
		added := 0
		for _, unit := range page.Units {
			if unit.UnitID <= 0 {
				return nil, fmt.Errorf("unit page %d returned a non-positive unit ID", pageIndex)
			}
			if _, exists := seen[unit.UnitID]; exists {
				continue
			}
			seen[unit.UnitID] = struct{}{}
			added++
			if unit.UnitFilterState != 1 {
				units = append(units, unit)
			}
		}
		if len(seen) >= page.TotalCount {
			return units, nil
		}
		if len(page.Units) == 0 || added == 0 {
			return nil, fmt.Errorf("unit pagination made no progress at page %d before total_count %d", pageIndex, page.TotalCount)
		}
		pageIndex++
	}
}

func (client *daemonClient) listAllActiveCreativities(ctx context.Context, advertiserID int64) ([]xhs.Creativity, error) {
	status := 2
	pageIndex := 1
	creativities := make([]xhs.Creativity, 0)
	seen := make(map[int64]struct{})
	for {
		request := xhs.CreativityListRequest{
			AdvertiserID: advertiserID,
			Status:       &status,
			Page:         &xhs.CampaignPageRequest{PageIndex: pageIndex, PageSize: 100},
		}
		var page xhs.CreativityListData
		if err := client.doWithRetry(ctx, func() error {
			page = xhs.CreativityListData{}
			return client.do(ctx, http.MethodPost, "/v1/creativities/list", request, &page)
		}); err != nil {
			return nil, fmt.Errorf("query creativity page %d: %w", pageIndex, err)
		}
		if page.Page.PageIndex != pageIndex {
			return nil, fmt.Errorf("creativity API returned page_index %d for requested page %d", page.Page.PageIndex, pageIndex)
		}
		added := 0
		for _, creativity := range page.Creativities {
			if creativity.CreativityID <= 0 {
				return nil, fmt.Errorf("creativity page %d returned a non-positive creativity ID", pageIndex)
			}
			if _, exists := seen[creativity.CreativityID]; exists {
				continue
			}
			seen[creativity.CreativityID] = struct{}{}
			creativities = append(creativities, creativity)
			added++
		}
		if len(creativities) >= page.Page.TotalCount {
			return creativities, nil
		}
		if len(page.Creativities) == 0 || added == 0 {
			return nil, fmt.Errorf("creativity pagination made no progress at page %d before total_count %d", pageIndex, page.Page.TotalCount)
		}
		pageIndex++
	}
}

func (client *daemonClient) doWithRetry(ctx context.Context, operation func() error) error {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		err = operation()
		if err == nil {
			return nil
		}
		if attempt == 3 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("failed after 3 attempts: %w", err)
}

func (client *daemonClient) do(ctx context.Context, method, path string, payload, result any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var envelope daemonResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(&envelope); err != nil {
		return fmt.Errorf("decode auth daemon response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !envelope.Success {
		message := strings.TrimSpace(envelope.Error)
		if message == "" {
			message = "request failed"
		}
		return fmt.Errorf("auth daemon HTTP %d: %s", response.StatusCode, message)
	}
	if err := json.Unmarshal(envelope.Data, result); err != nil {
		return fmt.Errorf("decode auth daemon data: %w", err)
	}
	return nil
}

func selectAdvertisers(status xhs.ManagerStatus, requestedID int64) ([]xhs.Advertiser, error) {
	advertisers := append([]xhs.Advertiser(nil), status.ApprovalAdvertisers...)
	if len(advertisers) == 0 && status.AdvertiserID > 0 {
		advertisers = append(advertisers, xhs.Advertiser{ID: status.AdvertiserID})
	}
	seen := make(map[int64]struct{}, len(advertisers))
	selected := make([]xhs.Advertiser, 0, len(advertisers))
	for _, advertiser := range advertisers {
		if advertiser.ID <= 0 || (requestedID > 0 && advertiser.ID != requestedID) {
			continue
		}
		if _, exists := seen[advertiser.ID]; exists {
			continue
		}
		seen[advertiser.ID] = struct{}{}
		selected = append(selected, advertiser)
	}
	if len(selected) == 0 {
		if requestedID > 0 {
			return nil, fmt.Errorf("advertiser %d is not present in the OAuth authorization", requestedID)
		}
		return nil, errors.New("OAuth authorization contains no advertisers")
	}
	sort.Slice(selected, func(left, right int) bool { return selected[left].ID < selected[right].ID })
	return selected, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
