package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"paipai-red-campaign-manager/internal/config"
	"paipai-red-campaign-manager/internal/delivery"
	"paipai-red-campaign-manager/internal/embedding"
	larksource "paipai-red-campaign-manager/internal/lark"
	"paipai-red-campaign-manager/internal/maituo"
	"paipai-red-campaign-manager/internal/model"
	"paipai-red-campaign-manager/internal/store"
	"paipai-red-campaign-manager/internal/syncer"
)

type baseSyncService interface {
	Run(context.Context) (model.SyncResult, error)
}

type manuscriptSyncService interface {
	RunProviders(context.Context, []string) (model.ProviderSyncResult, error)
}

type noteEmbeddingService interface {
	Refresh(context.Context, bool) (model.NoteEmbeddingRefreshResult, error)
}

type dandelionStatusStore interface {
	LarkSyncRuns(context.Context, int) ([]store.LarkSyncRun, error)
}

type manuscriptStatusStore interface {
	ProviderContentTables(context.Context) ([]model.ProviderContentTable, error)
}

type manuscriptAssetStore interface {
	ManuscriptAsset(context.Context, string) (maituo.ManuscriptAsset, bool, error)
}

type apiServer struct {
	dandelionSync        baseSyncService
	dandelionStatus      dandelionStatusStore
	coenzymeQ10Sync      coenzymeQ10SyncService
	coenzymeQ10Status    coenzymeQ10StatusStore
	dandelionExcelImport dandelionExcelImportStore
	manuscriptSync       manuscriptSyncService
	embeddingRefresh     noteEmbeddingService
	statusStore          manuscriptStatusStore
	manuscriptAssets     manuscriptAssetStore
	manualMaterials      manualMaterialStore
	maituoImport         maituoImportStore
	maituoSubaccounts    maituoSubaccountStore
	maituoProviders      maituoProviderStore
	maituoAnalytics      maituoAnalyticsStore
	maituoXHSLinksStore  maituoXHSLinkStore
	spotlightStore       spotlightCampaignStore
	guoraiAnalytics      guoraiAnalyticsStore
	businessOverview     businessOverviewStore
	contentAnalysis      contentAnalysisStore
	delivery             *delivery.Service
	deliveryCredentials  []deliveryCredential
	timeout              time.Duration
	logger               *slog.Logger
}

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type manuscriptSyncRequest struct {
	ProviderCodes []string `json:"provider_codes,omitempty"`
}

type providerStatus struct {
	ProviderCode string     `json:"provider_code"`
	ProviderName string     `json:"provider_name"`
	SheetName    string     `json:"sheet_name"`
	Status       string     `json:"status"`
	Error        string     `json:"error,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

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
	if err := requireLoopbackAddress(cfg.LarkSyncListen); err != nil {
		return err
	}
	deliveryCredentials, err := parseDeliveryCredentials(cfg.DeliveryCredentialsJSON)
	if err != nil {
		return fmt.Errorf("configure delivery API credentials: %w", err)
	}

	destination, err := store.NewPostgres(ctx, cfg.DatabaseURL, cfg.LarkAppToken)
	if err != nil {
		return err
	}
	defer destination.Close()
	if err := destination.Migrate(ctx); err != nil {
		return err
	}
	if err := destination.FailRunningProviderContentSyncs(ctx, "manual sync service restarted before the request finished"); err != nil {
		return err
	}
	if err := destination.FailRunningCoenzymeQ10Syncs(ctx, "manual sync service restarted before the request finished"); err != nil {
		return err
	}
	if err := destination.FailInterruptedPublishJobs(ctx, "delivery service restarted before publishing completed"); err != nil {
		return err
	}
	embeddingClient, err := embedding.NewClient(cfg.BailianAPIKey, cfg.BailianBaseURL, nil)
	if err != nil {
		return fmt.Errorf("configure Bailian embeddings: %w", err)
	}
	embeddingRefresher, err := embedding.NewRefresher(
		destination, embeddingClient, cfg.BailianEmbeddingModel, cfg.BailianDimensions,
	)
	if err != nil {
		return fmt.Errorf("configure note embedding refresh: %w", err)
	}
	deliveryGateway, err := delivery.NewHTTPGateway(cfg.XHSJGAuthdURL, cfg.XHSJGInternalAPIKey, nil)
	if err != nil {
		return fmt.Errorf("configure delivery XHS gateway: %w", err)
	}
	var semanticAdvisor delivery.SemanticAdvisor = delivery.RuleSemanticAdvisor{}
	if cfg.DeliveryLLMBaseURL != "" && cfg.DeliveryLLMModel != "" {
		llmAdvisor, advisorErr := delivery.NewOpenAICompatibleAdvisor(cfg.BailianAPIKey, cfg.DeliveryLLMBaseURL, cfg.DeliveryLLMModel, nil)
		if advisorErr != nil {
			return fmt.Errorf("configure delivery LLM: %w", advisorErr)
		}
		semanticAdvisor = delivery.FallbackAdvisor{Primary: llmAdvisor, Fallback: delivery.RuleSemanticAdvisor{}}
	}
	var deliveryRanker delivery.Ranker = delivery.HeuristicRanker{}
	if cfg.DeliveryRankerURL != "" && cfg.DeliveryRankerModel != "" {
		remoteRanker, rankerErr := delivery.NewRemoteRanker(cfg.DeliveryRankerURL, cfg.DeliveryRankerAPIKey, cfg.DeliveryRankerModel, nil)
		if rankerErr != nil {
			return fmt.Errorf("configure delivery ranker: %w", rankerErr)
		}
		deliveryRanker = delivery.FallbackRanker{Primary: remoteRanker, Fallback: delivery.HeuristicRanker{}}
	}
	deliveryService, err := delivery.NewService(destination, deliveryGateway, semanticAdvisor, deliveryRanker, cfg.DeliveryMediaWrites)
	if err != nil {
		return fmt.Errorf("configure delivery service: %w", err)
	}
	go deliveryService.RunPublisher(ctx, func(publishErr error) {
		logger.Error("delivery publisher failed", "error", publishErr)
	})

	dandelionDestination, err := store.NewTableScopedPostgres(ctx, cfg.DatabaseURL, cfg.LarkDandelionAppToken)
	if err != nil {
		return err
	}
	defer dandelionDestination.Close()

	source := larksource.NewClient(cfg.LarkAppID, cfg.LarkAppSecret, cfg.LarkAppToken)
	dandelionSource := larksource.NewSingleTableSource(
		cfg.LarkAppID,
		cfg.LarkAppSecret,
		cfg.LarkDandelionAppToken,
		cfg.LarkDandelionTableID,
	)
	handler := newAPIHandler(&apiServer{
		dandelionSync:        syncer.New(dandelionSource, dandelionDestination, cfg.DocumentRefreshInterval),
		dandelionStatus:      dandelionDestination,
		dandelionExcelImport: destination,
		coenzymeQ10Sync:      syncer.NewCoenzymeQ10(source, destination, cfg.CoenzymeQ10WikiToken, cfg.CoenzymeQ10SheetID, cfg.CoenzymeQ10SheetName),
		coenzymeQ10Status:    destination,
		manuscriptSync:       syncer.NewProvider(source, destination),
		embeddingRefresh:     embeddingRefresher,
		statusStore:          destination,
		manuscriptAssets:     destination,
		manualMaterials:      destination,
		maituoImport:         destination,
		maituoSubaccounts:    destination,
		maituoProviders:      destination,
		maituoAnalytics:      destination,
		maituoXHSLinksStore:  destination,
		spotlightStore:       destination,
		guoraiAnalytics:      destination,
		businessOverview:     destination,
		contentAnalysis:      destination,
		delivery:             deliveryService,
		deliveryCredentials:  deliveryCredentials,
		timeout:              cfg.SyncTimeout,
		logger:               logger,
	})
	listener, err := net.Listen("tcp", cfg.LarkSyncListen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.LarkSyncListen, err)
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.SyncTimeout + 5*time.Second,
		WriteTimeout:      cfg.SyncTimeout + 5*time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	serverErrors := make(chan error, 1)
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErrors <- serveErr
		}
	}()

	logger.Info("Lark manual sync API started",
		"listen", listener.Addr().String(),
		"manual_only", true,
		"targets", []string{"dandelion", "manuscripts", "cid"},
		"sync_timeout", cfg.SyncTimeout,
	)
	select {
	case <-ctx.Done():
		logger.Info("Lark manual sync API shutdown requested")
	case serveErr := <-serverErrors:
		return fmt.Errorf("serve Lark manual sync API: %w", serveErr)
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown Lark manual sync API: %w", err)
	}
	return nil
}

func newAPIHandler(server *apiServer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.health)
	mux.HandleFunc("/v1/sync/dandelion", server.syncDandelion)
	mux.HandleFunc("/v1/sync/dandelion/status", server.dandelionStatusHandler)
	mux.HandleFunc("/v1/imports/dandelion-excel", server.importDandelionExcel)
	mux.HandleFunc("/v1/imports/maituo-customer-daily", server.importMaituoCustomerDaily)
	mux.HandleFunc("/v1/sync/cid", server.syncCoenzymeQ10)
	mux.HandleFunc("/v1/sync/cid/status", server.coenzymeQ10StatusHandler)
	mux.HandleFunc("/v1/sync/coenzyme-q10", server.syncCoenzymeQ10)
	mux.HandleFunc("/v1/sync/coenzyme-q10/status", server.coenzymeQ10StatusHandler)
	mux.HandleFunc("/v1/imports/maituo-subaccount-directories", server.listMaituoSubaccountDirectories)
	mux.HandleFunc(maituoSubaccountDownloadPrefix, server.maituoSubaccountDownload)
	mux.HandleFunc("/v1/imports/maituo-provider-directories", server.listMaituoProviderDirectories)
	mux.HandleFunc(maituoProviderDownloadPrefix, server.maituoProviderDownload)
	mux.HandleFunc("/v1/analytics/maituo/note-campaigns", server.maituoNoteCampaignAnalysis)
	mux.HandleFunc("/v1/analytics/maituo/note-content", server.maituoNoteContent)
	mux.HandleFunc("/v1/analytics/maituo/reference-materials", server.maituoReferenceMaterials)
	mux.HandleFunc("/v1/analytics/maituo/reference-material-content", server.maituoReferenceMaterialContent)
	mux.HandleFunc("/v1/analytics/maituo/manual-materials", server.maituoManualMaterials)
	mux.HandleFunc("/v1/analytics/maituo/manual-material", server.maituoManualMaterial)
	mux.HandleFunc("/v1/analytics/maituo/manual-material-tags", server.maituoManualMaterialTags)
	mux.HandleFunc(manuscriptAssetPrefix, server.manuscriptAsset)
	mux.HandleFunc("/v1/analytics/maituo/account-plan-diagnosis", server.maituoAccountPlanDiagnosis)
	mux.HandleFunc("/v1/analytics/maituo/traffic-comparisons", server.maituoTrafficComparison)
	mux.HandleFunc("/v1/analytics/maituo/traffic-comparison-delivery", server.maituoTrafficDeliveryComparison)
	mux.HandleFunc("/v1/analytics/maituo/xhs-links", server.maituoXHSLinks)
	mux.HandleFunc("/v1/analytics/spotlight/campaigns", server.spotlightCampaigns)
	mux.HandleFunc("/v1/analytics/spotlight/campaign-detail", server.spotlightCampaignDetail)
	mux.HandleFunc("/v1/analytics/guorai/latest", server.guoraiLatest)
	mux.HandleFunc("/v1/analytics/overview", server.businessOverviewHandler)
	mux.HandleFunc("/v1/analytics/content-analysis", server.contentAnalysisHandler)
	mux.HandleFunc("/v1/sync/manuscripts", server.syncManuscripts)
	mux.HandleFunc("/v1/sync/manuscripts/status", server.manuscriptStatus)
	mux.HandleFunc("/v1/delivery", server.deliveryOverview)
	mux.HandleFunc("/v1/delivery/openapi.json", server.deliveryOpenAPI)
	mux.HandleFunc("/v1/delivery/session", server.deliverySession)
	mux.HandleFunc("/v1/delivery/capabilities", server.deliveryCapabilities)
	mux.HandleFunc("/v1/delivery/assets", server.deliveryAssets)
	mux.HandleFunc("/v1/delivery/assets/platform", server.deliveryPlatformAssets)
	mux.HandleFunc("/v1/delivery/target-options", server.deliveryTargetOptions)
	mux.HandleFunc("/v1/delivery/keyword-candidates", server.deliveryKeywordCandidates)
	mux.HandleFunc("/v1/delivery/negative-keywords", server.deliveryNegativeKeywords)
	mux.HandleFunc("/v1/delivery/audience-estimates", server.deliveryAudienceEstimates)
	mux.HandleFunc("/v1/delivery/campaigns/query", server.deliveryCampaignQuery)
	mux.HandleFunc("/v1/delivery/campaigns/status", server.deliveryCampaignStatus)
	mux.HandleFunc("/v1/delivery/units/query", server.deliveryUnitQuery)
	mux.HandleFunc("/v1/delivery/creativities/query", server.deliveryCreativityQuery)
	mux.HandleFunc("/v1/delivery/drafts", server.deliveryDrafts)
	mux.HandleFunc("/v1/delivery/drafts/", server.deliveryDraft)
	mux.HandleFunc("/v1/delivery/jobs/", server.deliveryJob)
	mux.HandleFunc("/v1/delivery/entities/", server.deliveryEntity)
	mux.HandleFunc("/v1/delivery/performance", server.deliveryPerformance)
	mux.HandleFunc("/v1/delivery/intelligence/capabilities", server.deliveryIntelligenceCapabilities)
	mux.HandleFunc("/v1/delivery/intelligence/bayesian", server.deliveryBayesian)
	mux.HandleFunc("/v1/delivery/intelligence/optimize-budget", server.deliveryOptimizeBudget)
	mux.HandleFunc("/v1/delivery/intelligence/bandit-shadow", server.deliveryBanditShadow)
	return noStoreHeaders(mux)
}

func (server *apiServer) health(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: map[string]string{"status": "ok"}})
}

func (server *apiServer) dandelionStatusHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	runs, err := server.dandelionStatus.LarkSyncRuns(ctx, 10)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: map[string]interface{}{"recent": runs}})
}

func (server *apiServer) syncDandelion(writer http.ResponseWriter, request *http.Request) {
	server.syncBitableTarget(writer, request, "dandelion", server.dandelionSync)
}

func (server *apiServer) syncBitableTarget(writer http.ResponseWriter, request *http.Request, target string, service baseSyncService) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	if !decodeOptionalEmptyRequest(writer, request) {
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	startedAt := time.Now()
	result, err := service.Run(ctx)
	server.logger.Info("Lark Bitable manual sync finished",
		"target", target,
		"result", result,
		"duration", time.Since(startedAt),
		"error", err,
	)
	writeSyncResult(writer, result, err)
}

func (server *apiServer) syncManuscripts(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}
	var payload manuscriptSyncRequest
	if !decodeOptionalJSON(writer, request, &payload) {
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	startedAt := time.Now()
	result, err := server.manuscriptSync.RunProviders(ctx, payload.ProviderCodes)
	if err == nil && server.embeddingRefresh != nil {
		embeddingResult, embeddingErr := server.embeddingRefresh.Refresh(ctx, false)
		result.Embeddings = &embeddingResult
		if embeddingErr != nil {
			err = fmt.Errorf("manuscript sync succeeded but embedding refresh failed: %w", embeddingErr)
		}
	}
	server.logger.Info("Lark manuscript manual sync finished",
		"provider_codes", payload.ProviderCodes,
		"result", result,
		"duration", time.Since(startedAt),
		"error", err,
	)
	writeSyncResult(writer, result, err)
}

func (server *apiServer) manuscriptStatus(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), server.timeout)
	defer cancel()
	tables, err := server.statusStore.ProviderContentTables(ctx)
	if err != nil {
		writeJSON(writer, http.StatusBadGateway, apiResponse{Success: false, Error: err.Error()})
		return
	}
	providers := make([]providerStatus, 0, len(tables))
	for _, table := range tables {
		providers = append(providers, providerStatus{
			ProviderCode: table.ProviderCode,
			ProviderName: table.ProviderName,
			SheetName:    table.SheetName,
			Status:       table.LastSyncStatus,
			Error:        table.LastSyncError,
			LastSyncedAt: table.LastSyncedAt,
		})
	}
	writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: map[string]interface{}{"providers": providers}})
}

func decodeOptionalEmptyRequest(writer http.ResponseWriter, request *http.Request) bool {
	var payload struct{}
	return decodeOptionalJSON(writer, request, &payload)
}

func decodeOptionalJSON(writer http.ResponseWriter, request *http.Request, target interface{}) bool {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "invalid JSON request"})
		return false
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSON(writer, http.StatusBadRequest, apiResponse{Success: false, Error: "request body must contain one JSON object"})
		return false
	}
	return true
}

func writeSyncResult(writer http.ResponseWriter, result interface{}, err error) {
	if err == nil {
		writeJSON(writer, http.StatusOK, apiResponse{Success: true, Data: result})
		return
	}
	status := http.StatusBadGateway
	switch {
	case errors.Is(err, syncer.ErrAlreadyRunning), errors.Is(err, store.ErrSyncLocked), errors.Is(err, store.ErrNoteEmbeddingRefreshLocked), errors.Is(err, store.ErrCoenzymeQ10SyncLocked):
		status = http.StatusConflict
	case errors.Is(err, syncer.ErrUnknownProvider):
		status = http.StatusBadRequest
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	case errors.Is(err, context.Canceled):
		status = http.StatusServiceUnavailable
	}
	writeJSON(writer, status, apiResponse{Success: false, Data: result, Error: err.Error()})
}

func writeJSON(writer http.ResponseWriter, status int, payload apiResponse) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func methodNotAllowed(writer http.ResponseWriter, method string) {
	writer.Header().Set("Allow", method)
	writeJSON(writer, http.StatusMethodNotAllowed, apiResponse{Success: false, Error: "method not allowed"})
}

func noStoreHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(writer, request)
	})
}

func requireLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse LARK_SYNC_LISTEN: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("Lark sync API must listen on a loopback address")
	}
	return nil
}
