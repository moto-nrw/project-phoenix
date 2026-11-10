#!/usr/bin/env python3
"""Recover the lost detect-changes result on partial GitHub Actions reruns.

Temporary workaround for re-actors/alls-green#44. Never infer success from
`abandoned`: require the original selection guard and audit all job attempts.
"""
import json
import os
import sys
import urllib.request


def normalize(needs, jobs):
    abandoned = [name for name, job in needs.items() if job["result"] == "abandoned"]
    if not abandoned:
        return needs
    if abandoned != ["detect-changes"]:
        raise ValueError(f"Cannot safely recover abandoned dependencies: {abandoned}")
    if not any(job["name"] == "Detect changes" and job["run_attempt"] == 1
               and job["status"] == "completed" and job["conclusion"] == "success" for job in jobs):
        raise ValueError("Original Detect changes job did not succeed")

    # Missing selection outputs must not disable the mandatory-skip guard.
    # Attempt 1 still had those outputs; require evidence that its guard passed.
    original_gates = [job for job in jobs if job["name"] == "ci-gate" and job["run_attempt"] == 1]
    if len(original_gates) != 1 or not any(
        step["name"] == "Reject skipped mandatory checks" and step["conclusion"] == "success"
        for step in original_gates[0].get("steps", [])
    ):
        raise ValueError("Original mandatory-check guard did not pass")

    latest = {}
    for job in jobs:
        if job["name"] == "ci-gate":
            continue
        previous = latest.get(job["name"])
        # A skipped rerun wrapper must not erase a previous concrete failure.
        if job["conclusion"] == "skipped":
            continue
        if previous is None or (job["run_attempt"], job["id"]) > (previous["run_attempt"], previous["id"]):
            latest[job["name"]] = job
    if latest.get("Detect changes", {}).get("conclusion") != "success":
        raise ValueError("No successful Detect changes job in this run")
    failed = [name for name, job in latest.items() if job["status"] != "completed" or job["conclusion"] != "success"]
    if failed:
        raise ValueError(f"Unsuccessful or unfinished jobs in this run: {failed}")

    return {**needs, "detect-changes": {**needs["detect-changes"], "result": "success"}}


def load_jobs():
    jobs = []
    page = 1
    while True:
        url = (f"{os.environ['GITHUB_API_URL']}/repos/{os.environ['GITHUB_REPOSITORY']}"
               f"/actions/runs/{os.environ['GITHUB_RUN_ID']}/jobs?filter=all&per_page=100&page={page}")
        request = urllib.request.Request(url, headers={
            "Authorization": f"Bearer {os.environ['GH_TOKEN']}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        })
        with urllib.request.urlopen(request, timeout=30) as response:
            batch = json.load(response)["jobs"]
        jobs.extend(batch)
        if len(batch) < 100:
            return jobs
        page += 1


def main():
    needs = json.loads(os.environ["CI_NEEDS"])
    jobs = load_jobs() if any(job["result"] == "abandoned" for job in needs.values()) else []
    recovered = normalize(needs, jobs)
    with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
        output.write(f"jobs={json.dumps(recovered, separators=(',', ':'))}\n")
    if recovered != needs:
        print("Recovered abandoned detect-changes from this run's successful jobs; original mandatory guard passed.")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError) as error:
        print(f"::error::{error}", file=sys.stderr)
        sys.exit(1)
