"""Free-tier scoring endpoint: score transfers synchronously over HTTP instead
of consuming Kafka events, so the hosted demo needs no always-on broker or
consumer. Local development still uses the event-driven consumer (consumer.py);
this produces the identical fraud_scores rows, just triggered by a request from
the gateway after each transfer.

Idempotent by the same fraud_scores unique index as the consumer: scoring a
transfer that already has a row writes nothing (insert_score returns False).
"""

from __future__ import annotations

import logging
import os
from datetime import timezone

import psycopg
from flask import Flask, jsonify

from consumer import fetch_history, insert_score
from model import Scorer, build_features, decide

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("fraud-web")

app = Flask(__name__)
_scorer = Scorer()
log.info("scoring with %s", _scorer.version)


def score_pending(conn: psycopg.Connection) -> int:
    """Score every transfer that has no fraud_scores row yet, using the same
    features and model as the Kafka consumer. Returns how many were newly scored.
    Safe to call repeatedly and concurrently: already-scored transfers are
    skipped and the unique index absorbs races."""
    with conn.cursor() as cur:
        cur.execute(
            """SELECT id, source_account_id, dest_account_id, amount_minor, created_at
               FROM transfers t
               WHERE NOT EXISTS (SELECT 1 FROM fraud_scores f WHERE f.transfer_id = t.id)
               ORDER BY id"""
        )
        rows = cur.fetchall()

    scored = 0
    for transfer_id, source, dest, amount_minor, created_at in rows:
        event = {
            "transfer_id": transfer_id,
            "source_account_id": source,
            "dest_account_id": dest,
        }
        recent_count, recent_volume, new_destination = fetch_history(conn, event)
        features = build_features(
            amount_minor=amount_minor,
            recent_count=recent_count,
            recent_volume_minor=recent_volume,
            new_destination=new_destination,
            hour_of_day=created_at.astimezone(timezone.utc).hour,
        )
        score = _scorer.score(features)
        if insert_score(conn, transfer_id, score, decide(score), _scorer.version):
            scored += 1
    return scored


@app.post("/score-pending")
def score_pending_endpoint():
    dsn = os.environ["DATABASE_URL"]
    with psycopg.connect(dsn, autocommit=True) as conn:
        n = score_pending(conn)
    log.info("scored %d pending transfer(s)", n)
    return jsonify({"scored": n})


@app.get("/healthz")
def healthz():
    return jsonify({"status": "ok"})
