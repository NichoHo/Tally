"""Integration test: consuming the same event twice must not duplicate
fraud_scores rows. Needs a real Postgres; skipped if TEST_DATABASE_URL is unset."""

import os

import pytest

psycopg = pytest.importorskip("psycopg")

from consumer import insert_score  # noqa: E402

DSN = os.environ.get("TEST_DATABASE_URL")

pytestmark = pytest.mark.skipif(not DSN, reason="TEST_DATABASE_URL not set")

SCHEMA = """
DROP TABLE IF EXISTS fraud_scores;
DROP TABLE IF EXISTS transfers CASCADE;
CREATE TABLE transfers (
    id BIGSERIAL PRIMARY KEY,
    source_account_id BIGINT NOT NULL,
    dest_account_id BIGINT NOT NULL,
    amount_minor BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE fraud_scores (
    id BIGSERIAL PRIMARY KEY,
    transfer_id BIGINT NOT NULL REFERENCES transfers(id),
    score NUMERIC(4,3) NOT NULL,
    decision TEXT NOT NULL,
    model_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_fraud_transfer_unique ON fraud_scores(transfer_id);
"""


@pytest.fixture()
def conn():
    with psycopg.connect(DSN, autocommit=True) as c:
        c.execute(SCHEMA)
        yield c


def test_duplicate_event_writes_one_row(conn):
    tid = conn.execute(
        """INSERT INTO transfers (source_account_id, dest_account_id, amount_minor,
           currency, status) VALUES (1, 2, 1000, 'USD', 'completed') RETURNING id"""
    ).fetchone()[0]

    first = insert_score(conn, tid, 0.42, "allow", "rules-v1")
    second = insert_score(conn, tid, 0.42, "allow", "rules-v1")

    assert first is True  # first write lands
    assert second is False  # replay is absorbed

    count = conn.execute(
        "SELECT COUNT(*) FROM fraud_scores WHERE transfer_id = %s", (tid,)
    ).fetchone()[0]
    assert count == 1
