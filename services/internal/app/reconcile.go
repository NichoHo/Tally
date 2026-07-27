package app

import (
	"net/http"

	ledgerpb "github.com/nickho/tally/proto"
)

// Reconciliation proves the ledger has not drifted. For every account it
// compares the cached balance_minor against the balance recomputed purely from
// that account's ledger entries (sum of credits minus debits), and it checks
// that every entry across the system sums to zero (money is conserved). This is
// the same property the invariant tests assert (CLAUDE.md section 6.3), exposed
// live so it can be shown on screen instead of only in a test run.

type reconcileAccountView struct {
	AccountID     int64  `json:"account_id"`
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	CachedMinor   int64  `json:"cached_minor"`
	ComputedMinor int64  `json:"computed_minor"`
	OK            bool   `json:"ok"`
}

type reconcileView struct {
	OK             bool                   `json:"ok"`
	AccountCount   int                    `json:"account_count"`
	MismatchCount  int                    `json:"mismatch_count"`
	SystemSumMinor int64                  `json:"system_sum_minor"`
	Accounts       []reconcileAccountView `json:"accounts"`
}

// summarizeReconcile compares each account's cached balance to the balance
// recomputed from its ledger entries and totals the system-wide signed sum. It
// is pure so it can be unit tested without a ledger backend. entriesByAccount
// maps an account id to that account's ledger entries.
func summarizeReconcile(accounts []*ledgerpb.Account, entriesByAccount map[int64][]*ledgerpb.LedgerEntry) reconcileView {
	v := reconcileView{
		AccountCount: len(accounts),
		Accounts:     make([]reconcileAccountView, 0, len(accounts)),
	}
	for _, a := range accounts {
		var computed int64
		for _, e := range entriesByAccount[a.GetId()] {
			// credits add, debits subtract (matches store.creditMinusDebit).
			if e.GetDirection() == "credit" {
				computed += e.GetAmountMinor()
			} else {
				computed -= e.GetAmountMinor()
			}
		}
		ok := computed == a.GetBalanceMinor()
		if !ok {
			v.MismatchCount++
		}
		v.SystemSumMinor += computed
		v.Accounts = append(v.Accounts, reconcileAccountView{
			AccountID:     a.GetId(),
			Name:          a.GetName(),
			Currency:      a.GetCurrency(),
			CachedMinor:   a.GetBalanceMinor(),
			ComputedMinor: computed,
			OK:            ok,
		})
	}
	v.OK = v.MismatchCount == 0 && v.SystemSumMinor == 0
	return v
}

// getReconcile recomputes balances from ledger entries and reports whether the
// cached balances still match and whether the system sums to zero.
func (s *server) getReconcile(w http.ResponseWriter, r *http.Request) {
	resp, err := s.ledger.ListAccounts(r.Context(), &ledgerpb.ListAccountsRequest{})
	if err != nil {
		s.fail(w, err)
		return
	}
	accounts := resp.GetAccounts()

	// ponytail: N+1 entry fetch, fine at demo scale. The clean version is a
	// single Reconcile gRPC RPC doing one SQL pass (the store already has
	// RecomputeBalance and SystemLedgerSum). Add it when account counts grow and
	// proto can be regenerated.
	entriesByAccount := make(map[int64][]*ledgerpb.LedgerEntry, len(accounts))
	for _, a := range accounts {
		er, err := s.ledger.ListAccountEntries(r.Context(), &ledgerpb.ListAccountEntriesRequest{AccountId: a.GetId()})
		if err != nil {
			s.fail(w, err)
			return
		}
		entriesByAccount[a.GetId()] = er.GetEntries()
	}

	writeJSON(w, http.StatusOK, summarizeReconcile(accounts, entriesByAccount))
}
