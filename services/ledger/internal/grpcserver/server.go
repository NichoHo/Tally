// Package grpcserver adapts the store to the gRPC LedgerService contract. It
// converts between proto messages and domain types and maps domain errors to
// gRPC status codes.
package grpcserver

import (
	"context"
	"errors"

	ledgerpb "github.com/nickho/tally/proto"
	"github.com/nickho/tally/services/ledger/internal/domain"
	"github.com/nickho/tally/services/ledger/internal/events"
	"github.com/nickho/tally/services/ledger/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements ledgerpb.LedgerServiceServer.
type Server struct {
	ledgerpb.UnimplementedLedgerServiceServer
	store     *store.Store
	publisher *events.Publisher // nil is fine: publishing becomes a no-op
}

// New returns a gRPC server backed by the given store. publisher may be nil
// when Kafka is not configured (for example in tests).
func New(s *store.Store, publisher *events.Publisher) *Server {
	return &Server{store: s, publisher: publisher}
}

// mapErr turns a domain error into a gRPC status. The gateway maps these codes
// to HTTP status codes.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrSameAccount),
		errors.Is(err, domain.ErrInvalidCurrency),
		errors.Is(err, domain.ErrInvalidName),
		errors.Is(err, domain.ErrMissingIdempotencyKey):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrAccountNotFound),
		errors.Is(err, domain.ErrTransferNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrCurrencyMismatch),
		errors.Is(err, domain.ErrInsufficientFunds):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func (s *Server) CreateAccount(ctx context.Context, req *ledgerpb.CreateAccountRequest) (*ledgerpb.Account, error) {
	a, err := s.store.CreateAccount(ctx, domain.CreateAccountParams{
		Name:          req.GetName(),
		Currency:      req.GetCurrency(),
		AllowNegative: req.GetAllowNegative(),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoAccount(a), nil
}

func (s *Server) GetAccount(ctx context.Context, req *ledgerpb.GetAccountRequest) (*ledgerpb.Account, error) {
	a, err := s.store.GetAccount(ctx, req.GetId())
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoAccount(a), nil
}

func (s *Server) ListAccounts(ctx context.Context, _ *ledgerpb.ListAccountsRequest) (*ledgerpb.ListAccountsResponse, error) {
	accounts, err := s.store.ListAccounts(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &ledgerpb.ListAccountsResponse{}
	for i := range accounts {
		resp.Accounts = append(resp.Accounts, toProtoAccount(&accounts[i]))
	}
	return resp, nil
}

func (s *Server) ListAccountEntries(ctx context.Context, req *ledgerpb.ListAccountEntriesRequest) (*ledgerpb.ListAccountEntriesResponse, error) {
	entries, err := s.store.ListAccountEntries(ctx, req.GetAccountId())
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &ledgerpb.ListAccountEntriesResponse{}
	for i := range entries {
		resp.Entries = append(resp.Entries, toProtoEntry(entries[i]))
	}
	return resp, nil
}

func (s *Server) CreateTransfer(ctx context.Context, req *ledgerpb.CreateTransferRequest) (*ledgerpb.Transfer, error) {
	t, err := s.store.CreateTransfer(ctx, domain.TransferParams{
		SourceAccountID: req.GetSourceAccountId(),
		DestAccountID:   req.GetDestAccountId(),
		AmountMinor:     req.GetAmountMinor(),
		Currency:        req.GetCurrency(),
	}, req.GetIdempotencyKey())
	if err != nil {
		return nil, mapErr(err)
	}
	// The transaction has committed; announce it. Idempotent replays republish
	// the same event, which is harmless because consumers dedupe by transfer id.
	s.publisher.TransferCompleted(ctx, t)
	return toProtoTransfer(t), nil
}

func (s *Server) GetTransfer(ctx context.Context, req *ledgerpb.GetTransferRequest) (*ledgerpb.Transfer, error) {
	t, err := s.store.GetTransfer(ctx, req.GetId())
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoTransfer(t), nil
}

func (s *Server) ListFraudFlags(ctx context.Context, _ *ledgerpb.ListFraudFlagsRequest) (*ledgerpb.ListFraudFlagsResponse, error) {
	flagged, err := s.store.ListFraudFlags(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &ledgerpb.ListFraudFlagsResponse{}
	for i := range flagged {
		resp.Transfers = append(resp.Transfers, toProtoTransfer(&flagged[i]))
	}
	return resp, nil
}

func (s *Server) GetStats(ctx context.Context, _ *ledgerpb.GetStatsRequest) (*ledgerpb.Stats, error) {
	st, err := s.store.GetStats(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &ledgerpb.Stats{
		TotalAccounts:            st.TotalAccounts,
		TransferCountToday:       st.TransferCountToday,
		TransferVolumeTodayMinor: st.TransferVolumeTodayMinor,
		FlaggedCount:             st.FlaggedCount,
	}
	for _, d := range st.DailyVolume {
		resp.DailyVolume = append(resp.DailyVolume, &ledgerpb.DailyVolume{
			Date: d.Date, VolumeMinor: d.VolumeMinor, Count: d.Count,
		})
	}
	return resp, nil
}

func (s *Server) ListTransfers(ctx context.Context, req *ledgerpb.ListTransfersRequest) (*ledgerpb.ListTransfersResponse, error) {
	transfers, err := s.store.ListTransfers(ctx, int(req.GetLimit()), req.GetBeforeId())
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &ledgerpb.ListTransfersResponse{}
	for i := range transfers {
		resp.Transfers = append(resp.Transfers, toProtoTransfer(&transfers[i]))
	}
	return resp, nil
}
