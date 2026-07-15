package store

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/nickho/tally/services/ledger/internal/domain"
)

// A completed transfer moves the amount, writes exactly two entries (a debit and
// a credit that both equal the amount), and keeps the cached balances correct.
func TestCreateTransfer_MovesMoneyAndDoubleEntry(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	src := mustAccount(t, s, "alice", "USD", 5000)
	dst := mustAccount(t, s, "bob", "USD", 0)

	tr, err := s.CreateTransfer(ctx, domain.TransferParams{
		SourceAccountID: src.ID, DestAccountID: dst.ID, AmountMinor: 1500, Currency: "USD",
	}, randKey())
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	// Per-transfer invariant: one debit and one credit, both equal to amount.
	if len(tr.Entries) != 2 {
		t.Fatalf("want 2 ledger entries, got %d", len(tr.Entries))
	}
	var debits, credits int64
	for _, e := range tr.Entries {
		if e.AmountMinor != 1500 {
			t.Errorf("entry amount = %d, want 1500", e.AmountMinor)
		}
		switch e.Direction {
		case domain.Debit:
			debits += e.AmountMinor
			if e.AccountID != src.ID {
				t.Errorf("debit on account %d, want source %d", e.AccountID, src.ID)
			}
		case domain.Credit:
			credits += e.AmountMinor
			if e.AccountID != dst.ID {
				t.Errorf("credit on account %d, want dest %d", e.AccountID, dst.ID)
			}
		}
	}
	if debits != credits || debits != 1500 {
		t.Fatalf("debits=%d credits=%d, want both 1500", debits, credits)
	}

	// Cached balances updated.
	assertBalance(t, s, src.ID, 3500)
	assertBalance(t, s, dst.ID, 1500)
}

func TestCreateTransfer_InsufficientFunds(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	src := mustAccount(t, s, "alice", "USD", 100)
	dst := mustAccount(t, s, "bob", "USD", 0)

	_, err := s.CreateTransfer(ctx, domain.TransferParams{
		SourceAccountID: src.ID, DestAccountID: dst.ID, AmountMinor: 500, Currency: "USD",
	}, randKey())
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("err = %v, want ErrInsufficientFunds", err)
	}
	// Nothing moved.
	assertBalance(t, s, src.ID, 100)
	assertBalance(t, s, dst.ID, 0)
}

func TestCreateTransfer_CurrencyMismatch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	src := mustAccount(t, s, "alice", "USD", 5000)
	dst := mustAccount(t, s, "bob", "EUR", 0)

	_, err := s.CreateTransfer(ctx, domain.TransferParams{
		SourceAccountID: src.ID, DestAccountID: dst.ID, AmountMinor: 100, Currency: "USD",
	}, randKey())
	if !errors.Is(err, domain.ErrCurrencyMismatch) {
		t.Fatalf("err = %v, want ErrCurrencyMismatch", err)
	}
}

func TestCreateTransfer_AccountNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	src := mustAccount(t, s, "alice", "USD", 5000)

	_, err := s.CreateTransfer(ctx, domain.TransferParams{
		SourceAccountID: src.ID, DestAccountID: 999999, AmountMinor: 100, Currency: "USD",
	}, randKey())
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

// Sending the same idempotency key twice must move money exactly once and return
// the same transfer both times.
func TestIdempotency_DuplicateKeyMovesMoneyOnce(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	src := mustAccount(t, s, "alice", "USD", 5000)
	dst := mustAccount(t, s, "bob", "USD", 0)

	p := domain.TransferParams{SourceAccountID: src.ID, DestAccountID: dst.ID, AmountMinor: 1000, Currency: "USD"}
	key := randKey()

	first, err := s.CreateTransfer(ctx, p, key)
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	second, err := s.CreateTransfer(ctx, p, key)
	if err != nil {
		t.Fatalf("second transfer (replay): %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("replay returned different transfer id: %d vs %d", first.ID, second.ID)
	}
	// Money moved only once.
	assertBalance(t, s, src.ID, 4000)
	assertBalance(t, s, dst.ID, 1000)
	// Exactly one non-funding transfer between alice and bob (plus the funding one).
	assertSystemSumZero(t, s)
}

// The same key with a different request body is a client bug and must conflict.
func TestIdempotency_SameKeyDifferentRequestConflicts(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	src := mustAccount(t, s, "alice", "USD", 5000)
	dst := mustAccount(t, s, "bob", "USD", 0)
	key := randKey()

	_, err := s.CreateTransfer(ctx, domain.TransferParams{
		SourceAccountID: src.ID, DestAccountID: dst.ID, AmountMinor: 1000, Currency: "USD",
	}, key)
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	_, err = s.CreateTransfer(ctx, domain.TransferParams{
		SourceAccountID: src.ID, DestAccountID: dst.ID, AmountMinor: 2000, Currency: "USD", // different amount
	}, key)
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("err = %v, want ErrIdempotencyConflict", err)
	}
	// The differing request moved no extra money.
	assertBalance(t, s, src.ID, 4000)
}

// Many transfers running at once must never lose an update or corrupt balances.
func TestConcurrentTransfers_NoLostUpdates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	src := mustAccount(t, s, "alice", "USD", 100000)
	dst := mustAccount(t, s, "bob", "USD", 0)

	const n = 50
	const amount = 100
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.CreateTransfer(ctx, domain.TransferParams{
				SourceAccountID: src.ID, DestAccountID: dst.ID, AmountMinor: amount, Currency: "USD",
			}, randKey())
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent transfer failed: %v", err)
		}
	}

	assertBalance(t, s, src.ID, 100000-n*amount)
	assertBalance(t, s, dst.ID, n*amount)
	assertSystemSumZero(t, s)
	assertCachedMatchesLedger(t, s, src.ID)
	assertCachedMatchesLedger(t, s, dst.ID)
}

// Many goroutines racing with the SAME key must produce exactly one transfer.
func TestConcurrentSameKey_ExactlyOneTransfer(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	src := mustAccount(t, s, "alice", "USD", 100000)
	dst := mustAccount(t, s, "bob", "USD", 0)

	before, err := s.CountTransfers(ctx)
	if err != nil {
		t.Fatalf("count transfers: %v", err)
	}

	p := domain.TransferParams{SourceAccountID: src.ID, DestAccountID: dst.ID, AmountMinor: 700, Currency: "USD"}
	key := randKey()

	const n = 20
	var wg sync.WaitGroup
	ids := make(chan int64, n)
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr, err := s.CreateTransfer(ctx, p, key)
			if err != nil {
				errCh <- err
				return
			}
			ids <- tr.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errCh)
	for err := range errCh {
		t.Fatalf("same-key transfer errored: %v", err)
	}

	// Every caller got the same transfer id.
	seen := map[int64]bool{}
	for id := range ids {
		seen[id] = true
	}
	if len(seen) != 1 {
		t.Fatalf("expected 1 distinct transfer id, got %d", len(seen))
	}

	// Exactly one new transfer row was created.
	after, err := s.CountTransfers(ctx)
	if err != nil {
		t.Fatalf("count transfers: %v", err)
	}
	if after-before != 1 {
		t.Fatalf("expected exactly 1 new transfer, got %d", after-before)
	}
	// Money moved once.
	assertBalance(t, s, src.ID, 100000-700)
	assertBalance(t, s, dst.ID, 700)
}

// ---- assertion helpers ----

func assertBalance(t *testing.T, s *Store, accountID, want int64) {
	t.Helper()
	a, err := s.GetAccount(context.Background(), accountID)
	if err != nil {
		t.Fatalf("get account %d: %v", accountID, err)
	}
	if a.BalanceMinor != want {
		t.Fatalf("account %d balance = %d, want %d", accountID, a.BalanceMinor, want)
	}
}

func assertCachedMatchesLedger(t *testing.T, s *Store, accountID int64) {
	t.Helper()
	ctx := context.Background()
	recomputed, err := s.RecomputeBalance(ctx, accountID)
	if err != nil {
		t.Fatalf("recompute balance: %v", err)
	}
	a, err := s.GetAccount(ctx, accountID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if recomputed != a.BalanceMinor {
		t.Fatalf("account %d: cached %d != recomputed-from-entries %d", accountID, a.BalanceMinor, recomputed)
	}
}

func assertSystemSumZero(t *testing.T, s *Store) {
	t.Helper()
	sum, err := s.SystemLedgerSum(context.Background())
	if err != nil {
		t.Fatalf("system ledger sum: %v", err)
	}
	if sum != 0 {
		t.Fatalf("system-wide ledger sum = %d, want 0 (money must be conserved)", sum)
	}
}
