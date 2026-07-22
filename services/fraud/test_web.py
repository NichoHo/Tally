"""Integration test for the free-tier HTTP scoring path: score_pending must
score unscored transfers exactly once and be a no-op on a second call. Needs a
real Postgres; skipped if TEST_DATABASE_URL is unset."""

import os

import pytest

psycopg = pytest.importorskip("psycopg")

from web import score_pending  # noqa: E402

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


def test_score_pending_scores_once(conn):
    for _ in range(3):
        conn.execute(
            """INSERT INTO transfers (source_account_id, dest_account_id, amount_minor,
               currency, status) VALUES (1, 2, 1000, 'USD', 'completed')"""
        )

    assert score_pending(conn) == 3  # all three scored
    assert score_pending(conn) == 0  # nothing left to score

    count = conn.execute("SELECT COUNT(*) FROM fraud_scores").fetchone()[0]
    assert count == 3  # one row per transfer, no duplicates
