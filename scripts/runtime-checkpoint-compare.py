#!/usr/bin/env python3
"""Compare two runtime checkpoint summaries metric by metric.

Both inputs are ``summary.json`` files written by ``runtime-checkpoint-report.py``
for the same workload version. Every metric present in both summaries is
classified for the median and the worst run:

- ``unchanged``: identical values.
- ``material``: an exact-invariant metric (queries, rows, waits, deadlocks,
  retries, job counters, HTTP outcomes) changed at all, or a latency metric
  moved beyond both the relative and the absolute tolerance.
- ``within-tolerance``: a latency metric moved, but not beyond both tolerances.
- ``coverage``: observer coverage (lock sample count, sampling gap); reported,
  never a regression verdict.

A changed error contract is material. Metrics that only one side measured are
listed as unmeasured, not compared. The classification does not explain a
change; every material entry needs an explanation in the checkpoint issue.
"""

import argparse
import json
import pathlib

LATENCY_METRICS = frozenset({"latency_p50_ms", "latency_p95_ms", "job_duration_total_ms"})
COVERAGE_METRICS = frozenset({"lock_samples", "lock_max_sample_gap_ms"})
DEFAULT_RELATIVE_TOLERANCE = 0.20
DEFAULT_ABSOLUTE_TOLERANCE_MS = 0.5


def classify(metric, baseline, candidate, relative_tolerance, absolute_tolerance_ms):
    if metric in COVERAGE_METRICS:
        return "coverage"
    if baseline == candidate:
        return "unchanged"
    if metric not in LATENCY_METRICS:
        return "material"
    difference = abs(candidate - baseline)
    if difference > absolute_tolerance_ms and difference > relative_tolerance * abs(baseline):
        return "material"
    return "within-tolerance"


def ratio(baseline, candidate):
    # A zero baseline has no finite ratio; JSON has no infinity literal, so the
    # delta carries the change and the ratio stays null.
    if baseline == 0:
        return None
    return candidate / baseline


def compare(baseline, candidate, relative_tolerance=DEFAULT_RELATIVE_TOLERANCE,
            absolute_tolerance_ms=DEFAULT_ABSOLUTE_TOLERANCE_MS):
    if baseline["workload_version"] != candidate["workload_version"]:
        raise ValueError("workload version differs; measure an old/new bridge on the same commit instead")
    if set(baseline["scenarios"]) != set(candidate["scenarios"]):
        raise ValueError("scenario set differs between baseline and candidate")
    result = {
        "workload_version": baseline["workload_version"],
        "tolerance": {"relative": relative_tolerance, "absolute_ms": absolute_tolerance_ms},
        "scenarios": {},
        "material": [],
    }
    for name, base in baseline["scenarios"].items():
        cand = candidate["scenarios"][name]
        if base["kind"] != cand["kind"]:
            raise ValueError(f"scenario kind differs: {name}")
        shared = [key for key in base["median"] if key in cand["median"]]
        entry = {
            "kind": base["kind"],
            "metrics": {},
            "unmeasured_in_baseline": sorted(set(cand["median"]) - set(base["median"])),
            "unmeasured_in_candidate": sorted(set(base["median"]) - set(cand["median"])),
        }
        for key in shared:
            metric = {"baseline": {}, "candidate": {}, "delta": {}, "ratio": {}, "classification": {}}
            for statistic in ("median", "worst"):
                before, after = base[statistic][key], cand[statistic][key]
                metric["baseline"][statistic] = before
                metric["candidate"][statistic] = after
                metric["delta"][statistic] = after - before
                metric["ratio"][statistic] = ratio(before, after)
                verdict = classify(key, before, after, relative_tolerance, absolute_tolerance_ms)
                metric["classification"][statistic] = verdict
                if verdict == "material":
                    result["material"].append({
                        "scenario": name, "metric": key, "statistic": statistic,
                        "baseline": before, "candidate": after,
                        "delta": after - before, "ratio": ratio(before, after),
                    })
            entry["metrics"][key] = metric
        errors_changed = base["stable_errors"] != cand["stable_errors"]
        entry["stable_errors"] = {
            "baseline": base["stable_errors"], "candidate": cand["stable_errors"],
            "classification": "material" if errors_changed else "unchanged",
        }
        if errors_changed:
            result["material"].append({
                "scenario": name, "metric": "stable_errors", "statistic": "contract",
                "baseline": base["stable_errors"], "candidate": cand["stable_errors"],
            })
        result["scenarios"][name] = entry
    return result


def cell(metrics, key, statistic):
    """Format one table cell; a metric measured on only one side reads as unmeasured."""
    metric = metrics.get(key)
    if metric is None:
        return "unmeasured"
    before = metric["baseline"][statistic]
    after = metric["candidate"][statistic]
    text = f"{before:.3f} → {after:.3f}"
    if before != 0:
        text += f" ({(after - before) / before:+.1%})"
    elif after != 0:
        text += " (from zero)"
    return text


TABLE_COLUMNS = ("latency_p50_ms", "latency_p95_ms", "queries_total", "rows", "pool_wait_ms",
                 "lock_waiting_backend_samples")


def table(result, statistic):
    """One row per scenario for the given statistic; Material lists that statistic's verdicts only."""
    lines = ["| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |",
             "|---|---:|---:|---:|---:|---:|---:|---|"]
    for name, scenario in result["scenarios"].items():
        metrics = scenario["metrics"]
        row_key = "driver_rows_returned_or_changed"
        if scenario["kind"] == "worker":
            row_key = "job_rows_affected" if "job_rows_affected" in metrics else "job_claimed_rows"
        flagged = sorted({item["metric"] for item in result["material"]
                          if item["scenario"] == name and item["statistic"] in (statistic, "contract")})
        cells = [name] + [cell(metrics, row_key if key == "rows" else key, statistic) for key in TABLE_COLUMNS]
        cells.append(", ".join(flagged) if flagged else "none")
        lines.append("| " + " | ".join(cells) + " |")
    return lines


def markdown(result):
    tolerance = result["tolerance"]
    lines = [f"# Runtime workload {result['workload_version']}: baseline versus candidate", "",
             "Each cell shows baseline → candidate. Latency uses nearest-rank percentiles.",
             f"Latency changes count as material only beyond relative {tolerance['relative']:.1%} and "
             f"absolute {tolerance['absolute_ms']:.3f} ms; any change to another metric is material. "
             "The Material column of each table lists only that table's statistic.", "",
             "## Median of three runs", ""]
    lines.extend(table(result, "median"))
    lines.extend(["", "## Worst of three runs", "",
                  "The worst run is the maximum of each metric across runs, chosen per metric.", ""])
    lines.extend(table(result, "worst"))
    lines.extend(["", "¹ HTTP: driver-reported rows returned or changed, not distinct database rows. "
                  "Worker: outbox rows claimed or timetable instances created.", "",
                  "## Material changes", ""])
    if result["material"]:
        for item in result["material"]:
            if item["metric"] == "stable_errors":
                lines.append(f"- `{item['scenario']}` error contract changed: "
                             f"{json.dumps(item['baseline'])} → {json.dumps(item['candidate'])}")
            else:
                lines.append(f"- `{item['scenario']}` `{item['metric']}` {item['statistic']}: "
                             f"{item['baseline']:.3f} → {item['candidate']:.3f}")
    else:
        lines.append("None.")
    counts = {}
    for scenario in result["scenarios"].values():
        for metric in scenario["metrics"].values():
            for verdict in metric["classification"].values():
                counts[verdict] = counts.get(verdict, 0) + 1
    lines.extend(["", "## Classification counts across all scenarios, metrics, and statistics", ""])
    for verdict in ("unchanged", "within-tolerance", "material", "coverage"):
        lines.append(f"- {verdict}: {counts.get(verdict, 0)}")
    unmeasured = sorted({key for scenario in result["scenarios"].values()
                         for key in scenario["unmeasured_in_baseline"]})
    if unmeasured:
        lines.extend(["", "## Measured only in the candidate", "",
                      "These metrics have no baseline value and are not compared: " +
                      ", ".join(f"`{key}`" for key in unmeasured) + "."])
    lines.append("")
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("baseline", type=pathlib.Path, help="baseline summary.json")
    parser.add_argument("candidate", type=pathlib.Path, help="candidate summary.json")
    parser.add_argument("output_directory", type=pathlib.Path)
    parser.add_argument("--relative-tolerance", type=float, default=DEFAULT_RELATIVE_TOLERANCE)
    parser.add_argument("--absolute-tolerance-ms", type=float, default=DEFAULT_ABSOLUTE_TOLERANCE_MS)
    args = parser.parse_args()
    result = compare(json.loads(args.baseline.read_text()), json.loads(args.candidate.read_text()),
                     args.relative_tolerance, args.absolute_tolerance_ms)
    args.output_directory.mkdir(parents=True, exist_ok=True)
    (args.output_directory / "comparison.json").write_text(json.dumps(result, indent=2, allow_nan=False) + "\n")
    (args.output_directory / "comparison.md").write_text(markdown(result))


if __name__ == "__main__":
    main()
