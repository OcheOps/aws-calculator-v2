package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/auth"
	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/OcheOps/aws-calculator/api/internal/config"
	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/OcheOps/aws-calculator/api/internal/estimates"
	"github.com/OcheOps/aws-calculator/api/internal/fx"
	httpx "github.com/OcheOps/aws-calculator/api/internal/http"
	"github.com/OcheOps/aws-calculator/api/internal/httpapi"
	"github.com/OcheOps/aws-calculator/api/internal/ingest"
	"github.com/OcheOps/aws-calculator/api/internal/ingest/pricefiles"
	"github.com/OcheOps/aws-calculator/api/internal/mail"
	"github.com/OcheOps/aws-calculator/api/internal/pricing"
	"github.com/OcheOps/aws-calculator/api/internal/providers"
	awsprov "github.com/OcheOps/aws-calculator/api/internal/providers/aws"
	"github.com/OcheOps/aws-calculator/api/internal/workers"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		logger.Error("migrations failed", "err", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("db open failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	registry := providers.NewRegistry()
	if err := registerAWS(ctx, registry, cfg, logger, pool); err != nil {
		logger.Warn("aws provider not registered", "err", err)
	}

	orch := ingest.NewOrchestrator(pool).WithLogger(logger)
	trigger := &ingestTrigger{orch: orch, registry: registry, pool: pool, logger: logger}

	sched := workers.NewScheduler().WithLogger(logger)
	if _, hasAWS := registry.Get("aws"); hasAWS {
		if err := sched.Add(cfg.IngestCron, func(c context.Context) error {
			_, err := trigger.Trigger("aws")
			return err
		}); err != nil {
			logger.Warn("failed to schedule aws ingest", "err", err)
		}
	}
	fxStore := fx.NewStore(pool)
	fxRefresher := fx.NewRefresher(fx.NewHTTPClient(cfg.FXAPIURL), fxStore, []string{"EUR", "GBP", "NGN"})
	if err := sched.Add(cfg.FXCron, func(c context.Context) error {
		return fxRefresher.Refresh(c)
	}); err != nil {
		logger.Warn("failed to schedule fx refresh", "err", err)
	}
	go func() {
		if err := fxRefresher.Refresh(context.Background()); err != nil {
			logger.Warn("initial fx refresh failed", "err", err)
		}
	}()

	sched.Start(ctx)
	defer sched.Stop()

	store := catalog.NewStore(pool)
	priceCache, err := pricing.NewCache(1<<15, 5*time.Minute)
	if err != nil {
		logger.Error("price cache init failed", "err", err)
		os.Exit(1)
	}
	priceLookup := pricing.NewLookup(pricing.NewStore(pool), fxStore, priceCache, store)

	var mailer mail.Mailer = mail.NewFake()
	if cfg.SMTPUser != "" && cfg.SMTPPass != "" {
		mailer = mail.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)
		logger.Info("smtp mailer configured", "host", cfg.SMTPHost)
	} else {
		mailer = mail.NewFakeWithLogging()
		logger.Warn("no SMTP credentials; using fake mailer (magic links printed to stdout)")
	}
	authStore := auth.NewStore(pool)
	authSvc := auth.NewService(
		authStore, mailer,
		auth.NewLimiter(5, time.Hour), auth.NewLimiter(5, time.Hour),
		cfg.BaseURL,
	)
	estStore := estimates.NewStore(pool)
	repricer := estimates.NewRepricer(estStore, priceLookup)
	authSvc.WithClaimer(estStore)

	router := httpx.NewRouter(httpx.Deps{
		Ready:       combinedReady{pool: pool, catalog: httpx.CatalogReadiness{Store: store}},
		Logger:      logger,
		Catalog:     store,
		AdminToken:  cfg.AdminToken,
		Ingest:      trigger,
		Pricing:     priceLookup,
		Auth:        authSvc,
		AuthOptions: httpapi.AuthOptions{SecureCookie: cfg.CookieSecure, CookieDomain: cfg.CookieDomain},
		Estimates: httpapi.EstimatesDeps{
			Store: estStore, Repricer: repricer,
			AuthStore: authStore, BaseURL: cfg.BaseURL,
		},
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("http listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
	logger.Info("shutdown complete")
}

func registerAWS(ctx context.Context, registry *providers.Registry, cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool) error {
	if cfg.AWS.AccessKeyID == "" || cfg.AWS.SecretAccessKey == "" {
		return fmt.Errorf("AWS credentials not configured; ingestion disabled")
	}
	sdk, err := awsprov.NewRealSDK(ctx, cfg.AWS.AccessKeyID, cfg.AWS.SecretAccessKey, cfg.AWS.PricingRegion)
	if err != nil {
		return err
	}
	client := awsprov.NewClient(sdk)
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}

	bulkClient := awsprov.NewBulkClient(sdk)
	pfStore := pricefiles.NewStore(pool)
	ing := awsprov.NewBulkIngestor(client, bulkClient, pfStore, regions, cfg.IngestRegionConcurrency, cfg.IngestBatchSize)
	registry.Register(providerAdapter{
		meta: providers.Metadata{Code: "aws", Name: "Amazon Web Services"},
		run: func(ctx context.Context, deps providers.IngestDeps) error {
			return ing.Run(ctx, deps.DB)
		},
	})
	logger.Info("aws provider registered", "regions", regions, "concurrency", cfg.IngestRegionConcurrency, "batchSize", cfg.IngestBatchSize)
	return nil
}

type providerAdapter struct {
	meta providers.Metadata
	run  func(ctx context.Context, deps providers.IngestDeps) error
}

func (p providerAdapter) Metadata() providers.Metadata { return p.meta }
func (p providerAdapter) Ingest(ctx context.Context, deps providers.IngestDeps) error {
	return p.run(ctx, deps)
}

type ingestTrigger struct {
	mu       sync.Mutex
	running  bool
	orch     *ingest.Orchestrator
	registry *providers.Registry
	pool     *pgxpool.Pool
	logger   *slog.Logger
}

func (t *ingestTrigger) Trigger(providerCode string) (int64, error) {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return 0, fmt.Errorf("ingestion already running")
	}
	t.running = true
	t.mu.Unlock()

	p, ok := t.registry.Get(providerCode)
	if !ok {
		t.mu.Lock()
		t.running = false
		t.mu.Unlock()
		return 0, fmt.Errorf("provider %s not registered", providerCode)
	}

	go func() {
		defer func() {
			t.mu.Lock()
			t.running = false
			t.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Hour)
		defer cancel()
		if err := t.orch.Run(ctx, p); err != nil {
			t.logger.Error("ingestion run failed", "err", err, "provider", providerCode)
		}
	}()
	return 0, nil
}

type combinedReady struct {
	pool    *pgxpool.Pool
	catalog httpx.CatalogReadiness
}

func (c combinedReady) Ready(ctx context.Context) (bool, string) {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.pool.Ping(pingCtx); err != nil {
		return false, "db unreachable: " + err.Error()
	}
	return c.catalog.Ready(ctx)
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
