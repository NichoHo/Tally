// Command renderall runs the ledger and gateway in a single process, wired
// together over a loopback TCP connection instead of a real network hop.
//
// It exists only for the free Render deployment. Render's free plan does not
// resolve private short hostnames between two web services, and gRPC over its
// public onrender.com edge does not work without a custom domain, so two
// separate free services cannot reach each other over gRPC at all. Running
// both in one process sidesteps the problem entirely: the "real" two-service
// architecture (services/ledger, services/gateway) still exists and is what
// docker-compose and the k8s manifests run.
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	ledgerpb "github.com/nickho/tally/proto"
	"github.com/nickho/tally/services/internal/app"
	"github.com/nickho/tally/services/ledger/internal/grpcserver"
	"github.com/nickho/tally/services/ledger/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// loopbackAddr is never exposed outside the container; it only connects the
// gateway's gRPC client to the ledger's gRPC server within this process.
const loopbackAddr = "127.0.0.1:9090"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	pool, err := connectWithRetry(context.Background(), dsn, log)
	if err != nil {
		log.Error("connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	lis, err := net.Listen("tcp", loopbackAddr)
	if err != nil {
		log.Error("listen", "addr", loopbackAddr, "error", err)
		os.Exit(1)
	}
	grpcSrv := grpc.NewServer()
	// No Kafka publisher: the free deploy has no always-on broker, the
	// gateway nudges the fraud service directly instead (see FRAUD_SCORE_URL).
	ledgerpb.RegisterLedgerServiceServer(grpcSrv, grpcserver.New(store.New(pool), nil))
	go func() {
		log.Info("ledger listening (loopback)", "addr", loopbackAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Error("ledger serve", "error", err)
			os.Exit(1)
		}
	}()

	conn, err := grpc.NewClient(loopbackAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("dial ledger", "addr", loopbackAddr, "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	handler := app.New(ledgerpb.NewLedgerServiceClient(conn), log, os.Getenv("FRAUD_SCORE_URL"))

	httpAddr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		httpAddr = ":" + p
	}
	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Info("gateway listening", "addr", httpAddr)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Error("serve", "error", err)
		os.Exit(1)
	}
}

// connectWithRetry waits for Postgres to accept connections, since the
// database may still be waking (e.g. Neon scale-to-zero) when this boots.
func connectWithRetry(ctx context.Context, dsn string, log *slog.Logger) (*pgxpool.Pool, error) {
	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err
		log.Info("waiting for postgres", "attempt", attempt)
		time.Sleep(time.Second)
	}
	return nil, lastErr
}
