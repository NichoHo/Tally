// Command ledger runs the core ledger gRPC service.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"time"

	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	ledgerpb "github.com/nickho/tally/proto"
	"github.com/nickho/tally/services/ledger/internal/events"
	"github.com/nickho/tally/services/ledger/internal/grpcserver"
	"github.com/nickho/tally/services/ledger/internal/store"
	"google.golang.org/grpc"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	addr := envDefault("LEDGER_GRPC_ADDR", ":9090")
	// Render (and similar hosts) inject the port to listen on via PORT.
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	pool, err := connectWithRetry(context.Background(), dsn, log)
	if err != nil {
		log.Error("connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Kafka is optional: without KAFKA_BROKERS the ledger still works, it just
	// does not announce transfers.
	var publisher *events.Publisher
	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		publisher, err = events.New(strings.Split(brokers, ","), log)
		if err != nil {
			log.Error("connect to kafka", "brokers", brokers, "error", err)
			os.Exit(1)
		}
		defer publisher.Close()
		log.Info("kafka publisher enabled", "brokers", brokers)
	} else {
		log.Info("KAFKA_BROKERS not set; event publishing disabled")
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("listen", "addr", addr, "error", err)
		os.Exit(1)
	}

	srv := grpc.NewServer()
	ledgerpb.RegisterLedgerServiceServer(srv, grpcserver.New(store.New(pool), publisher))

	log.Info("ledger service listening", "addr", addr)
	if err := srv.Serve(lis); err != nil {
		log.Error("serve", "error", err)
		os.Exit(1)
	}
}

// connectWithRetry waits for Postgres to accept connections, since the database
// container may still be starting when the service boots.
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

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
