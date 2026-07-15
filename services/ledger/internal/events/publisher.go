// Package events publishes ledger events to Kafka. Publishing always happens
// after the database transaction has committed, so we never announce a transfer
// that did not actually happen (section 6.2 step 10 of CLAUDE.md).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/nickho/tally/services/ledger/internal/domain"
	"github.com/twmb/franz-go/pkg/kgo"
)

const TopicTransfersCompleted = "transfers.completed"

// transferCompleted is the wire format of a transfers.completed event
// (section 9 of CLAUDE.md). Keyed by transfer id.
type transferCompleted struct {
	TransferID      int64  `json:"transfer_id"`
	SourceAccountID int64  `json:"source_account_id"`
	DestAccountID   int64  `json:"dest_account_id"`
	AmountMinor     int64  `json:"amount_minor"`
	Currency        string `json:"currency"`
	OccurredAt      string `json:"occurred_at"`
}

// Publisher writes events to Kafka. A nil *Publisher is safe to call and does
// nothing, so tests and setups without Kafka need no special casing.
type Publisher struct {
	client *kgo.Client
	log    *slog.Logger
}

// New connects a producer to the given brokers.
func New(brokers []string, log *slog.Logger) (*Publisher, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}
	return &Publisher{client: client, log: log}, nil
}

// TransferCompleted publishes asynchronously and logs on failure. It must never
// fail the request: by the time we publish, the money has already moved, and the
// consumer side is idempotent so a retry after a missed event is safe.
// ponytail: fire-and-forget, add an outbox table if delivery must be guaranteed.
//
// The caller's request context is deliberately NOT used: the transfer is already
// committed, so the publish must outlive the request. Tying it to the request
// context cancels the delivery as soon as the gRPC call returns.
func (p *Publisher) TransferCompleted(_ context.Context, t *domain.Transfer) {
	if p == nil {
		return
	}
	ctx := context.Background()
	payload, err := json.Marshal(transferCompleted{
		TransferID:      t.ID,
		SourceAccountID: t.SourceAccountID,
		DestAccountID:   t.DestAccountID,
		AmountMinor:     t.AmountMinor,
		Currency:        t.Currency,
		OccurredAt:      t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
	if err != nil {
		p.log.Error("marshal transfers.completed", "transfer_id", t.ID, "error", err)
		return
	}
	record := &kgo.Record{
		Topic: TopicTransfersCompleted,
		Key:   []byte(strconv.FormatInt(t.ID, 10)),
		Value: payload,
	}
	p.client.Produce(ctx, record, func(_ *kgo.Record, err error) {
		if err != nil {
			p.log.Error("publish transfers.completed", "transfer_id", t.ID, "error", err)
		}
	})
}

// Close flushes and closes the producer.
func (p *Publisher) Close() {
	if p == nil {
		return
	}
	p.client.Close()
}
