#!/usr/bin/env python3
"""Summarize the versioned, three-run backend runtime checkpoint evidence."""

import argparse
import collections
import gzip
import json
import math
import pathlib
import statistics


def percentile(samples, fraction):
    values = sorted(sample["duration_ms"] for sample in samples)
    return values[math.ceil(len(values) * fraction) - 1]


def counters(text):
    result = {}
    for line in text.splitlines():
        key, value = line.rsplit(" ", 1)
        name = key.split("{", 1)[0]
        if name.endswith(("_total", "_sum", "_count")):
            number = float(value)
            if not math.isfinite(number):
                raise ValueError(f"non-finite counter: {key}")
            result[key] = number
    return result


def measured_result(result, worker):
    samples = result["samples"]
    if len(samples) != 30:
        raise ValueError("each scenario must contain exactly 30 measured samples")
    before, after = counters(result["metrics_before"]), counters(result["metrics_after"])
    delta = {key: value - before.get(key, 0) for key, value in after.items()}
    if any(value < 0 for value in delta.values()):
        raise ValueError("counters reset during a measured scenario")
    locks = result["lock_samples"]
    if locks.get("error"):
        raise ValueError(locks["error"])
    numeric = {
        "latency_p50_ms": percentile(samples, 0.50),
        "latency_p95_ms": percentile(samples, 0.95),
        "queries_total": sum(sample["queries"] for sample in samples),
        "queries_min_per_operation": min(sample["queries"] for sample in samples),
        "queries_max_per_operation": max(sample["queries"] for sample in samples),
        "driver_rows_returned_or_changed": sum(sample["rows_affected"] for sample in samples),
        "statements_with_row_counts": sum(sample["statements_with_rows"] for sample in samples),
        "pool_wait_count": sum(sample["pool_wait_count"] for sample in samples),
        "pool_wait_ms": sum(sample["pool_wait_ms"] for sample in samples),
        "lock_waiting_backend_samples": locks["waiting_backend_samples"],
        "lock_max_waiting_backends": locks["max_waiting_backends"],
        "lock_samples": locks["samples"],
        "lock_max_sample_gap_ms": locks["max_sample_gap_ms"],
        "deadlocks": result["deadlocks"],
        "transaction_rollbacks": sum(value for key, value in delta.items()
                                     if key.split("{", 1)[0] == "phoenix_unit_of_work_rollbacks_total"),
        "transaction_retries": sum(value for key, value in delta.items()
                                   if key.startswith("phoenix_unit_of_work_retries_total")),
        "module_rows_returned_or_changed": sum(value for key, value in delta.items()
                                              if "_rows_total{" in key),
    }
    if all("write_rows_affected" in sample for sample in samples):
        numeric["executed_write_rows_affected"] = sum(sample["write_rows_affected"] for sample in samples)
    if worker:
        numeric["job_duration_total_ms"] = sum(sample["duration_ms"] for sample in samples)
        if result.get("rows_affected"):
            numeric["job_rows_affected"] = sum(result["rows_affected"])
            numeric["job_rows_skipped_existing"] = sum(result["rows_skipped"])
        if result.get("claimed"):
            numeric.update({
                "job_claimed_rows": sum(result["claimed"]),
                "job_retry_scheduled": sum(state == "pending" for state in result["states"]),
                "job_backlog_before_max": max(result["backlog_before"]),
                "job_backlog_after_max": max(result["backlog_after"]),
                "job_attempts_max": max(result["attempts"]),
            })
        errors = dict(collections.Counter(error for error in (result.get("errors") or []) if error))
    else:
        expected_status = result["scenario"]["expected_status"]
        unexpected = sum(sample["status"] != expected_status for sample in samples)
        if unexpected != result["unexpected_statuses"]:
            raise ValueError("unexpected HTTP status count disagrees with samples")
        if unexpected:
            raise ValueError(f"unexpected HTTP status in {result['scenario']['name']}")
        error_count = sum(sample["status"] >= 400 for sample in samples)
        numeric.update({
            "http_requests": len(samples),
            "http_error_responses": error_count,
            "http_error_rate": error_count / len(samples),
            "http_unexpected_status_rate": unexpected / len(samples),
        })
        errors = dict(collections.Counter(sample["error_body"] for sample in samples
                                          if sample.get("error_body")))
    return {"metrics": numeric, "stable_errors": errors, "not_applicable": result.get("not_applicable", ""),
            "counter_deltas": {key: value for key, value in delta.items() if value}}


def summarize(raw):
    worker_run_counts = {
        "checkpoint-1-v1": 3,
        "enrollment-2694-reads-v1": 0,
        "enrollment-2694-phase-writes-v1": 0,
        "enrollment-2694-writes-v2": 0,
        "enrollment-2694-writes-v3": 0,
        "enrollment-2694-writes-v4": 0,
        "enrollment-2694-writes-v5": 0,
        "enrollment-2694-writes-v6": 0,
        "enrollment-2694-writes-v7": 0,
        "enrollment-2694-writes-v8": 0,
    }
    version = raw["workload_version"]
    if version not in worker_run_counts:
        raise ValueError("unsupported workload version")
    worker_runs = raw.get("worker_runs") or []
    if len(raw["runs"]) != 3 or len(worker_runs) != worker_run_counts[version]:
        raise ValueError(f"{version} requires three HTTP runs and {worker_run_counts[version]} worker runs")
    scenarios = {}
    for worker, runs in ((False, raw["runs"]), (True, worker_runs)):
        expected_names = None
        for run in runs:
            names = [result["name"] if worker else result["scenario"]["name"] for result in run]
            if len(set(names)) != len(names) or (expected_names is not None and names != expected_names):
                raise ValueError("scenario set or order changed between runs")
            expected_names = names
            for name, result in zip(names, run):
                scenarios.setdefault(name, {"kind": "worker" if worker else "http", "runs": []})
                if not worker:
                    definition = scenarios[name].setdefault("operation", result["scenario"])
                    if definition != result["scenario"]:
                        raise ValueError(f"HTTP operation definition changed between runs: {name}")
                scenarios[name]["runs"].append(measured_result(result, worker))
    for scenario in scenarios.values():
        scenario["median"] = {}
        scenario["worst"] = {}
        for key in scenario["runs"][0]["metrics"]:
            values = [run["metrics"][key] for run in scenario["runs"]]
            scenario["median"][key] = statistics.median(values)
            # Fewer observer samples means poorer lock-wait coverage.
            scenario["worst"][key] = min(values) if key == "lock_samples" else max(values)
        if any(run["stable_errors"] != scenario["runs"][0]["stable_errors"]
               for run in scenario["runs"]):
            raise ValueError("error contract changed between runs")
        scenario["stable_errors"] = scenario["runs"][0]["stable_errors"]
    summary = {"workload_version": raw["workload_version"], "scenarios": scenarios}
    if "final_state" in raw:
        summary["final_state"] = raw["final_state"]
    return summary


def markdown(summary):
    lines = [f"# Runtime workload {summary['workload_version']}: measured results", "",
             "Values are median / worst across three runs. Latency uses nearest-rank percentiles.", "",
             "| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations |",
             "|---|---:|---:|---:|---:|---:|---:|"]
    for name, scenario in summary["scenarios"].items():
        def pair(key):
            return f"{scenario['median'][key]:.3f} / {scenario['worst'][key]:.3f}"
        row_key = "driver_rows_returned_or_changed"
        if scenario["kind"] == "worker":
            row_key = "job_rows_affected" if "job_rows_affected" in scenario["median"] else "job_claimed_rows"
        cells = [name, pair("latency_p50_ms"), pair("latency_p95_ms"), pair("queries_total"),
                 pair(row_key), pair("pool_wait_ms"), pair("lock_waiting_backend_samples")]
        lines.append("| " + " | ".join(cells) + " |")
    lines.extend(["", "¹ HTTP: driver-reported rows returned or changed, not distinct database rows. "
                  "Worker: outbox rows claimed or timetable instances created. Zero does not assert that every adapter has row instrumentation.", "",
                  "`summary.json` contains every numeric metric's three values, median and worst, "
                  "stable errors, and nonzero raw counter deltas. Sampled Lock observations are not "
                  "exact wait durations. Acquisition-statement timing is not used as lock-wait evidence.", ""])
    if "final_state" in summary:
        lines.extend(["## Observed final database state", "",
                      "Measured after the workload, outside request timing. These totals are not per-request rows changed.", "",
                      "| Counter | Final value |", "|---|---:|"])
        for key, value in sorted(summary["final_state"].items()):
            lines.append(f"| {key} | {value} |")
        lines.append("")
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("raw", type=pathlib.Path)
    parser.add_argument("output_directory", type=pathlib.Path)
    args = parser.parse_args()
    opener = gzip.open if args.raw.suffix == ".gz" else open
    with opener(args.raw, "rt") as source:
        summary = summarize(json.load(source))
    args.output_directory.mkdir(parents=True, exist_ok=True)
    (args.output_directory / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    (args.output_directory / "results.md").write_text(markdown(summary))


if __name__ == "__main__":
    main()
