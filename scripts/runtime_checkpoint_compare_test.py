"""Validate the metric-by-metric checkpoint comparison."""

import copy
import importlib.util
import pathlib
import unittest

spec = importlib.util.spec_from_file_location(
    "runtime_checkpoint_compare", pathlib.Path(__file__).with_name("runtime-checkpoint-compare.py")
)
compare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(compare)


def scenario(kind="http", **overrides):
    metrics = {
        "latency_p50_ms": 2.0, "latency_p95_ms": 3.0, "queries_total": 150,
        "driver_rows_returned_or_changed": 30, "pool_wait_ms": 0, "deadlocks": 0,
        "lock_waiting_backend_samples": 0, "lock_samples": 50, "lock_max_sample_gap_ms": 3.0,
    }
    metrics.update(overrides)
    return {"kind": kind, "median": dict(metrics), "worst": dict(metrics),
            "runs": [{"metrics": dict(metrics)} for _ in range(3)],
            "stable_errors": {"invalid room ID": 30}}


def summary(**scenarios):
    return {"workload_version": "checkpoint-1-v1", "scenarios": scenarios}


class ComparisonTests(unittest.TestCase):
    def test_identical_summaries_have_no_material_change(self):
        baseline = summary(a=scenario())
        result = compare.compare(baseline, copy.deepcopy(baseline))
        self.assertEqual(result["material"], [])
        self.assertEqual(result["scenarios"]["a"]["metrics"]["queries_total"]["classification"],
                         {"median": "unchanged", "worst": "unchanged"})

    def test_any_change_to_an_invariant_metric_is_material(self):
        result = compare.compare(summary(a=scenario()), summary(a=scenario(queries_total=151)))
        self.assertEqual([(m["scenario"], m["metric"], m["statistic"]) for m in result["material"]],
                         [("a", "queries_total", "median"), ("a", "queries_total", "worst")])

    def test_latency_within_tolerance_is_not_material(self):
        result = compare.compare(summary(a=scenario()), summary(a=scenario(latency_p50_ms=2.3)))
        self.assertEqual(result["material"], [])
        self.assertEqual(result["scenarios"]["a"]["metrics"]["latency_p50_ms"]["classification"]["median"],
                         "within-tolerance")

    def test_latency_must_exceed_both_relative_and_absolute_tolerance(self):
        small_absolute = compare.compare(summary(a=scenario(latency_p50_ms=0.5)),
                                         summary(a=scenario(latency_p50_ms=0.9)))
        self.assertEqual(small_absolute["material"], [])
        large = compare.compare(summary(a=scenario()), summary(a=scenario(latency_p50_ms=2.6)))
        self.assertEqual([(m["metric"], m["statistic"]) for m in large["material"]],
                         [("latency_p50_ms", "median"), ("latency_p50_ms", "worst")])
        self.assertAlmostEqual(large["material"][0]["ratio"], 1.3)

    def test_worst_run_is_classified_separately_from_median(self):
        candidate = summary(a=scenario())
        candidate["scenarios"]["a"]["worst"]["latency_p95_ms"] = 9.0
        result = compare.compare(summary(a=scenario()), candidate)
        self.assertEqual([(m["metric"], m["statistic"]) for m in result["material"]],
                         [("latency_p95_ms", "worst")])

    def test_coverage_metrics_are_never_material(self):
        result = compare.compare(summary(a=scenario()),
                                 summary(a=scenario(lock_samples=10, lock_max_sample_gap_ms=40.0)))
        self.assertEqual(result["material"], [])
        self.assertEqual(result["scenarios"]["a"]["metrics"]["lock_samples"]["classification"]["median"],
                         "coverage")

    def test_changed_error_contract_is_material(self):
        candidate = summary(a=scenario())
        candidate["scenarios"]["a"]["stable_errors"] = {"invalid room id": 30}
        result = compare.compare(summary(a=scenario()), candidate)
        self.assertEqual([m["metric"] for m in result["material"]], ["stable_errors"])
        self.assertEqual(result["scenarios"]["a"]["stable_errors"]["classification"], "material")

    def test_metric_missing_from_baseline_is_unmeasured_not_material(self):
        result = compare.compare(summary(a=scenario()), summary(a=scenario(executed_write_rows_affected=0)))
        self.assertEqual(result["material"], [])
        self.assertEqual(result["scenarios"]["a"]["unmeasured_in_baseline"], ["executed_write_rows_affected"])
        self.assertNotIn("executed_write_rows_affected", result["scenarios"]["a"]["metrics"])

    def test_workload_version_must_match(self):
        candidate = summary(a=scenario())
        candidate["workload_version"] = "checkpoint-2-v1"
        with self.assertRaisesRegex(ValueError, "workload version"):
            compare.compare(summary(a=scenario()), candidate)

    def test_scenario_set_must_match(self):
        with self.assertRaisesRegex(ValueError, "scenario set"):
            compare.compare(summary(a=scenario()), summary(a=scenario(), b=scenario()))

    def test_markdown_reports_rows_and_material_changes(self):
        result = compare.compare(summary(a=scenario()), summary(a=scenario(queries_total=180, latency_p50_ms=2.1)))
        text = compare.markdown(result)
        self.assertIn("| a | 2.000 → 2.100 (+5.0%) |", text)
        self.assertIn("150.000 → 180.000 (+20.0%)", text)
        self.assertIn("`a` `queries_total` median: 150.000 → 180.000", text)
        self.assertIn("relative 20.0%", text)


if __name__ == "__main__":
    unittest.main()
