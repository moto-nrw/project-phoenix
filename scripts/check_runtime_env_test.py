"""Sidecar volume rule of scripts/check-runtime-env.py (#5544, ADR 0148)."""

import importlib.util
from pathlib import Path
import unittest

SPEC = importlib.util.spec_from_file_location("check_runtime_env", Path(__file__).with_name("check-runtime-env.py"))
CHECK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)

TARGET = CHECK.DEMO_UPLOADS_TARGET
SERVER = {"volumes": [{"type": "volume", "source": "demo-avatar-uploads", "target": TARGET}]}


def uploads(**changes):
    volume = {"type": "volume", "source": "demo-avatar-uploads", "target": TARGET}
    volume.update(changes)
    return volume


class DemoRuntimeVolumesTest(unittest.TestCase):
    def test_accepts_exactly_the_server_uploads_volume(self):
        CHECK.check_demo_runtime_volumes({"volumes": [uploads()]}, SERVER, False)

    def test_rejects_every_other_volume(self):
        for name, volumes in {
            "no volume": [],
            "a second volume": [uploads(), {"type": "volume", "source": "demo-postgres", "target": "/data"}],
            "another volume": [uploads(source="demo-postgres")],
            "another target": [uploads(target="/app/public")],
            "a bind mount": [uploads(type="bind", source="/srv/uploads")],
            "a dotenv mount": [{"type": "bind", "source": "/srv/.env", "target": "/app/.env"}],
            "read-only uploads": [uploads(read_only=True)],
        }.items():
            with self.subTest(name), self.assertRaises(ValueError):
                CHECK.check_demo_runtime_volumes({"volumes": volumes}, SERVER, False)

    def test_requires_the_server_to_mount_the_uploads(self):
        with self.assertRaises(ValueError):
            CHECK.check_demo_runtime_volumes({"volumes": [uploads()]}, {"volumes": []}, False)

    def test_history_without_volumes_stays_valid(self):
        CHECK.check_demo_runtime_volumes({}, SERVER, True)
        with self.assertRaises(ValueError):
            CHECK.check_demo_runtime_volumes({"volumes": [uploads(source="demo-postgres")]}, SERVER, True)


class BankRekeyServiceTest(unittest.TestCase):
    def service(self, **changes):
        job = {
            "image": "backend:immutable-test-sha",
            "profiles": ["maintenance"],
            "command": ["./main", "bank-data-rekey"],
            "environment": {"DB_DSN": "${DB_DSN:?DB_DSN is required}"},
        }
        job.update(changes)
        return {"bank-data-rekey": job, "server": {"image": "backend:immutable-test-sha"}}

    def test_accepts_explicit_pinned_maintenance_job(self):
        CHECK.check_bank_rekey_service(self.service())

    def test_rejects_wrong_authority_or_lifecycle(self):
        for change in (
            {"profiles": []},
            {"command": ["./main", "serve"]},
            {"image": "backend:different-release"},
            {"environment": {"DB_DSN": "postgres://phoenix_auth@postgres/postgres"}},
            {"ports": ["8080:8080"]},
            {"volumes": [{"source": ".env", "target": "/app/.env"}]},
        ):
            with self.subTest(change=change), self.assertRaises(ValueError):
                CHECK.check_bank_rekey_service(self.service(**change))


if __name__ == "__main__":
    unittest.main()
