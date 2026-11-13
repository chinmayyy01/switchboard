package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chinmay/switchboard/internal/agentauth"
	"github.com/chinmay/switchboard/internal/ai"
	"github.com/chinmay/switchboard/internal/api"
	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/billing"
	"github.com/chinmay/switchboard/internal/calls"
	"github.com/chinmay/switchboard/internal/jobs"
	"github.com/chinmay/switchboard/internal/mailer"
	"github.com/chinmay/switchboard/internal/metrics"
	"github.com/chinmay/switchboard/internal/obs"
	"github.com/chinmay/switchboard/internal/openai"
	"github.com/chinmay/switchboard/internal/storage"
	"github.com/chinmay/switchboard/internal/tenant"
	"github.com/redis/go-redis/v9"
)

func main() {
	addr := flag.String("addr", ":8090", "listen address")
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

	authStore, err := auth.OpenDefault()
	if err != nil {
		slog.Error("auth store", "err", err)
		os.Exit(1)
	}
	defer authStore.Close()
	if n, err := authStore.CountAccounts(ctx); err == nil && n == 0 {
		slog.Warn("no accounts; create the owner with cmd/account")
	}

	var redisClient *redis.Client
	limiter := auth.NewCallLimiter()
	attempts := auth.NewAttemptLimiter(5, 15*time.Minute)
	if redisURL := strings.TrimSpace(os.Getenv("REDIS_URL")); redisURL != "" {
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			slog.Error("REDIS_URL", "err", err)
			os.Exit(1)
		}
		redisClient = redis.NewClient(opts)
		if err := redisClient.Ping(ctx).Err(); err != nil {
			slog.Error("redis ping", "err", err)
			os.Exit(1)
		}
		limiter = auth.NewRedisCallLimiter(redisClient, 2*time.Hour)
		attempts = auth.NewRedisAttemptLimiter(redisClient, 5, 15*time.Minute)
		defer redisClient.Close()
		slog.Info("distributed limiters enabled")
	} else {
		slog.Warn("REDIS_URL is not set; call and login limits are per process")
	}
	ttl := auth.SessionTTL()
	secureCookie := auth.SecureCookie()
	mail := mailer.NewFromEnv()
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") && !mailer.Configured() {
		slog.Error("SMTP_HOST is required in production for invites, verification, and password reset")
		os.Exit(1)
	}
	origin := os.Getenv("APP_ORIGIN")
	if strings.TrimSpace(origin) == "" {
		origin = "http://localhost:3000,http://127.0.0.1:3000"
	}
	origins := strings.Split(origin, ",")

	tenants, err := tenant.Load("tenants")
	if err != nil {
		slog.Warn("tenant config", "err", err)
		tenants = tenant.Default()
	}
	bookingDB, err := jobs.OpenPostgres(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("bookings database", "err", err)
		os.Exit(1)
	}
	defer bookingDB.Close()
	jobsReg, err := jobs.NewPostgresRegistry(ctx, bookingDB, "tenants", tenants.IDs())
	if err != nil {
		slog.Error("booking store", "err", err)
		os.Exit(1)
	}
	callsStore := calls.New(bookingDB)
	calls.StartReaper(ctx, callsStore, 5*time.Minute, 2*time.Hour)
	recordingStorage, err := storage.NewFromEnv()
	if err != nil {
		slog.Error("recording storage", "err", err)
		os.Exit(1)
	}
	if recordingStorage == nil {
		slog.Warn("RECORDING_S3_BUCKET is not set; consenting calls will not be recorded")
	}
	recordingAPI := &api.RecordingAPI{Calls: callsStore, Storage: recordingStorage}

	pricingPath := strings.TrimSpace(os.Getenv("PRICING_FILE"))
	if pricingPath == "" {
		pricingPath = "config/pricing.json"
	}
	prices, err := billing.Load(pricingPath)
	if err != nil {
		slog.Error("pricing table", "path", pricingPath, "err", err)
		os.Exit(1)
	}
	billingService := billing.New(
		bookingDB,
		prices,
		envInt64("DEFAULT_MONTHLY_CAP_MINOR", 0),
		os.Getenv("BUDGET_ACTION"),
	)
	agentKey := os.Getenv("AGENT_API_KEY")
	agentTokenSecret := os.Getenv("AGENT_TOKEN_SECRET")
	if agentKey == "" {
		slog.Warn("AGENT_API_KEY is not set; the LiveKit agent cannot start sessions")
	}
	if agentTokenSecret == "" {
		agentTokenSecret = agentKey
	}
	if agentKey != "" && agentTokenSecret == agentKey {
		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			slog.Error("AGENT_TOKEN_SECRET must be set and different from AGENT_API_KEY in production")
			os.Exit(1)
		}
		slog.Warn("AGENT_TOKEN_SECRET is missing or equals AGENT_API_KEY; set a distinct value before production")
	}
	tools := &api.Tools{
		Jobs:        jobsReg,
		Calls:       callsStore,
		AI:          ai.NewFromEnv(),
		Tenants:     tenants,
		Billing:     billingService,
		AgentKey:    agentKey,
		TokenSecret: agentTokenSecret,
	}
	requireAgent := func(next http.HandlerFunc) http.HandlerFunc {
		return agentauth.Require(agentTokenSecret, next)
	}

	requireAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAccount(authStore, next)
	}
	requireRole := func(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
		return auth.RequireRole(roles...)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", auth.LoginHandler(authStore, attempts, ttl, secureCookie))
	mux.HandleFunc("POST /api/auth/logout", auth.LogoutHandler(authStore, secureCookie))
	mux.HandleFunc("GET /api/auth/me", requireAuth(auth.MeHandler(limiter)))
	mux.HandleFunc("GET /api/auth/invite", auth.InviteInfoHandler(authStore))
	mux.HandleFunc("POST /api/auth/signup", auth.SignupHandler(authStore, mail, attempts, tenants, "tenants"))
	mux.HandleFunc("POST /api/auth/verify", auth.VerifyHandler(authStore, ttl, secureCookie))
	mux.HandleFunc("POST /api/auth/resend-verification", auth.ResendVerificationHandler(authStore, mail, attempts))
	mux.HandleFunc("POST /api/auth/forgot-password", auth.ForgotPasswordHandler(authStore, mail, attempts))
	mux.HandleFunc("POST /api/auth/reset-password", auth.ResetPasswordHandler(authStore))
	mux.HandleFunc("GET /api/team/accounts", requireAuth(requireRole(auth.RoleOwner)(auth.TeamAccountsHandler(authStore))))
	mux.HandleFunc("GET /api/team/invites", requireAuth(requireRole(auth.RoleOwner)(auth.TeamInvitesHandler(authStore))))
	mux.HandleFunc("POST /api/team/invites", requireAuth(requireRole(auth.RoleOwner)(auth.InviteHandler(authStore, mail))))
	mux.HandleFunc("PATCH /api/team/accounts/{id}", requireAuth(requireRole(auth.RoleOwner)(auth.UpdateAccountHandler(authStore))))
	mux.HandleFunc("DELETE /api/team/accounts/{id}", requireAuth(requireRole(auth.RoleOwner)(auth.DeleteAccountHandler(authStore))))
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("GET /api/calls", requireAuth(api.Calls(callsStore)))
	mux.HandleFunc("GET /api/calls/live", requireAuth(api.LiveCalls(callsStore)))
	mux.HandleFunc("POST /api/calls/{id}/label", requireAuth(requireRole(auth.RoleOwner, auth.RoleManager)(api.Label(callsStore))))
	mux.HandleFunc("GET /api/calls/{id}/recording", requireAuth(recordingAPI.Get()))
	mux.HandleFunc("DELETE /api/calls/{id}/recording", requireAuth(recordingAPI.Delete()))
	mux.HandleFunc("GET /api/calls/{id}/supervisor-token", requireAuth(api.SupervisorToken(callsStore)))
	mux.HandleFunc("GET /api/report", requireAuth(api.Report(callsStore)))
	mux.HandleFunc("GET /api/tenants", requireAuth(api.Tenants(tenants)))
	mux.HandleFunc("GET /api/geocode", api.Geocode())
	mux.HandleFunc("GET /api/staff", requireAuth(api.Staff(jobsReg)))
	mux.HandleFunc("POST /api/staff", requireAuth(requireRole(auth.RoleOwner, auth.RoleManager)(api.Staff(jobsReg))))
	mux.HandleFunc("PUT /api/staff/{id}", requireAuth(requireRole(auth.RoleOwner, auth.RoleManager)(api.Staff(jobsReg))))
	mux.HandleFunc("PATCH /api/staff/{id}", requireAuth(requireRole(auth.RoleOwner, auth.RoleManager)(api.Staff(jobsReg))))
	mux.HandleFunc("GET /api/livekit/token", requireAuth(api.LiveKitToken(tenants)))
	mux.HandleFunc("POST /internal/agent/session", tools.AgentSession())
	mux.HandleFunc("POST /internal/livekit/webhook", api.LiveKitWebhook(callsStore))
	mux.HandleFunc("POST /internal/tools/availability", requireAgent(tools.Availability()))
	mux.HandleFunc("POST /internal/tools/save-address", requireAgent(tools.SaveAddress()))
	mux.HandleFunc("POST /internal/tools/save-phone", requireAgent(tools.SavePhone()))
	mux.HandleFunc("POST /internal/tools/confirm-booking", requireAgent(tools.ConfirmBooking()))
	mux.HandleFunc("POST /internal/tools/book", requireAgent(tools.Book()))
	mux.HandleFunc("POST /internal/tools/reschedule", requireAgent(tools.Reschedule()))
	mux.HandleFunc("POST /internal/tools/cancel", requireAgent(tools.Cancel()))
	mux.HandleFunc("POST /internal/tools/lookup", requireAgent(tools.Lookup()))
	mux.HandleFunc("POST /internal/tools/policy", requireAgent(tools.Policy()))
	mux.HandleFunc("POST /internal/tools/consent", requireAgent(tools.Consent()))
	mux.HandleFunc("POST /internal/tools/turn", requireAgent(tools.Turn()))
	mux.HandleFunc("POST /internal/tools/recording/started", requireAgent(tools.RecordingStarted()))
	mux.HandleFunc("POST /internal/tools/recording/stopped", requireAgent(tools.RecordingStopped()))
	mux.HandleFunc("POST /internal/metrics/agent", requireAgent(tools.AgentMetrics()))
	mux.HandleFunc("POST /internal/tools/usage", requireAgent(tools.Usage()))
	mux.HandleFunc("POST /internal/tools/escalate", requireAgent(tools.Escalate()))
	mux.HandleFunc("POST /internal/tools/end", requireAgent(tools.End()))
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

	srv := &http.Server{Addr: *addr, Handler: withCORS(withOriginGuard(mux, origins), origins)}

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

func envInt64(name string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return n
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
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

func withOriginGuard(next http.Handler, origins []string) http.Handler {
	allowed := map[string]bool{}
	for _, origin := range origins {
		origin = strings.TrimSpace(strings.TrimRight(origin, "/"))
		if origin != "" {
			allowed[origin] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			// Agent and LiveKit webhook calls are server-to-server and carry
			// their own authentication.
			if strings.HasPrefix(r.URL.Path, "/internal/") {
				next.ServeHTTP(w, r)
				return
			}
			origin := strings.TrimRight(r.Header.Get("Origin"), "/")
			if origin == "" {
				if ref, err := url.Parse(r.Header.Get("Referer")); err == nil && ref.Host != "" {
					origin = ref.Scheme + "://" + ref.Host
				}
			}
			if origin != "" && !allowed[origin] {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
