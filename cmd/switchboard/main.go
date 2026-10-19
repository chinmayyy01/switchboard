package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chinmay/switchboard/internal/api"
	"github.com/chinmay/switchboard/internal/hub"
	"github.com/chinmay/switchboard/internal/metrics"
	"github.com/chinmay/switchboard/internal/store"
	"github.com/chinmay/switchboard/internal/ws"
)

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	dataDir := flag.String("data", "data", "call-record data directory")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	h := hub.New()
	st := store.New(*dataDir, 500)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.Handler(h, st, ctx))
	mux.HandleFunc("/ws/supervisor", ws.SupervisorHandler(h, ctx))
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("GET /api/calls", api.Calls(st))
	mux.HandleFunc("POST /api/calls/{id}/label", api.Label(st))
	mux.HandleFunc("POST /api/calls/{id}/takeover", api.TakeOver(h))
	mux.HandleFunc("GET /api/report", api.Report(st))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: *addr, Handler: withCORS(mux)}

	go func() {
		<-ctx.Done()
		log.Printf("shutting down…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("switchboard realtime service on http://localhost%s", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
