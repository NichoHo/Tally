// Command gateway is the thin REST front door. It validates nothing beyond
// shape, forwards to the ledger service over gRPC, and translates gRPC status
// codes into HTTP status codes. All money fields are integer minor units.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	ledgerpb "github.com/nickho/tally/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ledgerAddr := envDefault("LEDGER_GRPC_ADDR", "ledger:9090")
	httpAddr := envDefault("GATEWAY_HTTP_ADDR", ":8080")
	// Render (and similar hosts) inject the port to listen on via PORT.
	if p := os.Getenv("PORT"); p != "" {
		httpAddr = ":" + p
	}

	conn, err := grpc.NewClient(ledgerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("dial ledger", "addr", ledgerAddr, "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	srv := &server{
		ledger: ledgerpb.NewLedgerServiceClient(conn),
		log:    log,
		// Free-tier deployment only: when set, the gateway nudges the fraud
		// service to score after each transfer, replacing the Kafka pipeline.
		// Empty (the default, e.g. local docker-compose) means the ledger's
		// Kafka publish drives scoring instead.
		fraudScoreURL: os.Getenv("FRAUD_SCORE_URL"),
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	r.Get("/healthz", srv.healthz)
	r.Get("/readyz", srv.readyz)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/accounts", srv.createAccount)
		r.Get("/accounts", srv.listAccounts)
		r.Get("/accounts/{id}", srv.getAccount)
		r.Get("/accounts/{id}/entries", srv.listAccountEntries)

		r.Post("/transfers", srv.createTransfer)
		r.Get("/transfers", srv.listTransfers)
		r.Get("/transfers/{id}", srv.getTransfer)

		r.Get("/fraud/flags", srv.listFraudFlags)
		r.Get("/stats", srv.getStats)
	})

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Info("gateway listening", "addr", httpAddr, "ledger", ledgerAddr)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Error("serve", "error", err)
		os.Exit(1)
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// readyz confirms the ledger service is reachable.
func (s *server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if _, err := s.ledger.ListAccounts(ctx, &ledgerpb.ListAccountsRequest{}); err != nil {
		writeError(w, http.StatusServiceUnavailable, "ledger not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
