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
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/store"
	"paipai-red-campaign-manager/internal/xhssync"
)

type dailySyncJob struct {
	label string
	path  string
}

type daemonResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Error   string `json:"error"`
}

func runDailySync(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("sync-daily", flag.ContinueOnError)
	flags.SetOutput(output)
	serviceURL := flags.String("url", envOrDefault("XHS_JG_AUTHD_URL", defaultServiceURL), "auth daemon base URL")
	totalTimeout := flags.Duration("timeout", 2*time.Hour, "maximum duration of the complete daily sync")
	pollInterval := flags.Duration("poll-interval", 5*time.Second, "interval between sync status checks")
	requestTimeout := flags.Duration("request-timeout", 35*time.Second, "timeout for each auth daemon request")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *totalTimeout <= 0 || *pollInterval <= 0 || *requestTimeout <= 0 {
		return errors.New("all duration flags must be positive")
	}

	runContext, cancel := context.WithTimeout(ctx, *totalTimeout)
	defer cancel()
	client := &http.Client{Timeout: *requestTimeout}
	baseURL := strings.TrimRight(strings.TrimSpace(*serviceURL), "/")
	if baseURL == "" {
		return errors.New("--url or XHS_JG_AUTHD_URL is required")
	}

	jobs := []dailySyncJob{
		{label: "campaigns", path: "/v1/sync/campaigns"},
		{label: "units", path: "/v1/sync/units"},
		{label: "creativities", path: "/v1/sync/creativities"},
	}
	for _, job := range jobs {
		if err := waitForDailySyncIdle(runContext, client, baseURL, *pollInterval); err != nil {
			return fmt.Errorf("wait for XHS Spotlight sync service before %s: %w", job.label, err)
		}
		run, err := requestDaemonData[store.XHSJGSyncRun](
			runContext,
			client,
			http.MethodPost,
			baseURL+job.path,
			syncRequest{Mode: string(xhssync.ModeFull)},
		)
		if err != nil {
			return fmt.Errorf("start daily XHS Spotlight %s sync: %w", job.label, err)
		}
		fmt.Fprintf(output, "started XHS Spotlight %s full sync: run %d\n", job.label, run.RunID)

		completed, err := waitForDailySyncRun(runContext, client, baseURL, run.RunID, *pollInterval)
		if err != nil {
			return fmt.Errorf("wait for daily XHS Spotlight %s sync: %w", job.label, err)
		}
		fmt.Fprintf(
			output,
			"completed XHS Spotlight %s full sync: run %d, advertisers=%d, campaigns=%d, units=%d, creativities=%d, deactivated=%d\n",
			job.label,
			completed.RunID,
			completed.AdvertisersCount,
			completed.CampaignsCount,
			completed.UnitsCount,
			completed.CreativitiesCount,
			completed.DeactivatedCount,
		)
	}
	return nil
}

func waitForDailySyncIdle(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	pollInterval time.Duration,
) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		status, err := requestDaemonData[xhssync.Status](
			ctx,
			client,
			http.MethodGet,
			baseURL+"/v1/sync/status",
			nil,
		)
		if err != nil {
			return err
		}
		if !status.Running {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("sync service did not become idle: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForDailySyncRun(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	runID int64,
	pollInterval time.Duration,
) (store.XHSJGSyncRun, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		status, err := requestDaemonData[xhssync.Status](
			ctx,
			client,
			http.MethodGet,
			baseURL+"/v1/sync/status",
			nil,
		)
		if err != nil {
			return store.XHSJGSyncRun{}, err
		}
		for _, run := range status.Recent {
			if run.RunID != runID {
				continue
			}
			switch run.Status {
			case "succeeded":
				if !status.Running {
					return run, nil
				}
			case "failed":
				message := strings.TrimSpace(run.ErrorMessage)
				if message == "" {
					message = "sync run failed without an error message"
				}
				return run, errors.New(message)
			}
			break
		}

		select {
		case <-ctx.Done():
			return store.XHSJGSyncRun{}, fmt.Errorf("sync run %d did not finish: %w", runID, ctx.Err())
		case <-ticker.C:
		}
	}
}

func requestDaemonData[T any](
	ctx context.Context,
	client *http.Client,
	method string,
	endpoint string,
	payload any,
) (T, error) {
	var zero T
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return zero, err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return zero, err
	}
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return zero, fmt.Errorf("request auth daemon: %w", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return zero, err
	}
	var envelope daemonResponse[T]
	if err := json.Unmarshal(data, &envelope); err != nil {
		return zero, fmt.Errorf("auth daemon returned HTTP %d with invalid JSON", response.StatusCode)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !envelope.Success {
		message := strings.TrimSpace(envelope.Error)
		if message == "" {
			message = strings.TrimSpace(string(data))
		}
		return zero, fmt.Errorf("auth daemon returned HTTP %d: %s", response.StatusCode, message)
	}
	return envelope.Data, nil
}
