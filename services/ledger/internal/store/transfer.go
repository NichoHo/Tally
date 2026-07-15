package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nickho/tally/services/ledger/internal/domain"
)

// CreateTransfer moves money from source to destination. Everything happens in
// one database transaction: idempotency claim, account locks, the transfer row,
// the two ledger entries, the balance updates, and the idempotency result. If
// any step fails the whole thing rolls back, so money is never left half moved
// (section 6.2 of CLAUDE.md).
func (s *Store) CreateTransfer(ctx context.Context, p domain.TransferParams, idemKey string) (*domain.Transfer, error) {
	if idemKey == "" {
		return nil, domain.ErrMissingIdempotencyKey
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	fingerprint := p.Fingerprint()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transfer tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	// Claim the idempotency key first. The primary key on idempotency_keys.key
	// serializes concurrent requests that share a key: the second inserter
	// blocks on the row lock until the first transaction ends, then sees the
	// committed row via ON CONFLICT and replays its result. This is what makes
	// duplicate sends move money exactly once.
	var claimedKey string
	err = tx.QueryRow(ctx,
		`INSERT INTO idempotency_keys (key, request_hash) VALUES ($1, $2)
		 ON CONFLICT (key) DO NOTHING RETURNING key`,
		idemKey, fingerprint).Scan(&claimedKey)
	if errors.Is(err, pgx.ErrNoRows) {
		// Key already existed (and its owning transaction committed). Either
		// replay the stored transfer or report a conflict.
		return replayTransfer(ctx, tx, idemKey, fingerprint)
	}
	if err != nil {
		return nil, fmt.Errorf("claim idempotency key: %w", err)
	}

	// We own the key. Lock both accounts in ascending id order to avoid
	// deadlocks when two transfers touch the same pair from opposite sides.
	accounts, err := lockAccounts(ctx, tx, p.SourceAccountID, p.DestAccountID)
	if err != nil {
		return nil, err
	}
	src := accounts[p.SourceAccountID]
	dst := accounts[p.DestAccountID]

	if src.Currency != p.Currency || dst.Currency != p.Currency {
		return nil, domain.ErrCurrencyMismatch
	}
	if !src.AllowNegative && src.BalanceMinor < p.AmountMinor {
		return nil, domain.ErrInsufficientFunds
	}

	t := domain.Transfer{
		SourceAccountID: p.SourceAccountID,
		DestAccountID:   p.DestAccountID,
		AmountMinor:     p.AmountMinor,
		Currency:        p.Currency,
		Status:          "completed",
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO transfers (source_account_id, dest_account_id, amount_minor, currency, status)
		 VALUES ($1, $2, $3, $4, 'completed') RETURNING id, created_at`,
		p.SourceAccountID, p.DestAccountID, p.AmountMinor, p.Currency,
	).Scan(&t.ID, &t.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert transfer: %w", err)
	}

	// Double entry: debit the source, credit the destination, same amount.
	debit, err := insertEntry(ctx, tx, t.ID, p.SourceAccountID, domain.Debit, p.AmountMinor)
	if err != nil {
		return nil, err
	}
	credit, err := insertEntry(ctx, tx, t.ID, p.DestAccountID, domain.Credit, p.AmountMinor)
	if err != nil {
		return nil, err
	}
	t.Entries = []domain.LedgerEntry{debit, credit}

	// Keep the cached balances in sync with the entries.
	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance_minor = balance_minor - $1, updated_at = now() WHERE id = $2`,
		p.AmountMinor, p.SourceAccountID); err != nil {
		return nil, fmt.Errorf("debit source balance: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance_minor = balance_minor + $1, updated_at = now() WHERE id = $2`,
		p.AmountMinor, p.DestAccountID); err != nil {
		return nil, fmt.Errorf("credit destination balance: %w", err)
	}

	// Record which transfer this key produced, so replays can return it.
	if _, err := tx.Exec(ctx,
		`UPDATE idempotency_keys SET transfer_id = $1 WHERE key = $2`, t.ID, idemKey); err != nil {
		return nil, fmt.Errorf("record idempotency result: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transfer: %w", err)
	}
	return &t, nil
}

// replayTransfer handles the case where the idempotency key already exists. Same
// request fingerprint means return the stored transfer; a different fingerprint
// is a client bug and becomes a conflict.
func replayTransfer(ctx context.Context, tx pgx.Tx, key, fingerprint string) (*domain.Transfer, error) {
	var storedHash string
	var transferID *int64
	if err := tx.QueryRow(ctx,
		`SELECT request_hash, transfer_id FROM idempotency_keys WHERE key = $1`, key,
	).Scan(&storedHash, &transferID); err != nil {
		return nil, fmt.Errorf("load idempotency record: %w", err)
	}
	if storedHash != fingerprint {
		return nil, domain.ErrIdempotencyConflict
	}
	if transferID == nil {
		// The owning transaction committed the key but not a transfer id. This
		// should be impossible given both happen in the same transaction.
		return nil, fmt.Errorf("idempotency key %q has no recorded transfer", key)
	}
	return loadTransfer(ctx, tx, *transferID)
}

// lockAccounts selects the given accounts FOR UPDATE, ordered by id ascending so
// locks are always taken in the same order. Returns ErrAccountNotFound if any id
// is missing.
func lockAccounts(ctx context.Context, tx pgx.Tx, ids ...int64) (map[int64]domain.Account, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+accountColumns+` FROM accounts WHERE id = ANY($1) ORDER BY id ASC FOR UPDATE`, ids)
	if err != nil {
		return nil, fmt.Errorf("lock accounts: %w", err)
	}
	defer rows.Close()

	out := make(map[int64]domain.Account, len(ids))
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("scan locked account: %w", err)
		}
		out[a.ID] = a
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate locked accounts: %w", err)
	}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			return nil, domain.ErrAccountNotFound
		}
	}
	return out, nil
}

func insertEntry(ctx context.Context, tx pgx.Tx, transferID, accountID int64, dir domain.Direction, amount int64) (domain.LedgerEntry, error) {
	e := domain.LedgerEntry{TransferID: transferID, AccountID: accountID, Direction: dir, AmountMinor: amount}
	if err := tx.QueryRow(ctx,
		`INSERT INTO ledger_entries (transfer_id, account_id, direction, amount_minor)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		transferID, accountID, string(dir), amount,
	).Scan(&e.ID, &e.CreatedAt); err != nil {
		return e, fmt.Errorf("insert %s entry: %w", dir, err)
	}
	return e, nil
}

// GetTransfer returns one transfer with its two ledger entries.
func (s *Store) GetTransfer(ctx context.Context, id int64) (*domain.Transfer, error) {
	return loadTransfer(ctx, s.pool, id)
}

func loadTransfer(ctx context.Context, q querier, id int64) (*domain.Transfer, error) {
	var t domain.Transfer
	err := q.QueryRow(ctx,
		`SELECT id, source_account_id, dest_account_id, amount_minor, currency, status, created_at
		 FROM transfers WHERE id = $1`, id,
	).Scan(&t.ID, &t.SourceAccountID, &t.DestAccountID, &t.AmountMinor, &t.Currency, &t.Status, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTransferNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load transfer: %w", err)
	}

	rows, err := q.Query(ctx,
		`SELECT id, transfer_id, account_id, direction, amount_minor, created_at
		 FROM ledger_entries WHERE transfer_id = $1 ORDER BY id ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("load transfer entries: %w", err)
	}
	defer rows.Close()
	entries, err := scanEntries(rows)
	if err != nil {
		return nil, err
	}
	t.Entries = entries

	if t.FraudScore, err = fraudScore(ctx, q, id); err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTransfers returns transfers newest first, using keyset pagination. Pass
// beforeID = 0 for the first page. Entries are not loaded for list views.
func (s *Store) ListTransfers(ctx context.Context, limit int, beforeID int64) ([]domain.Transfer, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.source_account_id, t.dest_account_id, t.amount_minor, t.currency,
		        t.status, t.created_at,
		        f.score::text, f.decision, f.model_version, f.created_at
		 FROM transfers t
		 LEFT JOIN fraud_scores f ON f.transfer_id = t.id
		 WHERE ($1 = 0 OR t.id < $1) ORDER BY t.id DESC LIMIT $2`, beforeID, limit)
	if err != nil {
		return nil, fmt.Errorf("list transfers: %w", err)
	}
	defer rows.Close()

	var out []domain.Transfer
	for rows.Next() {
		var t domain.Transfer
		var score, decision, version *string
		var scoredAt *time.Time
		if err := rows.Scan(&t.ID, &t.SourceAccountID, &t.DestAccountID, &t.AmountMinor, &t.Currency,
			&t.Status, &t.CreatedAt, &score, &decision, &version, &scoredAt); err != nil {
			return nil, fmt.Errorf("scan transfer: %w", err)
		}
		if score != nil && decision != nil && version != nil && scoredAt != nil {
			t.FraudScore = &domain.FraudScore{
				TransferID: t.ID, Score: *score, Decision: *decision,
				ModelVersion: *version, CreatedAt: *scoredAt,
			}
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transfers: %w", err)
	}
	return out, nil
}
