package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nickho/tally/services/ledger/internal/domain"
)

// randKey returns a unique idempotency key for tests that need distinct keys.
func randKey() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// newTestStore returns a Store backed by a fresh schema. It reads
// TEST_DATABASE_URL and skips the test if it is not set, so unit-only runs still
// work without a database.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	resetSchema(t, pool)
	return New(pool)
}

// resetSchema drops and recreates the tables so each test starts clean. It runs
// the same migration SQL the app uses.
func resetSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	down := readFile(t, "../../migrations/000001_init.down.sql")
	up := readFile(t, "../../migrations/000001_init.up.sql")
	if _, err := pool.Exec(ctx, down); err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("create schema: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// mustAccount creates an account with the given starting balance by opening it
// and, if needed, funding it from a genesis account that is allowed to go
// negative. This keeps the system-wide sum at zero while still letting tests set
// up balances.
func mustAccount(t *testing.T, s *Store, name, currency string, startingMinor int64) *domain.Account {
	t.Helper()
	ctx := context.Background()
	a, err := s.CreateAccount(ctx, domain.CreateAccountParams{Name: name, Currency: currency})
	if err != nil {
		t.Fatalf("create account %s: %v", name, err)
	}
	if startingMinor > 0 {
		fundAccount(t, s, a.ID, currency, startingMinor)
		// Refresh the cached balance.
		a, err = s.GetAccount(ctx, a.ID)
		if err != nil {
			t.Fatalf("reload account %s: %v", name, err)
		}
	}
	return a
}

// fundAccount moves money into an account from a genesis account that may go
// negative, so money is still conserved (the genesis balance goes negative by
// the same amount).
func fundAccount(t *testing.T, s *Store, accountID int64, currency string, amount int64) {
	t.Helper()
	ctx := context.Background()
	genesis, err := s.CreateAccount(ctx, domain.CreateAccountParams{
		Name: "genesis", Currency: currency, AllowNegative: true,
	})
	if err != nil {
		t.Fatalf("create genesis: %v", err)
	}
	_, err = s.CreateTransfer(ctx, domain.TransferParams{
		SourceAccountID: genesis.ID, DestAccountID: accountID, AmountMinor: amount, Currency: currency,
	}, "fund-"+randKey())
	if err != nil {
		t.Fatalf("fund account: %v", err)
	}
}
