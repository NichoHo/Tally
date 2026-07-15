package store

import (
	"context"
	"fmt"
	"time"
)

// DailyVolume is one day's transfer activity.
type DailyVolume struct {
	Date        string // YYYY-MM-DD
	VolumeMinor int64
	Count       int64
}

// Stats are the aggregates the dashboard home page shows.
// NOTE: volume sums minor units across all currencies. The demo data is single
// currency (USD); add a per-currency breakdown before mixing currencies.
type Stats struct {
	TotalAccounts            int64
	TransferCountToday       int64
	TransferVolumeTodayMinor int64
	FlaggedCount             int64
	DailyVolume              []DailyVolume // last 7 days, oldest first
}

// GetStats computes the dashboard aggregates with plain SQL.
func (s *Store) GetStats(ctx context.Context) (*Stats, error) {
	var st Stats
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&st.TotalAccounts); err != nil {
		return nil, fmt.Errorf("count accounts: %w", err)
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(amount_minor), 0) FROM transfers
		 WHERE created_at >= date_trunc('day', now())`,
	).Scan(&st.TransferCountToday, &st.TransferVolumeTodayMinor); err != nil {
		return nil, fmt.Errorf("today's transfers: %w", err)
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM fraud_scores WHERE decision IN ('review', 'block')`,
	).Scan(&st.FlaggedCount); err != nil {
		return nil, fmt.Errorf("count flagged: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT d::date, COALESCE(SUM(t.amount_minor), 0), COUNT(t.id)
		 FROM generate_series(now()::date - 6, now()::date, '1 day') AS d
		 LEFT JOIN transfers t ON t.created_at::date = d::date
		 GROUP BY 1 ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("daily volume: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var day time.Time
		var v DailyVolume
		if err := rows.Scan(&day, &v.VolumeMinor, &v.Count); err != nil {
			return nil, fmt.Errorf("scan daily volume: %w", err)
		}
		v.Date = day.Format("2006-01-02")
		st.DailyVolume = append(st.DailyVolume, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily volume: %w", err)
	}
	return &st, nil
}
