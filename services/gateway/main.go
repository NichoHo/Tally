// Command gateway is the thin REST front door. It validates nothing beyond
// shape, forwards to the ledger service over gRPC, and translates gRPC status
// codes into HTTP status codes. All money fields are integer minor units.
package main

import (
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nickho/tally/services/gateway/internal/app"
	ledgerpb "github.com/nickho/tally/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

	// Render's free plan does not resolve private short hostnames for web
	// services, so a standalone free deploy would need LEDGER_GRPC_ADDR
	// pointed at the ledger's public onrender.com hostname, which needs TLS.
	// (The actual free Render deploy avoids this by running the ledger
	// in-process instead; see services/renderall.) Local/compose/k8s targets
	// (bare host:port, no dot) stay plaintext.
	transportCreds := insecure.NewCredentials()
	if strings.Contains(ledgerAddr, ".") {
		transportCreds = credentials.NewTLS(&tls.Config{})
	}

	conn, err := grpc.NewClient(ledgerAddr, grpc.WithTransportCredentials(transportCreds))
	if err != nil {
		log.Error("dial ledger", "addr", ledgerAddr, "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	handler := app.New(
		ledgerpb.NewLedgerServiceClient(conn),
		log,
		// Free-tier deployment only: when set, the gateway nudges the fraud
		// service to score after each transfer, replacing the Kafka pipeline.
		// Empty (the default, e.g. local docker-compose) means the ledger's
		// Kafka publish drives scoring instead.
		os.Getenv("FRAUD_SCORE_URL"),
	)

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           handler,
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
