"""Fraud service: consume transfers.completed, score, write fraud_scores,
publish fraud.scored.

Consumption is idempotent: fraud_scores has a unique index on transfer_id and we
insert with ON CONFLICT DO NOTHING, so replaying the same event cannot create a
second row (section 9 of CLAUDE.md).
"""

from __future__ import annotations

import json
import logging
import os
import sys
from datetime import datetime

import psycopg
from confluent_kafka import Consumer, Producer

from model import Scorer, build_features, decide

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("fraud")

TOPIC_IN = "transfers.completed"
TOPIC_OUT = "fraud.scored"


def fetch_history(conn: psycopg.Connection, event: dict) -> tuple[int, int, bool]:
    """Source's recent velocity and whether the destination is new, computed
    from transfers before this one."""
    source = event["source_account_id"]
    dest = event["dest_account_id"]
    transfer_id = event["transfer_id"]
    with conn.cursor() as cur:
        cur.execute(
            """SELECT COUNT(*), COALESCE(SUM(amount_minor), 0) FROM transfers
               WHERE source_account_id = %s AND id <> %s
                 AND created_at > now() - interval '10 minutes'""",
            (source, transfer_id),
        )
        recent_count, recent_volume = cur.fetchone()
        cur.execute(
            """SELECT NOT EXISTS(
                 SELECT 1 FROM transfers
                 WHERE source_account_id = %s AND dest_account_id = %s AND id < %s)""",
            (source, dest, transfer_id),
        )
        (new_destination,) = cur.fetchone()
    return int(recent_count), int(recent_volume), bool(new_destination)


def insert_score(
    conn: psycopg.Connection, transfer_id: int, score: float, decision: str, version: str
) -> bool:
    """Write the fraud score. Returns False if this transfer was already scored
    (the idempotent replay case)."""
    with conn.cursor() as cur:
        cur.execute(
            """INSERT INTO fraud_scores (transfer_id, score, decision, model_version)
               VALUES (%s, %s, %s, %s)
               ON CONFLICT (transfer_id) DO NOTHING
               RETURNING id""",
            (transfer_id, round(score, 3), decision, version),
        )
        return cur.fetchone() is not None


def process(conn: psycopg.Connection, producer: Producer, scorer: Scorer, event: dict) -> None:
    transfer_id = event["transfer_id"]
    recent_count, recent_volume, new_destination = fetch_history(conn, event)
    hour = datetime.fromisoformat(event["occurred_at"].replace("Z", "+00:00")).hour
    features = build_features(
        amount_minor=event["amount_minor"],
        recent_count=recent_count,
        recent_volume_minor=recent_volume,
        new_destination=new_destination,
        hour_of_day=hour,
    )
    score = scorer.score(features)
    decision = decide(score)

    if not insert_score(conn, transfer_id, score, decision, scorer.version):
        log.info("transfer %s already scored, skipping (idempotent replay)", transfer_id)
        return

    payload = {
        "transfer_id": transfer_id,
        "score": round(score, 3),
        "decision": decision,
        "model_version": scorer.version,
    }
    producer.produce(TOPIC_OUT, key=str(transfer_id), value=json.dumps(payload))
    producer.poll(0)
    log.info("scored transfer %s: %.3f -> %s", transfer_id, score, decision)


def main() -> None:
    dsn = os.environ.get("DATABASE_URL")
    brokers = os.environ.get("KAFKA_BROKERS")
    if not dsn or not brokers:
        log.error("DATABASE_URL and KAFKA_BROKERS are required")
        sys.exit(1)

    scorer = Scorer()
    log.info("scoring with %s", scorer.version)

    conn = psycopg.connect(dsn, autocommit=True)
    consumer = Consumer(
        {
            "bootstrap.servers": brokers,
            "group.id": "fraud-service",
            "auto.offset.reset": "earliest",
            "enable.auto.commit": False,
        }
    )
    producer = Producer({"bootstrap.servers": brokers})
    consumer.subscribe([TOPIC_IN])
    log.info("consuming %s from %s", TOPIC_IN, brokers)

    while True:
        msg = consumer.poll(1.0)
        if msg is None:
            continue
        if msg.error():
            log.error("kafka error: %s", msg.error())
            continue
        try:
            event = json.loads(msg.value())
            process(conn, producer, scorer, event)
        except Exception:  # noqa: BLE001 - keep consuming; the event stays replayable
            log.exception("failed to process event, skipping: %s", msg.value())
        # Commit the offset only after the score is durably written, so a crash
        # replays the event and the ON CONFLICT dedupe absorbs it.
        consumer.commit(msg)


if __name__ == "__main__":
    main()
