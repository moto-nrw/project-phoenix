#!/usr/bin/env python3
"""Offline retention maintenance, only after backup expiry was verified.

No automatic timer: the operator first verifies all backup/release-snapshot
copies have expired within 90 days and stops erasure writers and restores.
"""
import datetime as dt
import fcntl
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile

from daily_backup import read_erasure_ledger, require, sha256


def compact(path, now):
    old_hash = sha256(path)
    entries = read_erasure_ledger(path)
    cutoff = now - dt.timedelta(days=97)  # Maximum 90-day backup + seven days.
    keep = [entry for entry in entries if dt.datetime.fromisoformat(
        entry['completed_at'].replace('Z', '+00:00')) >= cutoff]
    fd = os.open(path, os.O_RDWR | os.O_NOFOLLOW | os.O_NONBLOCK)
    temporary = None
    with os.fdopen(fd, 'rb') as locked:
        fcntl.flock(locked, fcntl.LOCK_EX | fcntl.LOCK_NB)
        info = os.fstat(locked.fileno())
        require(os.path.samestat(info, path.stat()))
        require(hashlib.file_digest(locked, 'sha256').hexdigest() == old_hash)
        try:
            with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as output:
                temporary = Path(output.name)
                os.fchown(output.fileno(), info.st_uid, info.st_gid)
                header = b'moto-erasure-ledger-v1\n'
                output.write(header)
                previous = hashlib.sha256(header).hexdigest().encode()
                for entry in keep:
                    payload = json.dumps(entry, separators=(',', ':')).encode()
                    previous = hashlib.sha256(previous + b'\n' + payload).hexdigest().encode()
                    output.write(previous + b' ' + payload + b'\n')
                output.flush()
                os.fsync(output.fileno())
            os.replace(temporary, path)
            directory = os.open(path.parent, os.O_RDONLY)
            try:
                os.fsync(directory)
            finally:
                os.close(directory)
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)


if __name__ == '__main__':
    try:
        require(len(sys.argv) == 3 and sys.argv[2] == '--backup-expiry-checked-writers-stopped')
        compact(Path(sys.argv[1]), dt.datetime.now(dt.timezone.utc))
        print('erasure_ledger_result=ok step=compacted')
    except Exception:
        print('erasure_ledger_result=failed step=compaction')
        sys.exit(1)
