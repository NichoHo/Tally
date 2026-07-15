// Package store is the database layer for the ledger. It owns the pgx pool and
// the SQL. The one function that matters most is CreateTransfer (transfer.go),
// which runs the whole money movement in a single transaction.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nickho/tally/services/ledger/internal/domain"
)

// Store wraps a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by the given pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// querier is satisfied by both *pgxpool.Pool and pgx.Tx, so read helpers can run
// either on their own connection or inside an open transaction.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

const accountColumns = `id, name, currency, balance_minor, allow_negative, created_at, updated_at`

func scanAccount(row pgx.Row) (domain.Account, error) {
	var a domain.Account
	err := row.Scan(&a.ID, &a.Name, &a.Currency, &a.BalanceMinor, &a.AllowNegative, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// CreateAccount opens a new account with a zero balance.
func (s *Store) CreateAccount(ctx context.Context, p domain.CreateAccountParams) (*domain.Account, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO accounts (name, currency, allow_negative) VALUES ($1, $2, $3)
		 RETURNING `+accountColumns,
		p.Name, p.Currency, p.AllowNegative)
	a, err := scanAccount(row)
	if err != nil {
		return nil, fmt.Errorf("insert account: %w", err)
	}
	return &a, nil
}

// GetAccount returns one account by id.
func (s *Store) GetAccount(ctx context.Context, id int64) (*domain.Account, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+accountColumns+` FROM accounts WHERE id = $1`, id)
	a, err := scanAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	return &a, nil
}

// ListAccounts returns all accounts, newest first.
func (s *Store) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+accountColumns+` FROM accounts ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var out []domain.Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounts: %w", err)
	}
	return out, nil
}

// ListAccountEntries returns the ledger entries for one account, newest first.
func (s *Store) ListAccountEntries(ctx context.Context, accountID int64) ([]domain.LedgerEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, transfer_id, account_id, direction, amount_minor, created_at
		 FROM ledger_entries WHERE account_id = $1 ORDER BY id DESC`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list account entries: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

func scanEntries(rows pgx.Rows) ([]domain.LedgerEntry, error) {
	var out []domain.LedgerEntry
	for rows.Next() {
		var e domain.LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransferID, &e.AccountID, &e.Direction, &e.AmountMinor, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ledger entries: %w", err)
	}
	return out, nil
}
