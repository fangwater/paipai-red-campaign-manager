package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"paipai-red-campaign-manager/internal/store"
	"paipai-red-campaign-manager/internal/xhs"
	"paipai-red-campaign-manager/internal/xhssync"

	"golang.org/x/term"
)

const (
	defaultListenAddress = "127.0.0.1:18080"
	defaultServiceURL    = "http://127.0.0.1:18080"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return errors.New("a command is required")
	}
	switch args[0] {
	case "serve":
		return runServe(ctx, args[1:])
	case "authorize":
		return runAuthorize(ctx, args[1:])
	case "status":
		return runControlRequest(ctx, http.MethodGet, "/v1/oauth/status", args[1:])
	case "refresh":
		return runControlRequest(ctx, http.MethodPost, "/v1/oauth/refresh", args[1:])
	case "sync-status":
		return runControlRequest(ctx, http.MethodGet, "/v1/sync/status", args[1:])
	case "sync-campaigns":
		return runSyncControlRequest(ctx, "/v1/sync/campaigns", xhssync.ModeIncremental, args[1:])
	case "sync-units":
		return runSyncControlRequest(ctx, "/v1/sync/units", xhssync.ModeIncremental, args[1:])
	case "sync-creativities":
		return runSyncControlRequest(ctx, "/v1/sync/creativities", xhssync.ModeFull, args[1:])
	case "sync-daily":
		return runDailySync(ctx, args[1:], os.Stdout)
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runServe(ctx context.Context, args []string) error {
	refreshBefore, err := durationFromEnvironment("XHS_JG_REFRESH_BEFORE", 10*time.Minute)
	if err != nil {
		return err
	}
	retryInterval, err := durationFromEnvironment("XHS_JG_REFRESH_RETRY", time.Minute)
	if err != nil {
		return err
	}
	syncTimeout, err := durationFromEnvironment("XHS_JG_SYNC_TIMEOUT", 30*time.Minute)
	if err != nil {
		return err
	}

	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	listenAddress := flags.String("listen", envOrDefault("XHS_JG_AUTHD_LISTEN", defaultListenAddress), "loopback HTTP listen address")
	databaseURL := flags.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	syncTimezone := flags.String("sync-timezone", envOrDefault("XHS_JG_SYNC_TIMEZONE", "Asia/Shanghai"), "incremental sync date-window time zone")
	sessionPath := flags.String("session", defaultSessionPath(), "path to the persistent OAuth session")
	flags.DurationVar(&refreshBefore, "refresh-before", refreshBefore, "refresh access token this long before expiry")
	flags.DurationVar(&retryInterval, "retry-interval", retryInterval, "retry interval after refresh failure")
	flags.DurationVar(&syncTimeout, "sync-timeout", syncTimeout, "maximum duration of one manual sync")
	requestTimeout := flags.Duration("request-timeout", 30*time.Second, "upstream request timeout")
	shutdownTimeout := flags.Duration("shutdown-timeout", 10*time.Second, "HTTP shutdown timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if refreshBefore <= 0 || retryInterval <= 0 || *requestTimeout <= 0 || *shutdownTimeout <= 0 {
		return errors.New("all duration flags must be positive")
	}
	if err := requireLoopbackAddress(*listenAddress); err != nil {
		return err
	}
	if strings.TrimSpace(*databaseURL) == "" {
		return errors.New("--database-url or DATABASE_URL is required")
	}
	location, err := time.LoadLocation(*syncTimezone)
	if err != nil {
		return fmt.Errorf("load XHS_JG_SYNC_TIMEZONE: %w", err)
	}

	client, err := newClientFromEnvironment()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	manager, err := xhs.NewTokenManager(
		client,
		*sessionPath,
		xhs.WithRefreshBefore(refreshBefore),
		xhs.WithRefreshRetry(retryInterval),
		xhs.WithRefreshErrorHandler(func(refreshErr error) {
			logger.Error("XHS Spotlight token refresh failed", "error", refreshErr)
		}),
	)
	if err != nil {
		return err
	}

	destination, err := store.NewPostgres(ctx, *databaseURL, "xhs-jg-manual-sync")
	if err != nil {
		return err
	}
	defer destination.Close()
	if err := destination.Migrate(ctx); err != nil {
		return err
	}
	if err := destination.FailRunningXHSJGSyncRuns(ctx, "sync run exceeded its timeout before auth daemon restart", time.Now().Add(-syncTimeout)); err != nil {
		return err
	}
	syncService, err := xhssync.New(manager, destination, logger, syncTimeout, location)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", *listenAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", *listenAddress, err)
	}
	server := &http.Server{
		Handler:           newAuthHandler(manager, *requestTimeout, withSyncService(ctx, syncService)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       *requestTimeout + 5*time.Second,
		WriteTimeout:      *requestTimeout + 5*time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go manager.Run(ctx)
	serverErrors := make(chan error, 1)
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErrors <- serveErr
		}
	}()

	status := manager.Status()
	logger.Info("XHS Spotlight auth daemon started",
		"listen", listener.Addr().String(),
		"authorized", status.Authorized,
		"session", *sessionPath,
		"refresh_before", refreshBefore,
		"retry_interval", retryInterval,
		"sync_timezone", location.String(),
		"sync_timeout", syncTimeout,
	)
	select {
	case <-ctx.Done():
		logger.Info("XHS Spotlight auth daemon shutdown requested")
	case serveErr := <-serverErrors:
		return fmt.Errorf("serve XHS Spotlight auth daemon: %w", serveErr)
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), *shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown XHS Spotlight auth daemon: %w", err)
	}
	if err := syncService.Wait(shutdownContext); err != nil {
		logger.Warn("timed out waiting for XHS Spotlight sync shutdown", "error", err)
	}
	return nil
}

func runAuthorize(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("authorize", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	serviceURL := flags.String("url", envOrDefault("XHS_JG_AUTHD_URL", defaultServiceURL), "auth daemon base URL")
	authCodeFlag := flags.String("auth-code", "", "temporary authorization code; prompted when omitted")
	timeout := flags.Duration("timeout", 35*time.Second, "HTTP request timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	authCode, err := readAuthCode(*authCodeFlag)
	if err != nil {
		return err
	}
	return callDaemon(ctx, http.MethodPost, *serviceURL+"/v1/oauth/authorize", authorizeRequest{AuthCode: authCode}, *timeout)
}

func runControlRequest(ctx context.Context, method, path string, args []string) error {
	flags := flag.NewFlagSet(strings.TrimPrefix(path, "/v1/oauth/"), flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	serviceURL := flags.String("url", envOrDefault("XHS_JG_AUTHD_URL", defaultServiceURL), "auth daemon base URL")
	timeout := flags.Duration("timeout", 35*time.Second, "HTTP request timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return callDaemon(ctx, method, *serviceURL+path, nil, *timeout)
}

func runSyncControlRequest(ctx context.Context, path string, defaultMode xhssync.Mode, args []string) error {
	flags := flag.NewFlagSet(strings.TrimPrefix(path, "/v1/sync/"), flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	serviceURL := flags.String("url", envOrDefault("XHS_JG_AUTHD_URL", defaultServiceURL), "auth daemon base URL")
	advertiserID := flags.Int64("advertiser-id", 0, "limit the refresh to one authorized advertiser; zero refreshes all")
	mode := flags.String("mode", string(defaultMode), "sync mode: incremental or full")
	timeout := flags.Duration("timeout", 35*time.Second, "HTTP request timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *advertiserID < 0 {
		return errors.New("--advertiser-id cannot be negative")
	}
	selectedMode := xhssync.Mode(strings.TrimSpace(*mode))
	if selectedMode != xhssync.ModeIncremental && selectedMode != xhssync.ModeFull {
		return errors.New("--mode must be incremental or full")
	}
	if path == "/v1/sync/creativities" && selectedMode != xhssync.ModeFull {
		return errors.New("creativity sync only supports --mode full")
	}
	payload := syncRequest{AdvertiserID: *advertiserID, Mode: string(selectedMode)}
	return callDaemon(ctx, http.MethodPost, *serviceURL+path, payload, *timeout)
}

func callDaemon(ctx context.Context, method, endpoint string, payload interface{}, timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("--timeout must be positive")
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	requestContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, method, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := (&http.Client{Timeout: timeout}).Do(request)
	if err != nil {
		return fmt.Errorf("request auth daemon: %w", err)
	}
	defer response.Body.Close()
	var result json.RawMessage
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if !json.Valid(data) {
		return fmt.Errorf("auth daemon returned HTTP %d with invalid JSON", response.StatusCode)
	}
	result = data
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("auth daemon returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(result)))
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, result, "", "  "); err != nil {
		return err
	}
	pretty.WriteByte('\n')
	_, err = os.Stdout.Write(pretty.Bytes())
	return err
}

func newClientFromEnvironment() (*xhs.Client, error) {
	appIDRaw := strings.TrimSpace(os.Getenv("XHS_JG_APP_ID"))
	secret := strings.TrimSpace(os.Getenv("XHS_JG_SECRET"))
	if appIDRaw == "" || secret == "" {
		return nil, errors.New("XHS_JG_APP_ID and XHS_JG_SECRET are required")
	}
	appID, err := strconv.ParseInt(appIDRaw, 10, 64)
	if err != nil || appID <= 0 {
		return nil, errors.New("XHS_JG_APP_ID must be a positive integer")
	}
	return xhs.NewClient(appID, secret)
}

func readAuthCode(flagValue string) (string, error) {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(os.Getenv("XHS_JG_AUTH_CODE")); value != "" {
		return value, nil
	}
	fmt.Fprint(os.Stderr, "Auth code: ")
	if term.IsTerminal(int(syscall.Stdin)) {
		value, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("read auth code: %w", err)
		}
		authCode := strings.TrimSpace(string(value))
		if authCode == "" {
			return "", errors.New("auth code is required")
		}
		return authCode, nil
	}
	value, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read auth code: %w", err)
	}
	authCode := strings.TrimSpace(value)
	if authCode == "" {
		return "", errors.New("auth code is required")
	}
	return authCode, nil
}

func defaultSessionPath() string {
	if value := strings.TrimSpace(os.Getenv("XHS_JG_SESSION_FILE")); value != "" {
		return value
	}
	return filepath.Join(".xhs-jg", "session.json")
}

func durationFromEnvironment(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func requireLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse listen address: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("XHS Spotlight auth daemon must listen on a loopback address")
	}
	return nil
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, "Usage:")
	fmt.Fprintln(output, "  xhs-jg-authd serve")
	fmt.Fprintln(output, "  xhs-jg-authd authorize")
	fmt.Fprintln(output, "  xhs-jg-authd status")
	fmt.Fprintln(output, "  xhs-jg-authd refresh")
	fmt.Fprintln(output, "  xhs-jg-authd sync-status")
	fmt.Fprintln(output, "  xhs-jg-authd sync-campaigns [--mode incremental|full] [--advertiser-id ID]")
	fmt.Fprintln(output, "  xhs-jg-authd sync-units [--mode incremental|full] [--advertiser-id ID]")
	fmt.Fprintln(output, "  xhs-jg-authd sync-creativities [--advertiser-id ID]")
	fmt.Fprintln(output, "  xhs-jg-authd sync-daily [--timeout DURATION] [--poll-interval DURATION]")
}
