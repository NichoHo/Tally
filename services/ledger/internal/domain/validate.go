package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Validate checks the request-level rules that do not need the database:
// amount positive, source != destination, currency well formed. Rules that
// need account state (currencies match, sufficient funds) are checked in the
// store inside the transaction, where the accounts are locked.
func (p TransferParams) Validate() error {
	if p.AmountMinor <= 0 {
		return ErrInvalidAmount
	}
	if p.SourceAccountID == p.DestAccountID {
		return ErrSameAccount
	}
	if len(p.Currency) != 3 {
		return ErrInvalidCurrency
	}
	return nil
}

// Fingerprint is a stable hash of the money-affecting fields of a request. Two
// requests sent with the same idempotency key must produce the same
// fingerprint; a different fingerprint for the same key is a client bug and is
// rejected as a conflict.
func (p TransferParams) Fingerprint() string {
	raw := fmt.Sprintf("%d:%d:%d:%s", p.SourceAccountID, p.DestAccountID, p.AmountMinor, p.Currency)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// Validate checks the rules for opening an account.
func (p CreateAccountParams) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return ErrInvalidName
	}
	if len(p.Currency) != 3 {
		return ErrInvalidCurrency
	}
	return nil
}
