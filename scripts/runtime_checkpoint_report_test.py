"""Validate HTTP outcome accounting in runtime evidence reports."""

import importlib.util
import copy
import pathlib
import unittest

spec = importlib.util.spec_from_file_location(
    "runtime_checkpoint_report", pathlib.Path(__file__).with_name("runtime-checkpoint-report.py")
)
report = importlib.util.module_from_spec(spec)
spec.loader.exec_module(report)


def http_result(status):
    return {
        "scenario": {"name": "enrollment.submit", "expected_status": status},
        "unexpected_statuses": 0,
        "samples": [dict(duration_ms=1, queries=2, rows_affected=0,
                         statements_with_rows=0, pool_wait_count=0, pool_wait_ms=0,
                         status=status, error_body="throttled" if status == 429 else "")
                    for _ in range(30)],
        "metrics_before": "", "metrics_after": "", "deadlocks": 0,
        "lock_samples": dict(waiting_backend_samples=0, max_waiting_backends=0,
                             samples=30, max_sample_gap_ms=2),
    }


def contention_run(pool_waits):
    samples = [dict(duration_ms=float(i), queries=60, status=200 if i < 20 else 409) for i in range(30)]
    return {
        "name": "contention.student-graph", "concurrency": 16, "pool_max_open_connections": 12,
        "warmup_rounds": 5, "measured_rounds": 30, "round_wall_ms": [30.0] * 30,
        "pool_wait_count": pool_waits, "pool_wait_ms": 10.0 * pool_waits, "rounds_with_pool_wait": pool_waits,
        "lock_samples": dict(samples=100, waiting_backend_samples=40, max_waiting_backends=5, max_sample_gap_ms=3),
        "deadlocks": 0,
        "metrics_before": 'phoenix_unit_of_work_rollbacks_total{entry_point="http"} 10',
        "metrics_after": 'phoenix_unit_of_work_rollbacks_total{entry_point="http"} 12',
        "operations": [{"name": "delete", "allowed_statuses": [200, 409], "samples": samples,
                        "status_counts": {"200": 20, "409": 10}, "unexpected_statuses": 0}],
    }


class HTTPOutcomeTests(unittest.TestCase):
    def test_write_rows_remain_separate_from_returned_rows(self):
        result = http_result(201)
        for sample in result["samples"]:
            sample["rows_affected"] = 100
            sample["write_rows_affected"] = 3
        raw = {"workload_version": "enrollment-2694-writes-v7",
               "runs": [[copy.deepcopy(result)] for _ in range(3)]}
        summary = report.summarize(raw)
        metrics = summary["scenarios"]["enrollment.submit"]["median"]
        self.assertEqual(metrics["executed_write_rows_affected"], 90)
        self.assertEqual(metrics["driver_rows_returned_or_changed"], 3000)

    def test_write_report_preserves_final_state(self):
        result = http_result(201)
        state = {"schema_versions": 106, "latest_schema_version": 106,
                 "requests": 210, "children": 210,
                 "throttled_ip_attempts": 115, "throttled_email_attempts": 110}
        raw = {"workload_version": "enrollment-2694-writes-v7",
               "runs": [[copy.deepcopy(result)] for _ in range(3)],
               "final_state": state}
        summary = report.summarize(raw)
        self.assertEqual(summary["final_state"], state)
        self.assertIn("| requests | 210 |", report.markdown(summary))

    def test_rollback_count_uses_observed_counter_deltas(self):
        result = http_result(429)
        result["metrics_before"] = 'phoenix_unit_of_work_rollbacks_total{entry_point="http"} 12'
        result["metrics_after"] = 'phoenix_unit_of_work_rollbacks_total{entry_point="http"} 19'
        measured = report.measured_result(result, False)
        self.assertEqual(measured["metrics"]["transaction_rollbacks"], 7)
        self.assertEqual(measured["metrics"]["http_error_responses"], 30)

    def test_parent_workload_preserves_account_link_counts(self):
        result = http_result(201)
        result["scenario"]["name"] = "enrollment.parent-submit"
        state = {"requests": 315, "children": 315, "parent_requests": 105}
        raw = {"workload_version": "enrollment-2694-writes-v8",
               "runs": [[copy.deepcopy(result)] for _ in range(3)],
               "final_state": state}
        summary = report.summarize(raw)
        self.assertEqual(summary["final_state"], state)
        self.assertEqual(summary["scenarios"]["enrollment.parent-submit"]["median"]["http_unexpected_status_rate"], 0)
        self.assertIn("| parent_requests | 105 |", report.markdown(summary))

    def test_operation_definition_cannot_change_under_a_stable_name(self):
        for key, value in (("method", "DELETE"), ("path", "/different"),
                           ("body", "{}"), ("authenticated", True),
                           ("expected_status", 202)):
            with self.subTest(key=key):
                result = http_result(201)
                raw = {"workload_version": "enrollment-2694-reads-v1",
                       "runs": [[copy.deepcopy(result)] for _ in range(3)]}
                raw["runs"][1][0]["scenario"][key] = value
                if key == "expected_status":
                    for sample in raw["runs"][1][0]["samples"]:
                        sample["status"] = value
                with self.assertRaisesRegex(ValueError, "HTTP operation definition changed"):
                    report.summarize(raw)

    def test_expected_throttling_is_an_error_response_not_an_unexpected_status(self):
        result = report.measured_result(http_result(429), False)
        self.assertEqual(result["metrics"]["http_error_rate"], 1)
        self.assertEqual(result["metrics"]["http_unexpected_status_rate"], 0)
        self.assertEqual(result["stable_errors"], {"throttled": 30})

    def test_success_has_zero_error_rate(self):
        result = report.measured_result(http_result(201), False)
        self.assertEqual(result["metrics"]["http_requests"], 30)
        self.assertEqual(result["metrics"]["http_error_responses"], 0)
        self.assertEqual(result["metrics"]["http_error_rate"], 0)

    def test_declared_zero_cannot_hide_a_failed_sample(self):
        result = http_result(201)
        result["samples"][0]["status"] = 500
        with self.assertRaisesRegex(ValueError, "disagrees with samples"):
            report.measured_result(result, False)

    def test_v2_summarizes_contention_and_recordset_sizes(self):
        result = http_result(200)
        result["scenario"]["name"] = "care-plan.care-end"
        result["jsonb_recordsets"] = [{"site": "jsonb_to_recordset(<set>) AS removal(", "calls": 30,
                                       "min_rows": 3, "max_rows": 3, "total_rows": 90}]
        raw = {"workload_version": "checkpoint-1-v2", "concurrency": 1,
               "runs": [[copy.deepcopy(result)] for _ in range(3)],
               "worker_runs": [[] for _ in range(3)],
               "concurrent_runs": [contention_run(waits) for waits in (4, 8, 6)]}
        summary = report.summarize(raw)
        self.assertEqual(summary["serial_concurrency"], 1)
        self.assertEqual(summary["jsonb_recordsets"]["care-plan.care-end"]["jsonb_to_recordset(<set>) AS removal("],
                         {"calls": [30, 30, 30], "min_rows": [3, 3, 3], "max_rows": [3, 3, 3], "unparsed": 0})
        concurrent = summary["concurrent"]
        self.assertEqual(concurrent["concurrency"], 16)
        self.assertEqual(concurrent["median"]["pool_wait_count"], 6)
        self.assertEqual(concurrent["worst"]["pool_wait_count"], 8)
        self.assertEqual(concurrent["median"]["transaction_rollbacks"], 2)
        text = report.markdown(summary)
        self.assertIn("16 requests in flight per round against a pool of 12 connections", text)
        self.assertIn("| delete | 30 |", text)
        self.assertIn("200: 20, 409: 10", text)

    def test_contention_status_outside_the_allowed_set_is_rejected(self):
        run = contention_run(4)
        run["operations"][0]["samples"][0]["status"] = 500
        run["operations"][0]["unexpected_statuses"] = 1
        with self.assertRaisesRegex(ValueError, "unexpected HTTP status in contention operation delete"):
            report.summarize_concurrent([run])

    def test_contention_counters_exclude_work_between_rounds(self):
        run = contention_run(4)
        run.pop("metrics_before")
        run.pop("metrics_after")
        run["measured_rounds"] = 2
        run["round_wall_ms"] = [30.0, 30.0]
        run["metric_rounds"] = [
            {"before": 'phoenix_unit_of_work_rollbacks_total{entry_point="http"} 10',
             "after": 'phoenix_unit_of_work_rollbacks_total{entry_point="http"} 11'},
            {"before": 'phoenix_unit_of_work_rollbacks_total{entry_point="http"} 20',
             "after": 'phoenix_unit_of_work_rollbacks_total{entry_point="http"} 22'},
        ]
        summary = report.summarize_concurrent([run])
        self.assertEqual(summary["median"]["transaction_rollbacks"], 3)

    def test_consistently_reported_unexpected_status_is_rejected(self):
        result = http_result(201)
        result["samples"][0]["status"] = 500
        result["unexpected_statuses"] = 1
        with self.assertRaisesRegex(ValueError, "unexpected HTTP status in"):
            report.measured_result(result, False)
