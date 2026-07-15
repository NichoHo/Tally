package domain

import (
	"errors"
	"testing"
)

func TestTransferParamsValidate(t *testing.T) {
	tests := []struct {
		name string
		p    TransferParams
		want error
	}{
		{"ok", TransferParams{SourceAccountID: 1, DestAccountID: 2, AmountMinor: 100, Currency: "USD"}, nil},
		{"zero amount", TransferParams{SourceAccountID: 1, DestAccountID: 2, AmountMinor: 0, Currency: "USD"}, ErrInvalidAmount},
		{"negative amount", TransferParams{SourceAccountID: 1, DestAccountID: 2, AmountMinor: -5, Currency: "USD"}, ErrInvalidAmount},
		{"same account", TransferParams{SourceAccountID: 1, DestAccountID: 1, AmountMinor: 100, Currency: "USD"}, ErrSameAccount},
		{"bad currency", TransferParams{SourceAccountID: 1, DestAccountID: 2, AmountMinor: 100, Currency: "US"}, ErrInvalidCurrency},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.p.Validate(); !errors.Is(err, tt.want) {
				t.Fatalf("Validate() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestFingerprintStableAndSensitive(t *testing.T) {
	base := TransferParams{SourceAccountID: 1, DestAccountID: 2, AmountMinor: 1000, Currency: "USD"}

	// Same request, same fingerprint.
	same := TransferParams{SourceAccountID: 1, DestAccountID: 2, AmountMinor: 1000, Currency: "USD"}
	if base.Fingerprint() != same.Fingerprint() {
		t.Fatal("fingerprint is not stable for identical requests")
	}

	// Any money-affecting field changing must change the fingerprint.
	changed := []TransferParams{
		{SourceAccountID: 9, DestAccountID: 2, AmountMinor: 1000, Currency: "USD"},
		{SourceAccountID: 1, DestAccountID: 9, AmountMinor: 1000, Currency: "USD"},
		{SourceAccountID: 1, DestAccountID: 2, AmountMinor: 1001, Currency: "USD"},
		{SourceAccountID: 1, DestAccountID: 2, AmountMinor: 1000, Currency: "EUR"},
	}
	for _, c := range changed {
		if c.Fingerprint() == base.Fingerprint() {
			t.Fatalf("fingerprint collided for different request: %+v", c)
		}
	}
}
