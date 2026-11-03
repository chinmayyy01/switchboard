package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chinmay/switchboard/internal/api"
	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/hub"
	"github.com/chinmay/switchboard/internal/jobs"
	"github.com/chinmay/switchboard/internal/metrics"
	"github.com/chinmay/switchboard/internal/obs"
	"github.com/chinmay/switchboard/internal/openai"
	"github.com/chinmay/switchboard/internal/store"
	"github.com/chinmay/switchboard/internal/tenant"
	"github.com/chinmay/switchboard/internal/ws"
)

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	dataDir := flag.String("data", "data", "call-record data directory")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracer, err := obs.Init(ctx)
	if err != nil {
		slog.Error("otel init", "err", err)
	} else {
		defer shutdownTracer(context.Background())
	}

	authPath := os.Getenv("AUTH_DB")
	if authPath == "" {
		authPath = filepath.Join(*dataDir, "auth.db")
	}
	authStore, err := auth.Open(authPath)
	if err != nil {
		slog.Error("auth store", "err", err)
		os.Exit(1)
	}
	defer authStore.Close()
	if n, err := authStore.CountAccounts(ctx); err == nil && n == 0 {
		slog.Warn("no accounts; create the owner with cmd/account")
	}

	limiter := auth.NewCallLimiter()
	attempts := auth.NewAttemptLimiter(5, 15*time.Minute)
	ttl := auth.SessionTTL()
	secureCookie := auth.SecureCookie()
	origin := os.Getenv("APP_ORIGIN")
	if strings.TrimSpace(origin) == "" {
		origin = "http://localhost:3000,http://127.0.0.1:3000"
	}
	origins := strings.Split(origin, ",")
	ws.SetAllowedOrigins(origins)

	h := hub.New()
	st := store.New(*dataDir, 500)
	reg := ws.NewBridgeRegistry()
	jobsReg := jobs.NewRegistry("tenants")

	tenants, err := tenant.Load("tenants")
	if err != nil {
		slog.Warn("tenant config", "err", err)
		tenants = tenant.Default()
	}

	requireAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAccount(authStore, next)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", auth.LoginHandler(authStore, attempts, ttl, secureCookie))
	mux.HandleFunc("POST /api/auth/logout", auth.LogoutHandler(authStore, secureCookie))
	mux.HandleFunc("GET /api/auth/me", requireAuth(auth.MeHandler(limiter)))
	mux.HandleFunc("/ws", requireAuth(ws.Handler(h, st, reg, jobsReg, tenants, authStore, limiter, ctx)))
	mux.HandleFunc("/ws/supervisor", requireAuth(ws.SupervisorHandler(h, ctx)))
	mux.HandleFunc("/ws/supervisor/call", requireAuth(ws.SupervisorCallHandler(reg, h, ctx)))
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("GET /api/calls", requireAuth(api.Calls(st)))
	mux.HandleFunc("POST /api/calls/{id}/label", requireAuth(api.Label(st)))
	mux.HandleFunc("POST /api/calls/{id}/takeover", requireAuth(api.TakeOver(h)))
	mux.HandleFunc("GET /api/report", requireAuth(api.Report(st)))
	mux.HandleFunc("GET /api/geocode", api.Geocode())
	mux.HandleFunc("GET /api/staff", requireAuth(api.Staff(jobsReg)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		brain := "rules"
		if openai.NewSarvamFromEnv() != nil {
			brain = "llm/sarvam"
		} else if openai.NewFromEnv() != nil {
			brain = "llm/openai"
		} else if mode := os.Getenv("BRAIN"); mode == "llm" {
			brain = "llm/ollama"
		} else if mode == "hybrid" {
			brain = "hybrid/ollama"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":        true,
			"brain":     brain,
			"inference": os.Getenv("INFERENCE_URL") != "",
			"tenants":   tenants.IDs(),
		})
	})

	srv := &http.Server{Addr: *addr, Handler: withCORS(mux, origins)}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	slog.Info("switchboard listening", "addr", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}
}

func withCORS(next http.Handler, origins []string) http.Handler {
	allowed := map[string]bool{}
	for _, origin := range origins {
		origin = strings.TrimSpace(strings.TrimRight(origin, "/"))
		if origin != "" {
			allowed[origin] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
