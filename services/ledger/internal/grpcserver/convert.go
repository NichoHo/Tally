package grpcserver

import (
	"time"

	ledgerpb "github.com/nickho/tally/proto"
	"github.com/nickho/tally/services/ledger/internal/domain"
)

func fmtTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func toProtoAccount(a *domain.Account) *ledgerpb.Account {
	return &ledgerpb.Account{
		Id:            a.ID,
		Name:          a.Name,
		Currency:      a.Currency,
		BalanceMinor:  a.BalanceMinor,
		AllowNegative: a.AllowNegative,
		CreatedAt:     fmtTime(a.CreatedAt),
		UpdatedAt:     fmtTime(a.UpdatedAt),
	}
}

func toProtoEntry(e domain.LedgerEntry) *ledgerpb.LedgerEntry {
	return &ledgerpb.LedgerEntry{
		Id:          e.ID,
		TransferId:  e.TransferID,
		AccountId:   e.AccountID,
		Direction:   string(e.Direction),
		AmountMinor: e.AmountMinor,
		CreatedAt:   fmtTime(e.CreatedAt),
	}
}

func toProtoTransfer(t *domain.Transfer) *ledgerpb.Transfer {
	pt := &ledgerpb.Transfer{
		Id:              t.ID,
		SourceAccountId: t.SourceAccountID,
		DestAccountId:   t.DestAccountID,
		AmountMinor:     t.AmountMinor,
		Currency:        t.Currency,
		Status:          t.Status,
		CreatedAt:       fmtTime(t.CreatedAt),
	}
	for _, e := range t.Entries {
		pt.Entries = append(pt.Entries, toProtoEntry(e))
	}
	if f := t.FraudScore; f != nil {
		pt.FraudScore = &ledgerpb.FraudScore{
			TransferId:   f.TransferID,
			Score:        f.Score,
			Decision:     f.Decision,
			ModelVersion: f.ModelVersion,
			CreatedAt:    fmtTime(f.CreatedAt),
		}
	}
	return pt
}
