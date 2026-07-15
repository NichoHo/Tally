#!/usr/bin/env bash
# Seed the ledger with realistic demo accounts and transfers by calling the REST
# gateway (so all money moves through the real double-entry path). The gateway
# must be running (make up). All amounts are minor units (cents).
#
# After seeding, transfer timestamps are spread over the past week so the
# dashboard chart has shape. Only created_at changes; balances and ledger
# entries are untouched, so every invariant still holds.
set -euo pipefail

API="${API:-http://localhost:8080}"

acct() { # name currency allow_negative -> prints new account id
  curl -s -X POST "$API/v1/accounts" -H 'Content-Type: application/json' \
    -d "{\"name\":\"$1\",\"currency\":\"$2\",\"allow_negative\":$3}" |
    sed -n 's/.*"id":\([0-9]*\).*/\1/p'
}

transfer() { # src dst amount
  curl -s -o /dev/null -w "transfer $1->$2 $3: HTTP %{http_code}\n" \
    -X POST "$API/v1/transfers" \
    -H "Idempotency-Key: seed-$(date +%s%N)" -H 'Content-Type: application/json' \
    -d "{\"source_account_id\":$1,\"dest_account_id\":$2,\"amount_minor\":$3,\"currency\":\"USD\"}"
}

echo "seeding demo data at $API"

# A treasury allowed to go negative acts as the money source, so the
# system-wide ledger sum stays zero.
TREASURY=$(acct "treasury" "USD" true)
ALICE=$(acct "alice nguyen" "USD" false)
BOB=$(acct "bob martinez" "USD" false)
CARMEN=$(acct "carmen okafor" "USD" false)
DEVI=$(acct "devi sharma" "USD" false)
ELIAS=$(acct "elias brandt" "USD" false)

echo "accounts: treasury=$TREASURY alice=$ALICE bob=$BOB carmen=$CARMEN devi=$DEVI elias=$ELIAS"

# Fund the customer accounts.
transfer "$TREASURY" "$ALICE" 250000   # $2,500.00
transfer "$TREASURY" "$BOB" 180000     # $1,800.00
transfer "$TREASURY" "$CARMEN" 140000  # $1,400.00
transfer "$TREASURY" "$DEVI" 90000     # $900.00
transfer "$TREASURY" "$ELIAS" 320000   # $3,200.00

# Everyday movements between customers.
transfer "$ALICE" "$BOB" 3500
transfer "$BOB" "$CARMEN" 12000
transfer "$CARMEN" "$ALICE" 4200
transfer "$ALICE" "$DEVI" 1500
transfer "$DEVI" "$ELIAS" 2750
transfer "$ELIAS" "$ALICE" 8900
transfer "$BOB" "$DEVI" 640
transfer "$CARMEN" "$ELIAS" 5300
transfer "$ELIAS" "$BOB" 15000
transfer "$ALICE" "$CARMEN" 2200
transfer "$DEVI" "$BOB" 1875
transfer "$BOB" "$ALICE" 950

# A couple of large transfers so the fraud page has something to show.
transfer "$ELIAS" "$CARMEN" 120000    # $1,200.00, above the large threshold
transfer "$ALICE" "$ELIAS" 155000     # $1,550.00

# Spread transfer timestamps over the past week so the 7 day chart has shape.
# created_at is display-only; the money math never reads it.
if docker ps --format '{{.Names}}' | grep -q '^tally-postgres-1$'; then
  docker exec tally-postgres-1 psql -U tally -d tally -q -c "
    UPDATE transfers SET created_at = created_at - (interval '1 day' * floor(random() * 7))
    WHERE id > 5;
    UPDATE ledger_entries e SET created_at = t.created_at
    FROM transfers t WHERE e.transfer_id = t.id;
    UPDATE fraud_scores f SET created_at = t.created_at
    FROM transfers t WHERE f.transfer_id = t.id;"
  echo "spread transfer dates over the past 7 days"
fi

echo "done. open http://localhost:3000 to see the dashboard."
