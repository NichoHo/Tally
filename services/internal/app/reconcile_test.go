package app

import (
	"testing"

	ledgerpb "github.com/nickho/tally/proto"
)

func acct(id, balance int64) *ledgerpb.Account {
	return &ledgerpb.Account{Id: id, Name: "acct", Currency: "USD", BalanceMinor: balance}
}

func entry(direction string, amount int64) *ledgerpb.LedgerEntry {
	return &ledgerpb.LedgerEntry{Direction: direction, AmountMinor: amount}
}

// A 1000 transfer debits the source (balance -1000) and credits the dest
// (balance +1000). Recomputed balances match the cache and the system sums to 0.
func TestSummarizeReconcile_Balanced(t *testing.T) {
	accounts := []*ledgerpb.Account{acct(1, -1000), acct(2, 1000)}
	entries := map[int64][]*ledgerpb.LedgerEntry{
		1: {entry("debit", 1000)},
		2: {entry("credit", 1000)},
	}
	v := summarizeReconcile(accounts, entries)
	if !v.OK {
		t.Fatalf("expected reconcile ok, got %+v", v)
	}
	if v.SystemSumMinor != 0 {
		t.Errorf("system sum = %d, want 0", v.SystemSumMinor)
	}
	if v.MismatchCount != 0 {
		t.Errorf("mismatch count = %d, want 0", v.MismatchCount)
	}
}

// If a cached balance drifts from the ledger entries, reconcile must catch it:
// the system still sums to zero, but the account is flagged and OK is false.
func TestSummarizeReconcile_DetectsDrift(t *testing.T) {
	accounts := []*ledgerpb.Account{acct(1, -999), acct(2, 1000)} // acct 1 cache is wrong
	entries := map[int64][]*ledgerpb.LedgerEntry{
		1: {entry("debit", 1000)},
		2: {entry("credit", 1000)},
	}
	v := summarizeReconcile(accounts, entries)
	if v.OK {
		t.Fatalf("expected reconcile to fail on drift, got ok: %+v", v)
	}
	if v.MismatchCount != 1 {
		t.Errorf("mismatch count = %d, want 1", v.MismatchCount)
	}
	if v.Accounts[0].OK {
		t.Errorf("account 1 should be flagged as a mismatch")
	}
	if !v.Accounts[1].OK {
		t.Errorf("account 2 reconciles and should not be flagged")
	}
}
