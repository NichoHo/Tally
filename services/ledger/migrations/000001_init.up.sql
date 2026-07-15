-- accounts
CREATE TABLE accounts (
    id             BIGSERIAL   PRIMARY KEY,
    name           TEXT        NOT NULL,
    currency       CHAR(3)     NOT NULL,
    balance_minor  BIGINT      NOT NULL DEFAULT 0,   -- cached balance, always kept in sync
    allow_negative BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- transfers (one row per money movement)
CREATE TABLE transfers (
    id                 BIGSERIAL   PRIMARY KEY,
    source_account_id  BIGINT      NOT NULL REFERENCES accounts(id),
    dest_account_id    BIGINT      NOT NULL REFERENCES accounts(id),
    amount_minor       BIGINT      NOT NULL CHECK (amount_minor > 0),
    currency           CHAR(3)     NOT NULL,
    status             TEXT        NOT NULL,   -- 'completed' | 'failed'
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ledger_entries (two rows per transfer: one debit, one credit)
CREATE TABLE ledger_entries (
    id           BIGSERIAL   PRIMARY KEY,
    transfer_id  BIGINT      NOT NULL REFERENCES transfers(id),
    account_id   BIGINT      NOT NULL REFERENCES accounts(id),
    direction    TEXT        NOT NULL CHECK (direction IN ('debit','credit')),
    amount_minor BIGINT      NOT NULL CHECK (amount_minor > 0),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- idempotency_keys (dedupe transfers)
CREATE TABLE idempotency_keys (
    key          TEXT        PRIMARY KEY,
    request_hash TEXT        NOT NULL,   -- fingerprint of the request body
    transfer_id  BIGINT      REFERENCES transfers(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- fraud_scores (written by the fraud service in phase 2)
CREATE TABLE fraud_scores (
    id            BIGSERIAL    PRIMARY KEY,
    transfer_id   BIGINT       NOT NULL REFERENCES transfers(id),
    score         NUMERIC(4,3) NOT NULL,   -- 0.000 to 1.000
    decision      TEXT         NOT NULL,   -- 'allow' | 'review' | 'block'
    model_version TEXT         NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_account ON ledger_entries(account_id);
CREATE INDEX idx_ledger_transfer ON ledger_entries(transfer_id);
CREATE INDEX idx_transfers_created ON transfers(created_at DESC);
CREATE INDEX idx_fraud_transfer ON fraud_scores(transfer_id);
