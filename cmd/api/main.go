package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/Vedoputra/LLUNARA-BE/internal/config"
	"github.com/Vedoputra/LLUNARA-BE/internal/handler"
	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
	"github.com/Vedoputra/LLUNARA-BE/internal/service"
)

const version = "0.1.0"

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	startupCtx, cancelStartup := context.WithTimeout(ctx, 10*time.Second)
	pool, err := repository.NewPool(startupCtx, cfg.DatabaseURL)
	cancelStartup()
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	// jwks keeps its own HTTP client and background refresh goroutine tied
	// to ctx, so it stops automatically on shutdown.
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{cfg.SupabaseJWKSURL})
	if err != nil {
		log.Fatalf("jwks: %v", err)
	}

	router := newRouter(pool, jwks)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	stop()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown: %v", err)
	}
	slog.Info("server stopped")
}

func newRouter(pool *pgxpool.Pool, jwks keyfunc.Keyfunc) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	r.Get("/health", handleHealth(pool))

	cycleRepo := repository.NewCycleRepository(pool)
	cycleHandler := handler.NewCycleHandler(service.NewCycleService(cycleRepo))

	dailyLogRepo := repository.NewDailyLogRepository(pool)
	symptomRepo := repository.NewSymptomRepository(pool)
	dailyLogHandler := handler.NewDailyLogHandler(service.NewDailyLogService(dailyLogRepo, cycleRepo, symptomRepo))
	symptomHandler := handler.NewSymptomHandler(service.NewSymptomService(symptomRepo))

	insightRepo := repository.NewInsightRepository(pool)
	insightHandler := handler.NewInsightHandler(service.NewInsightService(cycleRepo, insightRepo))

	wellnessRepo := repository.NewWellnessRepository(pool)
	wellnessHandler := handler.NewWellnessHandler(service.NewWellnessService(wellnessRepo))

	exportRepo := repository.NewExportRepository(pool)
	exportHandler := handler.NewExportHandler(service.NewExportService(exportRepo, wellnessRepo, cycleRepo))

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(jwks))

		r.Get("/me", handler.HandleMe)

		r.Post("/cycles", cycleHandler.StartCycle)
		r.Get("/cycles", cycleHandler.ListCycles)
		r.Get("/cycles/prediction", cycleHandler.GetPrediction)
		r.Patch("/cycles/{id}", cycleHandler.UpdateCycle)
		r.Delete("/cycles/{id}", cycleHandler.DeleteCycle)

		r.Post("/daily-logs", dailyLogHandler.UpsertLog)
		r.Get("/daily-logs", dailyLogHandler.ListLogs)
		r.Delete("/daily-logs/{date}", dailyLogHandler.DeleteLog)

		r.Get("/symptoms", symptomHandler.ListSymptoms)
		r.Post("/symptoms", symptomHandler.CreateSymptom)
		r.Delete("/symptoms/{id}", symptomHandler.DeleteSymptom)

		r.Get("/insights/summary", insightHandler.GetCycleSummary)
		r.Get("/insights/symptoms", insightHandler.GetSymptomInsights)
		r.Get("/insights/mood", insightHandler.GetMoodInsights)

		r.Post("/wellness", wellnessHandler.UpsertLog)
		r.Get("/wellness", wellnessHandler.ListLogs)

		r.Post("/export", exportHandler.Export)
	})

	return r
}

// handleHealth issues a real query against the database so that scheduled
// health checks also count as Supabase activity, preventing the free-tier
// project from auto-pausing after 7 idle days.
func handleHealth(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		status := "ok"
		httpStatus := http.StatusOK

		var one int
		if err := pool.QueryRow(ctx, "select 1").Scan(&one); err != nil {
			slog.Error("health check: database query failed", "error", err)
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
		}

		resp := map[string]string{
			"status":    status,
			"version":   version,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
