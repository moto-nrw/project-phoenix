#!/usr/bin/env python3
"""Summarize the versioned, three-run backend runtime checkpoint evidence."""

import argparse
import collections
import gzip
import json
import lzma
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
        "checkpoint-1-v2": 3,
        "enrollment-2694-reads-v1": 0,
        "enrollment-2694-phase-writes-v1": 0,
        "enrollment-2694-writes-v2": 0,
        "enrollment-2694-writes-v3": 0,
        "enrollment-2694-writes-v4": 0,
        "enrollment-2694-writes-v5": 0,
        "enrollment-2694-writes-v6": 0,
        "enrollment-2694-writes-v7": 0,
        "enrollment-2694-writes-v8": 0,
        "enrollment-2696-change-requests-v1": 0,
        "enrollment-2699-acceptance-v1": 0,
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
    if "concurrency" in raw:
        summary["serial_concurrency"] = raw["concurrency"]
    recordsets = recordset_summary(raw)
    if recordsets:
        summary["jsonb_recordsets"] = recordsets
    if raw.get("concurrent_runs"):
        summary["concurrent"] = summarize_concurrent(raw["concurrent_runs"])
    if "final_state" in raw:
        summary["final_state"] = raw["final_state"]
    return summary


def recordset_summary(raw):
    """Per scenario and jsonb_to_recordset call site: calls and set sizes per run."""
    sites = {}
    sources = [(result["scenario"]["name"], result) for run in raw["runs"] for result in run]
    sources += [(result["name"], result) for run in (raw.get("worker_runs") or []) for result in run]
    sources += [(run["name"], run) for run in (raw.get("concurrent_runs") or [])]
    for name, result in sources:
        for entry in result.get("jsonb_recordsets") or []:
            site = sites.setdefault(name, {}).setdefault(
                entry["site"], {"calls": [], "min_rows": [], "max_rows": [], "unparsed": 0})
            site["calls"].append(entry["calls"])
            site["min_rows"].append(entry["min_rows"])
            site["max_rows"].append(entry["max_rows"])
            site["unparsed"] += entry.get("unparsed", 0)
    return sites


def summarize_concurrent(runs):
    """Contention runs: pool, lock and deadlock evidence plus per-operation outcomes."""
    names = {run["name"] for run in runs}
    if len(names) != 1:
        raise ValueError("concurrent runs must repeat one contention workload")
    summary = {"name": runs[0]["name"], "concurrency": runs[0]["concurrency"],
               "pool_max_open_connections": runs[0]["pool_max_open_connections"], "runs": []}
    for run in runs:
        if run["concurrency"] != summary["concurrency"]:
            raise ValueError("concurrency changed between contention runs")
        if len(run["round_wall_ms"]) != run["measured_rounds"]:
            raise ValueError("each contention run must record one wall time per measured round")
        metric_rounds = run.get("metric_rounds")
        if metric_rounds is None:
            metric_rounds = [{"before": run["metrics_before"], "after": run["metrics_after"]}]
        elif len(metric_rounds) != run["measured_rounds"]:
            raise ValueError("each contention run must record one counter delta per measured round")
        delta = collections.Counter()
        for metric_round in metric_rounds:
            before, after = counters(metric_round["before"]), counters(metric_round["after"])
            round_delta = {key: value - before.get(key, 0) for key, value in after.items()}
            if any(value < 0 for value in round_delta.values()):
                raise ValueError("counters reset during a contention round")
            delta.update(round_delta)
        locks = run["lock_samples"]
        if locks.get("error"):
            raise ValueError(locks["error"])
        walls = [{"duration_ms": value} for value in run["round_wall_ms"]]
        metrics = {
            "round_wall_p50_ms": percentile(walls, 0.50),
            "round_wall_p95_ms": percentile(walls, 0.95),
            "pool_wait_count": run["pool_wait_count"],
            "pool_wait_ms": run["pool_wait_ms"],
            "rounds_with_pool_wait": run["rounds_with_pool_wait"],
            "lock_waiting_backend_samples": locks["waiting_backend_samples"],
            "lock_max_waiting_backends": locks["max_waiting_backends"],
            "lock_samples": locks["samples"],
            "lock_max_sample_gap_ms": locks["max_sample_gap_ms"],
            "deadlocks": run["deadlocks"],
            "transaction_rollbacks": sum(value for key, value in delta.items()
                                         if key.split("{", 1)[0] == "phoenix_unit_of_work_rollbacks_total"),
            "transaction_retries": sum(value for key, value in delta.items()
                                       if key.startswith("phoenix_unit_of_work_retries_total")),
        }
        operations = {}
        for operation in run["operations"]:
            samples = operation["samples"]
            unexpected = sum(sample["status"] not in operation["allowed_statuses"] for sample in samples)
            if unexpected != operation["unexpected_statuses"]:
                raise ValueError("unexpected contention status count disagrees with samples")
            if unexpected:
                raise ValueError(f"unexpected HTTP status in contention operation {operation['name']}")
            operations[operation["name"]] = {
                "requests": len(samples),
                "latency_p50_ms": percentile(samples, 0.50),
                "latency_p95_ms": percentile(samples, 0.95),
                "queries_total": sum(sample["queries"] for sample in samples),
                "status_counts": operation["status_counts"],
            }
        summary["runs"].append({"metrics": metrics, "operations": operations})
    summary["median"] = {}
    summary["worst"] = {}
    for key in summary["runs"][0]["metrics"]:
        values = [run["metrics"][key] for run in summary["runs"]]
        summary["median"][key] = statistics.median(values)
        # Fewer observer samples means poorer lock-wait coverage.
        summary["worst"][key] = min(values) if key == "lock_samples" else max(values)
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
    if "concurrent" in summary:
        lines.extend(concurrent_markdown(summary["concurrent"]))
    if "jsonb_recordsets" in summary:
        lines.extend(["## jsonb_to_recordset set sizes", "",
                      "Rows per call of each call site, one value per run. A bounded use keeps its maximum "
                      "independent of the tenant's size.", "",
                      "| Scenario | Call site | Calls per run | Min rows | Max rows |", "|---|---|---|---|---|"])
        for name, sites in summary["jsonb_recordsets"].items():
            for site, values in sites.items():
                lines.append(f"| {name} | `{site}` | {values['calls']} | {values['min_rows']} | {values['max_rows']} |")
        lines.append("")
    if "final_state" in summary:
        lines.extend(["## Observed final database state", "",
                      "Measured after the workload, outside request timing. These totals are not per-request rows changed.", "",
                      "| Counter | Final value |", "|---|---:|"])
        for key, value in sorted(summary["final_state"].items()):
            lines.append(f"| {key} | {value} |")
        lines.append("")
    return "\n".join(lines)


def concurrent_markdown(concurrent):
    lines = [f"## Contention run `{concurrent['name']}`", "",
             f"{concurrent['concurrency']} requests in flight per round against a pool of "
             f"{concurrent['pool_max_open_connections']} connections. Median / worst across three runs.", "",
             "| Metric | Median | Worst |", "|---|---:|---:|"]
    for key, value in concurrent["median"].items():
        lines.append(f"| {key} | {value:.3f} | {concurrent['worst'][key]:.3f} |")
    lines.extend(["", "| Operation | Requests per run | p50 ms per run | p95 ms per run | Statuses per run |",
                  "|---|---:|---|---|---|"])
    for name, operation in concurrent["runs"][0]["operations"].items():
        runs = [run["operations"][name] for run in concurrent["runs"]]
        statuses = " / ".join(", ".join(f"{code}: {count}" for code, count in sorted(run["status_counts"].items()))
                              for run in runs)
        p50 = " / ".join(f"{run['latency_p50_ms']:.3f}" for run in runs)
        p95 = " / ".join(f"{run['latency_p95_ms']:.3f}" for run in runs)
        lines.append(f"| {name} | {operation['requests']} | {p50} | {p95} | {statuses} |")
    lines.append("")
    return lines


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("raw", type=pathlib.Path)
    parser.add_argument("output_directory", type=pathlib.Path)
    args = parser.parse_args()
    opener = {".gz": gzip.open, ".xz": lzma.open}.get(args.raw.suffix, open)
    with opener(args.raw, "rt") as source:
        summary = summarize(json.load(source))
    args.output_directory.mkdir(parents=True, exist_ok=True)
    (args.output_directory / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    (args.output_directory / "results.md").write_text(markdown(summary))


if __name__ == "__main__":
    main()
