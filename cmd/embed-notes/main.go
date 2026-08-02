package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"paipai-red-campaign-manager/internal/config"
	"paipai-red-campaign-manager/internal/embedding"
	"paipai-red-campaign-manager/internal/store"
)

func main() {
	force := flag.Bool("force", false, "regenerate embeddings even when the content hash is unchanged")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *force); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, force bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	destination, err := store.NewPostgres(ctx, cfg.DatabaseURL, cfg.LarkAppToken)
	if err != nil {
		return err
	}
	defer destination.Close()
	if err := destination.Migrate(ctx); err != nil {
		return err
	}
	client, err := embedding.NewClient(cfg.BailianAPIKey, cfg.BailianBaseURL, nil)
	if err != nil {
		return fmt.Errorf("configure Bailian embeddings: %w", err)
	}
	refresher, err := embedding.NewRefresher(
		destination, client, cfg.BailianEmbeddingModel, cfg.BailianDimensions,
	)
	if err != nil {
		return fmt.Errorf("configure note embedding refresh: %w", err)
	}
	result, refreshErr := refresher.Refresh(ctx, force)
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return fmt.Errorf("encode embedding result: %w", err)
	}
	return refreshErr
}
