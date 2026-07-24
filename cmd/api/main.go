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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/Vedoputra/LLUNARA-BE/internal/config"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

const version = "0.1.0"

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := repository.NewPool(startupCtx, cfg.DatabaseURL)
	cancelStartup()
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	router := newRouter(pool)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

func newRouter(pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", handleHealth(pool))

	r.Route("/api/v1", func(r chi.Router) {
		// Endpoint bisnis didaftarkan di sini seiring fase berikutnya.
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
