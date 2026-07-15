package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/nickho/tally/services/ledger/internal/domain"
)

// fraudScore loads the fraud score for one transfer, or nil if it has not been
// scored yet. score is selected as text so the exact decimal survives.
func fraudScore(ctx context.Context, q querier, transferID int64) (*domain.FraudScore, error) {
	var f domain.FraudScore
	err := q.QueryRow(ctx,
		`SELECT transfer_id, score::text, decision, model_version, created_at
		 FROM fraud_scores WHERE transfer_id = $1`, transferID,
	).Scan(&f.TransferID, &f.Score, &f.Decision, &f.ModelVersion, &f.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load fraud score: %w", err)
	}
	return &f, nil
}

// ListFraudFlags returns transfers whose fraud decision is review or block,
// newest first, with their fraud scores attached.
func (s *Store) ListFraudFlags(ctx context.Context) ([]domain.Transfer, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.source_account_id, t.dest_account_id, t.amount_minor, t.currency,
		        t.status, t.created_at,
		        f.transfer_id, f.score::text, f.decision, f.model_version, f.created_at
		 FROM transfers t
		 JOIN fraud_scores f ON f.transfer_id = t.id
		 WHERE f.decision IN ('review', 'block')
		 ORDER BY t.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list fraud flags: %w", err)
	}
	defer rows.Close()

	var out []domain.Transfer
	for rows.Next() {
		var t domain.Transfer
		var f domain.FraudScore
		if err := rows.Scan(
			&t.ID, &t.SourceAccountID, &t.DestAccountID, &t.AmountMinor, &t.Currency,
			&t.Status, &t.CreatedAt,
			&f.TransferID, &f.Score, &f.Decision, &f.ModelVersion, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan fraud flag: %w", err)
		}
		t.FraudScore = &f
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fraud flags: %w", err)
	}
	return out, nil
}
