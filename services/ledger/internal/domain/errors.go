package domain

import "errors"

// Sentinel errors for the money paths. The gRPC layer maps each to a status
// code, and the gateway maps that to an HTTP code (400/404/409/422). Keeping
// them here means the rules and their meanings live in one place.
var (
	// Bad input from the client (maps to 400).
	ErrInvalidAmount         = errors.New("amount must be a positive integer of minor units")
	ErrSameAccount           = errors.New("source and destination accounts must differ")
	ErrInvalidCurrency       = errors.New("currency must be a 3 letter ISO code")
	ErrInvalidName           = errors.New("account name must not be empty")
	ErrMissingIdempotencyKey = errors.New("an idempotency key is required")

	// Not found (maps to 404).
	ErrAccountNotFound  = errors.New("account not found")
	ErrTransferNotFound = errors.New("transfer not found")

	// Business rule rejections (maps to 422).
	ErrCurrencyMismatch  = errors.New("account currencies do not match the transfer")
	ErrInsufficientFunds = errors.New("insufficient funds")

	// Idempotency conflict: same key, different request (maps to 409).
	ErrIdempotencyConflict = errors.New("idempotency key reused with a different request")
)
