"""Regression cases for the fail-closed partial-rerun workaround."""
import copy
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("normalize_ci_needs", Path(__file__).with_name("normalize-ci-needs.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def job(name, conclusion="success", attempt=1, identifier=1):
    return {"name": name, "conclusion": conclusion, "status": "completed", "run_attempt": attempt, "id": identifier}


class NormalizeTests(unittest.TestCase):
    def setUp(self):
        self.needs = {"detect-changes": {"result": "abandoned", "outputs": {}}, "test": {"result": "skipped", "outputs": {}}}
        self.gate = job("ci-gate", "failure")
        self.gate["steps"] = [{"name": "Reject skipped mandatory checks", "conclusion": "success"}]
        self.jobs = [job("Detect changes"), self.gate, job("Test / Backend tests (0)")]

    def test_recovers_observed_partial_rerun_without_mutating_input(self):
        original = copy.deepcopy(self.needs)
        result = module.normalize(self.needs, self.jobs)
        self.assertEqual(result["detect-changes"]["result"], "success")
        self.assertEqual(result["test"]["result"], "skipped")
        self.assertEqual(self.needs, original)

    def test_no_abandoned_preserves_failures_cancellations_and_skips(self):
        for result in ("success", "failure", "cancelled", "skipped", None, "unknown"):
            with self.subTest(result=result):
                needs = {"test": {"result": result, "outputs": {}}}
                self.assertEqual(module.normalize(needs, []), needs)

    def test_requires_original_selection_and_guard_evidence(self):
        for mutation in ("missing-gate", "failed-guard", "missing-steps", "later-gate", "missing-selector", "failed-selector"):
            with self.subTest(mutation=mutation):
                jobs = copy.deepcopy(self.jobs)
                if mutation == "missing-gate":
                    jobs.pop(1)
                elif mutation == "failed-guard":
                    jobs[1]["steps"][0]["conclusion"] = "failure"
                elif mutation == "missing-steps":
                    jobs[1]["steps"] = []
                elif mutation == "later-gate":
                    jobs[1]["run_attempt"] = 2
                elif mutation == "missing-selector":
                    jobs.pop(0)
                else:
                    jobs[0]["conclusion"] = "failure"
                with self.assertRaises(ValueError):
                    module.normalize(self.needs, jobs)

    def test_every_unresolved_bad_or_pending_job_blocks_even_with_skipped_wrapper(self):
        for conclusion in ("failure", "cancelled", "timed_out", "action_required", "abandoned", "neutral", None):
            with self.subTest(conclusion=conclusion):
                jobs = self.jobs + [job("Lint / Backend lint", conclusion), job("Lint", "skipped", 3)]
                with self.assertRaises(ValueError):
                    module.normalize(self.needs, jobs)
        pending = job("Lint / Backend lint")
        pending["status"] = "in_progress"
        with self.assertRaises(ValueError):
            module.normalize(self.needs, self.jobs + [pending])

    def test_latest_success_replaces_failure_regardless_of_api_order(self):
        failed = job("Test / Backend tests (0)", "failure", 1, 10)
        passed = job("Test / Backend tests (0)", "success", 2, 20)
        for extra in ([passed, failed], [failed, passed]):
            self.assertEqual(module.normalize(self.needs, self.jobs + extra)["detect-changes"]["result"], "success")

    def test_later_failure_or_skip_cannot_hide_red(self):
        for conclusion in ("failure", "skipped"):
            with self.subTest(conclusion=conclusion):
                jobs = self.jobs + [job("Lint / Backend lint", "failure", 1, 10), job("Lint / Backend lint", conclusion, 2, 20)]
                with self.assertRaises(ValueError):
                    module.normalize(self.needs, jobs)

    def test_other_abandoned_dependencies_fail_closed(self):
        self.needs["test"]["result"] = "abandoned"
        with self.assertRaises(ValueError):
            module.normalize(self.needs, self.jobs)

    def test_successful_selector_rerun_cannot_replace_missing_original_outputs(self):
        self.jobs[0]["conclusion"] = "failure"
        with self.assertRaises(ValueError):
            module.normalize(self.needs, self.jobs + [job("Detect changes", "success", 2, 20)])

    def test_cli_output_and_no_api_on_normal_attempt(self):
        with tempfile.TemporaryDirectory() as directory:
            output = str(Path(directory) / "output")
            needs = {"test": {"result": "failure", "outputs": {"value": "a\nb"}}}
            with patch.dict(os.environ, CI_NEEDS=json.dumps(needs), GITHUB_OUTPUT=output), patch.object(module, "load_jobs") as load:
                module.main()
                load.assert_not_called()
            self.assertEqual(json.loads(Path(output).read_text().removeprefix("jobs=")), needs)

    def test_pagination_does_not_ignore_failures_after_page_one(self):
        first = self.jobs + [job(f"passing-{i}") for i in range(97)]
        responses = [io.BytesIO(json.dumps({"jobs": first}).encode()), io.BytesIO(json.dumps({"jobs": [job("hidden failure", "failure")]}).encode())]
        env = {"GITHUB_API_URL": "https://api.github.com", "GITHUB_REPOSITORY": "owner/repo", "GITHUB_RUN_ID": "123", "GH_TOKEN": "fixture"}
        with patch.dict(os.environ, env), patch.object(module.urllib.request, "urlopen", side_effect=responses) as request:
            jobs = module.load_jobs()
            self.assertIn("page=2", request.call_args.args[0].full_url)
            with self.assertRaises(ValueError):
                module.normalize(self.needs, jobs)


if __name__ == "__main__":
    unittest.main()
