"""Fraud scoring logic: features, rules, and the optional IsolationForest.

This module is pure: no Kafka, no database. The consumer feeds it plain numbers
and it returns a score and a decision. Scores are illustrative; the model is
trained on synthetic data and is not production fraud detection.
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

MODEL_PATH = Path(__file__).parent / "model.joblib"
MODEL_VERSION_RULES = "rules-v1"
MODEL_VERSION_IFOREST = "iforest-v1"

# Thresholds are configurable via env (section 10 of CLAUDE.md).
REVIEW_THRESHOLD = float(os.getenv("FRAUD_REVIEW_THRESHOLD", "0.5"))
BLOCK_THRESHOLD = float(os.getenv("FRAUD_BLOCK_THRESHOLD", "0.8"))

LARGE_AMOUNT_MINOR = int(os.getenv("FRAUD_LARGE_AMOUNT_MINOR", "100000"))  # $1,000.00
HUGE_AMOUNT_MINOR = int(os.getenv("FRAUD_HUGE_AMOUNT_MINOR", "500000"))  # $5,000.00
HIGH_VELOCITY_COUNT = int(os.getenv("FRAUD_HIGH_VELOCITY_COUNT", "5"))


@dataclass(frozen=True)
class Features:
    """The signals we score a transfer on (section 10 of CLAUDE.md)."""

    amount_minor: int
    recent_count: int  # source's transfers in the last 10 minutes
    recent_volume_minor: int  # source's total volume in the last 10 minutes
    new_destination: bool  # source has never paid this destination before
    hour_of_day: int  # 0 to 23, from the event's occurred_at


def build_features(
    amount_minor: int,
    recent_count: int,
    recent_volume_minor: int,
    new_destination: bool,
    hour_of_day: int,
) -> Features:
    """Assemble and sanity-check the feature vector."""
    if not 0 <= hour_of_day <= 23:
        raise ValueError(f"hour_of_day must be 0..23, got {hour_of_day}")
    if amount_minor <= 0:
        raise ValueError(f"amount_minor must be positive, got {amount_minor}")
    return Features(
        amount_minor=amount_minor,
        recent_count=max(0, recent_count),
        recent_volume_minor=max(0, recent_volume_minor),
        new_destination=new_destination,
        hour_of_day=hour_of_day,
    )


def rule_score(f: Features) -> float:
    """v1 rule-based score: additive, capped at 1.0, easy to explain."""
    score = 0.0
    if f.amount_minor > LARGE_AMOUNT_MINOR:
        score += 0.4
    if f.amount_minor > HUGE_AMOUNT_MINOR:
        score += 0.3
    if f.recent_count > HIGH_VELOCITY_COUNT:
        score += 0.3
    if f.new_destination:
        score += 0.2
    if f.hour_of_day < 6:  # small hours are mildly unusual
        score += 0.1
    return min(score, 1.0)


def to_vector(f: Features) -> list[float]:
    """Feature order shared between training (train.py) and scoring."""
    return [
        float(f.amount_minor),
        float(f.recent_count),
        float(f.recent_volume_minor),
        1.0 if f.new_destination else 0.0,
        float(f.hour_of_day),
    ]


class Scorer:
    """Scores transfers. Uses the IsolationForest if model.joblib exists,
    blended with the rules; otherwise rules alone."""

    def __init__(self, model_path: Path = MODEL_PATH) -> None:
        self.model = None
        self.version = MODEL_VERSION_RULES
        if model_path.exists():
            import joblib  # imported lazily so rule-only tests need no sklearn

            self.model = joblib.load(model_path)
            self.version = MODEL_VERSION_IFOREST

    def score(self, f: Features) -> float:
        rules = rule_score(f)
        if self.model is None:
            return rules
        # decision_function: positive = normal, negative = anomalous, roughly
        # in [-0.2, 0.2]. ponytail: crude linear calibration to 0..1, replace
        # with a calibrated mapping if score quality ever matters.
        df = float(self.model.decision_function([to_vector(f)])[0])
        ml = min(1.0, max(0.0, 0.5 - df * 2.5))
        return max(rules, ml)


def decide(score: float) -> str:
    """Map a 0..1 score to a decision: allow, review, or block."""
    if score > BLOCK_THRESHOLD:
        return "block"
    if score >= REVIEW_THRESHOLD:
        return "review"
    return "allow"
