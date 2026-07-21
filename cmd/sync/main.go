package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"paipai-red-campaign-manager/internal/config"
	larksource "paipai-red-campaign-manager/internal/lark"
	"paipai-red-campaign-manager/internal/store"
	"paipai-red-campaign-manager/internal/syncer"

	"github.com/robfig/cron/v3"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	location, err := time.LoadLocation(cfg.SyncTimezone)
	if err != nil {
		return errors.New("load SYNC_TIMEZONE: " + err.Error())
	}

	destination, err := store.NewPostgres(ctx, cfg.DatabaseURL, cfg.LarkAppToken)
	if err != nil {
		return err
	}
	defer destination.Close()

	if err := destination.Migrate(ctx); err != nil {
		return err
	}

	source := larksource.NewClient(cfg.LarkAppID, cfg.LarkAppSecret, cfg.LarkAppToken)
	service := syncer.New(source, destination, cfg.DocumentRefreshInterval)
	providerService := syncer.NewProvider(source, destination)
	runSync := func() {
		syncCtx, cancel := context.WithTimeout(ctx, cfg.SyncTimeout)
		defer cancel()

		startedAt := time.Now()
		result, err := service.Run(syncCtx)
		if err != nil {
			if errors.Is(err, syncer.ErrAlreadyRunning) {
				logger.Info("Bitable sync skipped because another run is active")
				return
			}
			logger.Error("Bitable sync failed", "error", err, "duration", time.Since(startedAt))
			return
		}
		logger.Info("Bitable sync completed",
			"tables", result.Tables,
			"fetched", result.Fetched,
			"upserted", result.Upserted,
			"deleted", result.Deleted,
			"documents_fetched", result.Documents,
			"document_errors", result.DocumentErrors,
			"duration", time.Since(startedAt),
		)
	}
	runProviderSync := func() {
		syncCtx, cancel := context.WithTimeout(ctx, cfg.SyncTimeout)
		defer cancel()

		startedAt := time.Now()
		result, err := providerService.Run(syncCtx)
		if err != nil {
			if errors.Is(err, syncer.ErrAlreadyRunning) {
				logger.Info("provider content sync skipped because another run is active")
				return
			}
			logger.Error("provider content sync failed", "error", err, "duration", time.Since(startedAt))
			return
		}
		logger.Info("provider content sync completed",
			"providers", result.Providers,
			"fetched", result.Fetched,
			"upserted", result.Upserted,
			"notes", result.Notes,
			"note_errors", result.NoteErrors,
			"deleted", result.Deleted,
			"duration", time.Since(startedAt),
		)
	}
	runAllSyncs := func() {
		runSync()
		runProviderSync()
	}

	scheduler := cron.New(cron.WithLocation(location))
	if _, err := scheduler.AddFunc(cfg.SyncCron, runAllSyncs); err != nil {
		return errors.New("parse SYNC_CRON: " + err.Error())
	}
	scheduler.Start()
	defer func() { <-scheduler.Stop().Done() }()

	logger.Info("sync service started",
		"cron", cfg.SyncCron,
		"timezone", cfg.SyncTimezone,
		"document_refresh_interval", cfg.DocumentRefreshInterval,
	)
	if cfg.SyncOnStart {
		runAllSyncs()
	}

	<-ctx.Done()
	logger.Info("shutdown requested")
	return nil
}
