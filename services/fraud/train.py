"""Train the IsolationForest on synthetic transaction data.

Run: python train.py
Writes model.joblib next to this file. The data is synthetic and the model is
illustrative, not production fraud detection. Kept as a script so the whole
pipeline is reproducible (section 10 of CLAUDE.md).
"""

from __future__ import annotations

import joblib
import numpy as np
from sklearn.ensemble import IsolationForest

from model import MODEL_PATH

RNG = np.random.default_rng(42)
N = 20_000


def synthetic_normal_transactions(n: int) -> np.ndarray:
    """Plausible everyday transfers, in the to_vector() feature order:
    amount_minor, recent_count, recent_volume_minor, new_destination, hour."""
    amounts = np.clip(RNG.lognormal(mean=8.0, sigma=1.2, size=n), 100, 200_000)
    recent_count = RNG.poisson(lam=1.0, size=n)
    recent_volume = amounts * np.maximum(recent_count, 1) * RNG.uniform(0.5, 1.5, size=n)
    # Most payments go to known destinations.
    new_dest = (RNG.uniform(size=n) < 0.15).astype(float)
    # Daytime-heavy hours.
    hours = np.clip(RNG.normal(loc=14, scale=4, size=n), 0, 23).round()
    return np.column_stack([amounts, recent_count, recent_volume, new_dest, hours])


def main() -> None:
    X = synthetic_normal_transactions(N)
    model = IsolationForest(n_estimators=100, contamination=0.02, random_state=42)
    model.fit(X)
    joblib.dump(model, MODEL_PATH)
    print(f"trained IsolationForest on {N} synthetic rows -> {MODEL_PATH}")


if __name__ == "__main__":
    main()
