package store

import (
	"context"
	"fmt"
)

// These helpers treat the ledger entries as the source of truth. The tests use
// them to prove the cached balances never drift and that money is conserved.

// creditMinusDebit is the signed value of an entry: credits add, debits subtract.
const creditMinusDebit = `CASE direction WHEN 'credit' THEN amount_minor ELSE -amount_minor END`

// RecomputeBalance returns an account's balance computed purely from its ledger
// entries (sum of credits minus sum of debits). It must equal the cached
// balance_minor on the account.
func (s *Store) RecomputeBalance(ctx context.Context, accountID int64) (int64, error) {
	var balance int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(`+creditMinusDebit+`), 0) FROM ledger_entries WHERE account_id = $1`,
		accountID,
	).Scan(&balance); err != nil {
		return 0, fmt.Errorf("recompute balance: %w", err)
	}
	return balance, nil
}

// SystemLedgerSum returns the signed sum of every ledger entry across the whole
// system. It must always be exactly zero: money is conserved (section 6.3).
func (s *Store) SystemLedgerSum(ctx context.Context) (int64, error) {
	var sum int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(`+creditMinusDebit+`), 0) FROM ledger_entries`,
	).Scan(&sum); err != nil {
		return 0, fmt.Errorf("system ledger sum: %w", err)
	}
	return sum, nil
}

// CountTransfers returns the number of transfer rows. Used by the idempotency
// test to prove a duplicate key creates exactly one transfer.
func (s *Store) CountTransfers(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transfers`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count transfers: %w", err)
	}
	return n, nil
}
