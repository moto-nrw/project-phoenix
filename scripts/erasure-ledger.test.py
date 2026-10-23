"""Synthetic restore gate contracts; no network or application services."""
import hashlib
import datetime as dt
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('daily_backup', Path(__file__).with_name('daily_backup.py'))
backup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(backup)
maintenance_spec = importlib.util.spec_from_file_location('compact', Path(__file__).with_name('compact-erasure-ledger.py'))
maintenance = importlib.util.module_from_spec(maintenance_spec)
maintenance_spec.loader.exec_module(maintenance)


class ErasureLedgerTest(unittest.TestCase):
    def test_retention_rebuild_keeps_recent_erasures_and_a_valid_chain(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'ledger'
            header = b'moto-erasure-ledger-v1\n'
            previous = hashlib.sha256(header).hexdigest().encode()
            content = header
            for school, when in [(17, '2026-01-01T00:00:00Z'), (29, '2026-10-24T00:00:00Z')]:
                payload = json.dumps(dict(school_id=school, order_reference='ORDER-RETENTION',
                                          completed_at=when, receipt_sha256='b' * 64)).encode()
                previous = hashlib.sha256(previous + b'\n' + payload).hexdigest().encode()
                content += previous + b' ' + payload + b'\n'
            path.write_bytes(content)
            path.chmod(0o600)
            maintenance.compact(path, dt.datetime(2026, 10, 24, tzinfo=dt.timezone.utc))
            self.assertEqual([entry['school_id'] for entry in backup.read_erasure_ledger(path)], [29])
            maintenance.compact(path, dt.datetime(2026, 10, 24, tzinfo=dt.timezone.utc))
            self.assertEqual(len(backup.read_erasure_ledger(path)), 1)

    def test_missing_or_damaged_ledger_blocks_review(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'ledger'
            with self.assertRaises(Exception):
                backup.read_erasure_ledger(path)
            for content in (b'', b'moto-erasure-ledger-v1\ntruncated', b'moto-erasure-ledger-v1\n{}\n'):
                path.write_bytes(content)
                path.chmod(0o600)
                with self.assertRaises(Exception):
                    backup.read_erasure_ledger(path)

    def test_restored_school_blocks_release_and_other_school_is_untouched(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / 'ledger'
            header = b'moto-erasure-ledger-v1\n'
            entry = json.dumps(dict(school_id=17, order_reference='ORDER-RESTORE',
                                    completed_at='2026-10-24T12:00:00Z', receipt_sha256='a' * 64)).encode()
            digest = hashlib.sha256(hashlib.sha256(header).hexdigest().encode() + b'\n' + entry).hexdigest()
            path.write_bytes(header + digest.encode() + b' ' + entry + b'\n')
            path.chmod(0o600)
            recovery = backup.Recovery.__new__(backup.Recovery)
            schools = {17, 29}
            queries = []

            def sql(query):
                queries.append(query)
                return str(int(17 in schools))

            recovery.sql = sql
            with self.assertRaises(Exception):
                recovery.check_erasures(path)
            self.assertEqual(backup.step, 'erasure_replay_required')
            self.assertEqual(schools, {17, 29})
            schools.remove(17)  # Simulate completed tenant erase, never alter B.
            recovery.check_erasures(path)
            self.assertEqual(schools, {29})
            self.assertTrue(all(q.startswith('SELECT ') for q in queries))


if __name__ == '__main__':
    unittest.main()
