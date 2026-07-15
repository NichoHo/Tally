// Package domain holds the money-movement rules for the ledger. It is pure Go:
// no database, no transport. The rules here are what section 6 of CLAUDE.md
// calls the heart of the project.
package domain

import "time"

// Account is a single balance in one currency. balance_minor is a cached value
// kept in sync inside the transfer transaction; the ledger entries are the
// source of truth (see RecomputeBalance in the store).
type Account struct {
	ID            int64
	Name          string
	Currency      string
	BalanceMinor  int64
	AllowNegative bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Direction is the side of a double-entry record. A debit reduces an account's
// balance, a credit increases it.
type Direction string

const (
	Debit  Direction = "debit"
	Credit Direction = "credit"
)

// LedgerEntry is one side of a double-entry pair. amount_minor is always
// positive; direction says whether it adds or removes money.
type LedgerEntry struct {
	ID          int64
	TransferID  int64
	AccountID   int64
	Direction   Direction
	AmountMinor int64
	CreatedAt   time.Time
}

// Transfer is one money movement. It always has exactly two ledger entries once
// completed: a debit on the source and a credit on the destination.
type Transfer struct {
	ID              int64
	SourceAccountID int64
	DestAccountID   int64
	AmountMinor     int64
	Currency        string
	Status          string
	CreatedAt       time.Time
	Entries         []LedgerEntry
	FraudScore      *FraudScore // nil until the fraud service has scored it
}

// FraudScore is the fraud service's verdict on one transfer. Score is kept as
// the database's exact decimal text (for example "0.870"), never a float, so
// display and comparison never drift.
type FraudScore struct {
	TransferID   int64
	Score        string
	Decision     string
	ModelVersion string
	CreatedAt    time.Time
}

// TransferParams is a validated request to move money.
type TransferParams struct {
	SourceAccountID int64
	DestAccountID   int64
	AmountMinor     int64
	Currency        string
}

// CreateAccountParams is a request to open a new account.
type CreateAccountParams struct {
	Name          string
	Currency      string
	AllowNegative bool
}
