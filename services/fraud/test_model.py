"""Unit tests for the fraud model: feature builder, rules, decision mapping.
Pure Python, no Kafka or database needed."""

import pytest

from model import Features, build_features, decide, rule_score


def features(**overrides) -> Features:
    base = dict(
        amount_minor=2000,
        recent_count=1,
        recent_volume_minor=5000,
        new_destination=False,
        hour_of_day=14,
    )
    base.update(overrides)
    return build_features(**base)


class TestBuildFeatures:
    def test_valid(self):
        f = features()
        assert f.amount_minor == 2000
        assert f.hour_of_day == 14

    def test_rejects_bad_hour(self):
        with pytest.raises(ValueError):
            features(hour_of_day=24)

    def test_rejects_non_positive_amount(self):
        with pytest.raises(ValueError):
            features(amount_minor=0)

    def test_clamps_negative_counts(self):
        f = features(recent_count=-3, recent_volume_minor=-100)
        assert f.recent_count == 0
        assert f.recent_volume_minor == 0


class TestRuleScore:
    def test_ordinary_transfer_scores_low(self):
        assert rule_score(features()) == 0.0

    def test_large_amount_raises_score(self):
        assert rule_score(features(amount_minor=150_000)) == pytest.approx(0.4)

    def test_huge_amount_raises_more(self):
        assert rule_score(features(amount_minor=600_000)) == pytest.approx(0.7)

    def test_velocity_raises_score(self):
        assert rule_score(features(recent_count=10)) == pytest.approx(0.3)

    def test_new_destination_raises_score(self):
        assert rule_score(features(new_destination=True)) == pytest.approx(0.2)

    def test_small_hours_raise_score(self):
        assert rule_score(features(hour_of_day=3)) == pytest.approx(0.1)

    def test_everything_bad_caps_at_one(self):
        f = features(
            amount_minor=1_000_000, recent_count=20, new_destination=True, hour_of_day=2
        )
        assert rule_score(f) == 1.0


class TestDecide:
    @pytest.mark.parametrize(
        ("score", "want"),
        [
            (0.0, "allow"),
            (0.49, "allow"),
            (0.5, "review"),
            (0.8, "review"),
            (0.81, "block"),
            (1.0, "block"),
        ],
    )
    def test_thresholds(self, score, want):
        assert decide(score) == want
