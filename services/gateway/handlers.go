package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	ledgerpb "github.com/nickho/tally/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	ledger ledgerpb.LedgerServiceClient
	log    *slog.Logger
}

// ---- JSON helpers ----

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// httpStatus maps a gRPC error to the HTTP status codes required by section 8.1.
func httpStatus(err error) (int, string) {
	st, _ := status.FromError(err)
	switch st.Code() {
	case codes.InvalidArgument:
		return http.StatusBadRequest, st.Message()
	case codes.NotFound:
		return http.StatusNotFound, st.Message()
	case codes.FailedPrecondition:
		return http.StatusUnprocessableEntity, st.Message()
	case codes.AlreadyExists:
		return http.StatusConflict, st.Message()
	default:
		return http.StatusInternalServerError, "internal error"
	}
}

func (s *server) fail(w http.ResponseWriter, err error) {
	code, msg := httpStatus(err)
	if code == http.StatusInternalServerError {
		s.log.Error("ledger call failed", "error", err)
	}
	writeError(w, code, msg)
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// ---- view types (snake_case JSON, matching section 8/9) ----

type accountView struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	BalanceMinor  int64  `json:"balance_minor"`
	AllowNegative bool   `json:"allow_negative"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type entryView struct {
	ID          int64  `json:"id"`
	TransferID  int64  `json:"transfer_id"`
	AccountID   int64  `json:"account_id"`
	Direction   string `json:"direction"`
	AmountMinor int64  `json:"amount_minor"`
	CreatedAt   string `json:"created_at"`
}

type transferView struct {
	ID              int64           `json:"id"`
	SourceAccountID int64           `json:"source_account_id"`
	DestAccountID   int64           `json:"dest_account_id"`
	AmountMinor     int64           `json:"amount_minor"`
	Currency        string          `json:"currency"`
	Status          string          `json:"status"`
	CreatedAt       string          `json:"created_at"`
	Entries         []entryView     `json:"entries,omitempty"`
	FraudScore      *fraudScoreView `json:"fraud_score,omitempty"`
}

type fraudScoreView struct {
	TransferID   int64  `json:"transfer_id"`
	Score        string `json:"score"`
	Decision     string `json:"decision"`
	ModelVersion string `json:"model_version"`
	CreatedAt    string `json:"created_at"`
}

func viewAccount(a *ledgerpb.Account) accountView {
	return accountView{
		ID: a.GetId(), Name: a.GetName(), Currency: a.GetCurrency(),
		BalanceMinor: a.GetBalanceMinor(), AllowNegative: a.GetAllowNegative(),
		CreatedAt: a.GetCreatedAt(), UpdatedAt: a.GetUpdatedAt(),
	}
}

func viewEntry(e *ledgerpb.LedgerEntry) entryView {
	return entryView{
		ID: e.GetId(), TransferID: e.GetTransferId(), AccountID: e.GetAccountId(),
		Direction: e.GetDirection(), AmountMinor: e.GetAmountMinor(), CreatedAt: e.GetCreatedAt(),
	}
}

func viewTransfer(t *ledgerpb.Transfer) transferView {
	v := transferView{
		ID: t.GetId(), SourceAccountID: t.GetSourceAccountId(), DestAccountID: t.GetDestAccountId(),
		AmountMinor: t.GetAmountMinor(), Currency: t.GetCurrency(), Status: t.GetStatus(),
		CreatedAt: t.GetCreatedAt(),
	}
	for _, e := range t.GetEntries() {
		v.Entries = append(v.Entries, viewEntry(e))
	}
	if f := t.GetFraudScore(); f != nil {
		v.FraudScore = &fraudScoreView{
			TransferID:   f.GetTransferId(),
			Score:        f.GetScore(),
			Decision:     f.GetDecision(),
			ModelVersion: f.GetModelVersion(),
			CreatedAt:    f.GetCreatedAt(),
		}
	}
	return v
}

// ---- account handlers ----

func (s *server) createAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          string `json:"name"`
		Currency      string `json:"currency"`
		AllowNegative bool   `json:"allow_negative"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	a, err := s.ledger.CreateAccount(r.Context(), &ledgerpb.CreateAccountRequest{
		Name: body.Name, Currency: body.Currency, AllowNegative: body.AllowNegative,
	})
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewAccount(a))
}

func (s *server) listAccounts(w http.ResponseWriter, r *http.Request) {
	resp, err := s.ledger.ListAccounts(r.Context(), &ledgerpb.ListAccountsRequest{})
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]accountView, 0, len(resp.GetAccounts()))
	for _, a := range resp.GetAccounts() {
		out = append(out, viewAccount(a))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) getAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	a, err := s.ledger.GetAccount(r.Context(), &ledgerpb.GetAccountRequest{Id: id})
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewAccount(a))
}

func (s *server) listAccountEntries(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}
	resp, err := s.ledger.ListAccountEntries(r.Context(), &ledgerpb.ListAccountEntriesRequest{AccountId: id})
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]entryView, 0, len(resp.GetEntries()))
	for _, e := range resp.GetEntries() {
		out = append(out, viewEntry(e))
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- transfer handlers ----

func (s *server) createTransfer(w http.ResponseWriter, r *http.Request) {
	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}
	var body struct {
		SourceAccountID int64  `json:"source_account_id"`
		DestAccountID   int64  `json:"dest_account_id"`
		AmountMinor     int64  `json:"amount_minor"`
		Currency        string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	t, err := s.ledger.CreateTransfer(r.Context(), &ledgerpb.CreateTransferRequest{
		SourceAccountId: body.SourceAccountID,
		DestAccountId:   body.DestAccountID,
		AmountMinor:     body.AmountMinor,
		Currency:        body.Currency,
		IdempotencyKey:  idemKey,
	})
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewTransfer(t))
}

func (s *server) getTransfer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid transfer id")
		return
	}
	t, err := s.ledger.GetTransfer(r.Context(), &ledgerpb.GetTransferRequest{Id: id})
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewTransfer(t))
}

func (s *server) listTransfers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	beforeID, _ := strconv.ParseInt(r.URL.Query().Get("before_id"), 10, 64)
	resp, err := s.ledger.ListTransfers(r.Context(), &ledgerpb.ListTransfersRequest{
		Limit: int32(limit), BeforeId: beforeID,
	})
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]transferView, 0, len(resp.GetTransfers()))
	for _, t := range resp.GetTransfers() {
		out = append(out, viewTransfer(t))
	}
	writeJSON(w, http.StatusOK, out)
}

// getStats returns the dashboard aggregates.
func (s *server) getStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.ledger.GetStats(r.Context(), &ledgerpb.GetStatsRequest{})
	if err != nil {
		s.fail(w, err)
		return
	}
	type daily struct {
		Date        string `json:"date"`
		VolumeMinor int64  `json:"volume_minor"`
		Count       int64  `json:"count"`
	}
	out := struct {
		TotalAccounts            int64   `json:"total_accounts"`
		TransferCountToday       int64   `json:"transfer_count_today"`
		TransferVolumeTodayMinor int64   `json:"transfer_volume_today_minor"`
		FlaggedCount             int64   `json:"flagged_count"`
		DailyVolume              []daily `json:"daily_volume"`
	}{
		TotalAccounts:            st.GetTotalAccounts(),
		TransferCountToday:       st.GetTransferCountToday(),
		TransferVolumeTodayMinor: st.GetTransferVolumeTodayMinor(),
		FlaggedCount:             st.GetFlaggedCount(),
		DailyVolume:              make([]daily, 0, len(st.GetDailyVolume())),
	}
	for _, d := range st.GetDailyVolume() {
		out.DailyVolume = append(out.DailyVolume, daily{
			Date: d.GetDate(), VolumeMinor: d.GetVolumeMinor(), Count: d.GetCount(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// listFraudFlags returns transfers whose fraud decision is review or block.
func (s *server) listFraudFlags(w http.ResponseWriter, r *http.Request) {
	resp, err := s.ledger.ListFraudFlags(r.Context(), &ledgerpb.ListFraudFlagsRequest{})
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]transferView, 0, len(resp.GetTransfers()))
	for _, t := range resp.GetTransfers() {
		out = append(out, viewTransfer(t))
	}
	writeJSON(w, http.StatusOK, out)
}
