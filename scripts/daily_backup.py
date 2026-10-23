#!/usr/bin/env python3
"""Whole-cluster disaster recovery, never a tenant export or an in-place rollback.

Only ciphertext crosses the transport boundary. Subprocess diagnostics are not
forwarded: tools can print SQL, paths, credentials or upload names on failure.
"""
import datetime as dt
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import secrets
import shutil
import signal
import subprocess
import sys
import tarfile
import tempfile
from urllib.parse import urlparse


FILES = ('database.dump', 'roles.sql', 'uploads.tar.gz')
BUNDLE_NAME = r'daily-[0-9]{8}T[0-9]{6}Z-[a-f0-9]{16}\.tar\.age'
step = 'configuration'


class MissingConfiguration(ValueError):
    """Only a fixed variable name may appear in the public error line."""


def require(condition):
    if not condition:
        raise ValueError('precondition failed')


def required(name):
    value = os.environ.get(name, '')
    if not value:
        raise MissingConfiguration(name)
    require('\n' not in value and '\r' not in value)
    return value


def command(args, *, stdin=None, stdout=None, data=None, env=None):
    result = subprocess.run(args, stdin=stdin, stdout=stdout or subprocess.PIPE,
                            stderr=subprocess.DEVNULL, input=data, env=env, check=True)
    return result.stdout


def sha256(path):
    with path.open('rb') as file:
        return hashlib.file_digest(file, 'sha256').hexdigest()


def regular_archive(path, *, expected=None):
    """Reject traversal, links, devices, duplicate paths and unlisted payloads."""
    names = set()
    with tarfile.open(path) as archive:
        for member in archive:
            name = PurePosixPath(member.name)
            require(not name.is_absolute() and '..' not in name.parts)
            require(member.isfile() or member.isdir())
            require(name not in names)
            names.add(name)
            if expected is not None:
                require(member.isfile() and member.name in expected)
        if expected is not None:
            require(names == {PurePosixPath(n) for n in expected})


class Recovery:
    def __init__(self, mode):
        self.mode = mode
        self.compose_file = Path(required('BACKUP_COMPOSE_FILE'))
        require(self.compose_file.is_absolute() and self.compose_file.is_file())
        self.project = required('BACKUP_PROJECT')
        self.volume = required('BACKUP_UPLOADS_VOLUME')
        require(re.fullmatch(r'[a-z0-9][a-z0-9_-]+', self.project))
        require(re.fullmatch(r'[a-zA-Z0-9][a-zA-Z0-9_.-]+', self.volume))
        self.compose = ['docker', 'compose', '--env-file', str(self.compose_file.parent / '.env'),
                        '-f', str(self.compose_file), '-p', self.project]
        self.transport = required('BACKUP_TRANSPORT')
        self.target = required('BACKUP_TARGET').rstrip('/')
        self.rclone_env = {key: value for key, value in os.environ.items()
                           if not key.startswith(('RCLONE_', 'AWS_'))}
        if self.transport == 's3':
            require(re.fullmatch(r'moto:[a-z0-9][a-z0-9.-]+/[a-zA-Z0-9_/-]+', self.target))
            require('..' not in self.target.split('/'))
            for key in ('PROVIDER', 'ENDPOINT', 'REGION', 'ACCESS_KEY_ID', 'SECRET_ACCESS_KEY'):
                name = 'RCLONE_CONFIG_MOTO_' + key
                self.rclone_env[name] = required(name)
            endpoint = urlparse(self.rclone_env['RCLONE_CONFIG_MOTO_ENDPOINT'])
            require(endpoint.scheme == 'https' and endpoint.hostname and not endpoint.username)
            require(not any(endpoint.hostname == host or endpoint.hostname.endswith('.' + host)
                            for host in ('moto-app.de', 'moto.nrw')))
            self.rclone_env['RCLONE_CONFIG_MOTO_TYPE'] = 's3'
            self.rclone_env['RCLONE_CONFIG_MOTO_ENV_AUTH'] = 'false'
        else:
            require(self.transport == 'local' and Path(self.target).is_absolute())
        self.work_root = Path(required('BACKUP_WORK_DIR'))
        require(self.work_root.is_absolute() and self.work_root.is_dir())
        if self.transport == 's3':
            # Plaintext is transient and must not land on persistent host disks.
            require(command(['findmnt', '-n', '-o', 'FSTYPE', '-T', str(self.work_root)]).strip() == b'tmpfs')
        self.retention = 0
        if mode == 'backup':
            days = required('BACKUP_RETENTION_DAYS')
            require(re.fullmatch(r'[1-9][0-9]?', days) and int(days) <= 90)
            self.retention = int(days)
            self.recipient = required('BACKUP_AGE_RECIPIENT')
            require(re.fullmatch(r'age1[0-9a-z]+', self.recipient))
        else:
            self.identity = Path(required('BACKUP_AGE_IDENTITY_FILE'))
            require(self.identity.is_absolute() and self.identity.is_file())

    def dc(self, *args, **kwargs):
        return command(self.compose + list(args), **kwargs)

    def sql(self, query):
        return self.dc('exec', '-T', 'postgres', 'psql', '-X', '-U', 'postgres',
                       '-d', 'postgres', '-At', '-v', 'ON_ERROR_STOP=1',
                       stdin=None, data=query.encode()).decode().strip()

    def rclone(self, *args):
        return command(['rclone', '--config', '/dev/null', '--quiet', '--retries', '1',
                        *map(str, args)], env=self.rclone_env)

    def cluster(self):
        require(self.sql("SELECT (SELECT count(*) FROM pg_database WHERE datname NOT IN "
                         "('postgres','template0','template1')) + (SELECT count(*) FROM pg_tablespace "
                         "WHERE spcname NOT IN ('pg_default','pg_global'));") == '0')
        container = self.dc('ps', '-q', 'postgres').decode().strip()
        require(container and '\n' not in container)
        self.image = command(['docker', 'inspect', '--format', '{{.Image}}', container]).decode().strip()
        require(re.fullmatch(r'sha256:[a-f0-9]{64}', self.image))
        self.pg_major = int(self.sql('SELECT current_setting(\'server_version_num\')::integer / 10000;'))

    def dump(self, output, *args):
        with output.open('wb') as file:
            self.dc('exec', '-T', 'postgres', *args, stdout=file)

    def uploads(self, *args, stdin=None, stdout=None):
        return command(['docker', 'run', '--rm', '-i', '--network', 'none',
                        '--mount', f'type=volume,src={self.volume},dst=/uploads' +
                        (',readonly' if self.mode == 'backup' else ''),
                        '--entrypoint', 'tar', self.image, *args], stdin=stdin, stdout=stdout)

    def backup(self, work):
        global step
        step = 'database_dump'
        self.cluster()
        command(['docker', 'volume', 'inspect', self.volume])  # Never auto-create a missing source.
        created = dt.datetime.now(dt.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')
        self.dump(work / 'database.dump', 'pg_dump', '-U', 'postgres', '-d', 'postgres', '-Fc')
        self.dump(work / 'roles.sql', 'pg_dumpall', '-U', 'postgres', '--globals-only', '--no-role-passwords')
        step = 'uploads_archive'
        with (work / 'uploads.tar.gz').open('wb') as file:
            self.uploads('-czf', '-', '-C', '/uploads', '.', stdout=file)
        regular_archive(work / 'uploads.tar.gz')
        step = 'manifest'
        manifest = {'format': 'moto-daily-backup-v1', 'created_at': created,
                    'pg_major': self.pg_major, 'database': 'postgres',
                    'sha256': {name: sha256(work / name) for name in FILES}}
        (work / 'manifest.json').write_text(json.dumps(manifest) + '\n')
        plain = work / 'bundle.tar'
        with tarfile.open(plain, 'w') as archive:
            for name in (*FILES, 'manifest.json'):
                archive.add(work / name, arcname=name, recursive=False)
        step = 'encryption'
        name = 'daily-' + created.replace('-', '').replace(':', '') + '-' + secrets.token_hex(8) + '.tar.age'
        encrypted = work / name
        command(['age', '-r', self.recipient, '-o', str(encrypted), str(plain)])
        step = 'upload'
        self.rclone('copyto', encrypted, self.target + '/' + name, '--immutable')
        step = 'remote_verification'
        downloaded = work / 'roundtrip.age'
        self.rclone('copyto', self.target + '/' + name, downloaded)
        require(sha256(downloaded) == sha256(encrypted))
        step = 'retention'
        # Only this flat, dedicated namespace. Do not delete unrelated objects.
        # Filename time, not mutable object mtime, is authoritative for age.
        cutoff = dt.datetime.now(dt.timezone.utc) - dt.timedelta(days=self.retention)
        objects = json.loads(self.rclone('lsjson', self.target, '--files-only', '--max-depth', '1'))
        for obj in objects:
            name = obj['Path']
            if re.fullmatch(BUNDLE_NAME, name):
                timestamp = dt.datetime.strptime(name[6:22], '%Y%m%dT%H%M%SZ').replace(tzinfo=dt.timezone.utc)
                if timestamp <= cutoff:
                    self.rclone('deletefile', self.target + '/' + name)
        remaining = json.loads(self.rclone('lsjson', self.target, '--files-only', '--max-depth', '1'))
        for obj in remaining:
            if re.fullmatch(BUNDLE_NAME, obj['Path']):
                timestamp = dt.datetime.strptime(obj['Path'][6:22], '%Y%m%dT%H%M%SZ').replace(tzinfo=dt.timezone.utc)
                require(timestamp > cutoff)

    def restore(self, work, name):
        global step
        require(re.fullmatch(BUNDLE_NAME, name))
        step = 'download'
        encrypted = work / 'bundle.age'
        self.rclone('copyto', self.target + '/' + name, encrypted)
        step = 'decryption'
        plain = work / 'bundle.tar'
        command(['age', '-d', '-i', str(self.identity), '-o', str(plain), str(encrypted)])
        step = 'checksums'
        regular_archive(plain, expected=(*FILES, 'manifest.json'))
        with tarfile.open(plain) as archive:
            # All members were validated as flat regular files; no extractall.
            for member in archive:
                with archive.extractfile(member) as source, (work / member.name).open('wb') as target:
                    shutil.copyfileobj(source, target)
        manifest = json.loads((work / 'manifest.json').read_text())
        require(manifest['format'] == 'moto-daily-backup-v1' and manifest['database'] == 'postgres')
        dt.datetime.strptime(manifest['created_at'], '%Y-%m-%dT%H:%M:%SZ')
        require(set(manifest['sha256']) == set(FILES))
        for name in FILES:
            require(sha256(work / name) == manifest['sha256'][name])
        regular_archive(work / 'uploads.tar.gz')
        step = 'fresh_target'
        require(self.dc('ps', '--status', 'running', '--services').decode().split() == ['postgres'])
        self.cluster()
        require(self.pg_major == manifest['pg_major'])
        require(self.sql("SELECT count(*) FROM pg_roles WHERE rolname !~ '^pg_' AND rolname <> 'postgres';") == '0')
        require(self.sql("SELECT count(*) FROM pg_namespace WHERE nspname NOT IN "
                         "('public','pg_catalog','information_schema','pg_toast') AND nspname !~ '^pg_temp_' "
                         "AND nspname !~ '^pg_toast_temp_';") == '0')
        require(self.sql("SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace "
                         "WHERE n.nspname='public';") == '0')
        listed = command(['docker', 'volume', 'ls', '--format', '{{.Name}}']).decode().splitlines()
        require(self.volume not in listed)
        # Durable handover barrier before the first write; never start any app.
        marker = self.compose_file.parent / '.daily-restore-review-required'
        with marker.open('x') as file:
            json.dump({'backup_created_at': manifest['created_at'], 'app_start': 'blocked',
                       'required': 'reapply_deletions_and_rotate_secrets'}, file)
        step = 'roles_restore'
        roles = (work / 'roles.sql').read_text()
        # Fresh initdb owns postgres already. All other duplicate roles are errors.
        roles = '\n'.join(line for line in roles.splitlines() if line != 'CREATE ROLE postgres;') + '\n'
        self.dc('exec', '-T', 'postgres', 'psql', '-X', '-U', 'postgres', '-d', 'postgres',
                '-v', 'ON_ERROR_STOP=1', data=roles.encode())
        step = 'database_restore'
        with (work / 'database.dump').open('rb') as file:
            self.dc('exec', '-T', 'postgres', 'pg_restore', '-U', 'postgres', '-d', 'postgres',
                    '--exit-on-error', stdin=file)
        step = 'uploads_restore'
        command(['docker', 'volume', 'create', self.volume])
        with (work / 'uploads.tar.gz').open('rb') as file:
            self.uploads('-xzf', '-', '-C', '/uploads', stdin=file)


def interrupted(_signum, _frame):
    raise InterruptedError('interrupted')


def main():
    global step
    os.umask(0o077)
    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    mode = sys.argv[1] if len(sys.argv) > 1 else 'backup'
    label = 'restore' if mode == 'restore' else 'backup'
    work = None
    lock = None
    status = 1
    missing = ''
    try:
        require((mode == 'backup' and len(sys.argv) == 2) or (mode == 'restore' and len(sys.argv) == 3))
        recovery = Recovery(mode)
        step = 'operation_lock'
        lock_path = recovery.compose_file.parent / '.release-operation.lock'
        lock_path.mkdir(mode=0o700)
        lock = lock_path
        work = Path(tempfile.mkdtemp(prefix='daily-', dir=recovery.work_root))
        if mode == 'backup':
            recovery.backup(work)
        else:
            recovery.restore(work, sys.argv[2])
        step = 'complete' if mode == 'backup' else 'deletion_review_required'
        status = 0
    except MissingConfiguration as error:
        missing = f' missing={error.args[0]}'
        status = 1
    except Exception:
        # No raw exceptions or subprocess output: they may contain personal data.
        status = 1
    finally:
        try:
            if work is not None:
                shutil.rmtree(work)
            if lock is not None:
                lock.rmdir()
        except Exception:
            step = 'cleanup'
            status = 1
    suffix = ' app_start=blocked' if mode == 'restore' else ''
    print(f'{label}_result={"ok" if status == 0 else "failed"} step={step}{missing}{suffix}', flush=True)
    return status


if __name__ == '__main__':
    sys.exit(main())
