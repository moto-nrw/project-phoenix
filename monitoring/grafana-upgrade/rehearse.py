"""Disposable Docker Compose migration and full-volume rollback verification."""
import base64
import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

HERE = Path(__file__).resolve().parent


def main():
    endpoint = subprocess.check_output(
        ['docker', 'context', 'inspect', '--format', '{{.Endpoints.docker.Host}}'], text=True
    ).strip()
    if not endpoint.startswith('unix://') or os.environ.get('DOCKER_HOST') or os.environ.get('DOCKER_CONTEXT'):
        raise SystemExit('Use a local Docker context without DOCKER_HOST/DOCKER_CONTEXT overrides')
    with tempfile.TemporaryDirectory(prefix='grafana-rehearsal-') as directory:
        root = Path(directory)
        project = 'grafana-rehearsal-' + secrets.token_hex(4)
        password = secrets.token_urlsafe(24)
        shutil.copytree(HERE.parent / 'grafana/provisioning', root / 'provisioning')
        # Grafana scans this optional directory even when no apps are provisioned.
        (root / 'provisioning/plugins').mkdir(exist_ok=True)
        (root / 'prometheus.yml').write_text('global:\n  scrape_interval: 5s\nscrape_configs:\n  - job_name: fixture\n    static_configs:\n      - targets: ["prometheus:9090"]\n')
        (root / 'loki.yml').write_text('''auth_enabled: false
server:
  http_listen_port: 3100
common:
  path_prefix: /tmp/loki
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory
  storage:
    filesystem:
      chunks_directory: /tmp/loki/chunks
      rules_directory: /tmp/loki/rules
schema_config:
  configs:
    - from: 2024-01-01
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h
''')
        config = {'services': {
            'grafana': {
                'image': 'grafana/grafana:11.5.2', 'platform': 'linux/amd64',
                'ports': ['127.0.0.1::3000'],
                'environment': {'GF_SECURITY_ADMIN_USER': 'rehearsal', 'GF_SECURITY_ADMIN_PASSWORD': password,
                    'GF_AUTH_ANONYMOUS_ENABLED': 'false', 'GF_USERS_ALLOW_SIGN_UP': 'false',
                    'GF_ANALYTICS_REPORTING_ENABLED': 'false', 'GF_ANALYTICS_CHECK_FOR_UPDATES': 'false'},
                'volumes': ['./provisioning:/etc/grafana/provisioning:ro', 'data:/var/lib/grafana']},
            'prometheus': {'image': 'prom/prometheus:v3.7.3', 'volumes': ['./prometheus.yml:/etc/prometheus/prometheus.yml:ro']},
            'loki': {'image': 'grafana/loki:3.4.3', 'command': ['-config.file=/etc/loki/fixture.yml'],
                     'volumes': ['./loki.yml:/etc/loki/fixture.yml:ro']}},
            'volumes': {'data': {}}}
        compose_file = root / 'compose.json'
        compose_file.write_text(json.dumps(config))
        compose_file.chmod(0o600)

        def compose(*args, overlay=None, capture=False):
            command = ['docker', 'compose', '--project-name', project, '--env-file', '/dev/null', '-f', str(compose_file)]
            if overlay:
                command += ['-f', str(overlay)]
            return subprocess.run(command + list(args), check=True, text=True,
                                  stdout=subprocess.PIPE if capture else None).stdout

        base_url = ''
        authorization = 'Basic ' + base64.b64encode(('rehearsal:' + password).encode()).decode()

        def api(path, body=None, method=None, auth=True):
            headers = {'Content-Type': 'application/json'}
            if auth:
                headers['Authorization'] = authorization
            request = urllib.request.Request(base_url + path, headers=headers,
                data=json.dumps(body).encode() if body is not None else None, method=method)
            with urllib.request.urlopen(request, timeout=10) as response:
                return json.load(response)

        def ready(version):
            nonlocal base_url
            base_url = 'http://' + compose('port', 'grafana', '3000', capture=True).strip()
            for _ in range(120):
                try:
                    health = api('/api/health', auth=False)
                    if health['database'] == 'ok' and health['version'] == version:
                        return
                except (OSError, ValueError):
                    pass
                time.sleep(1)
            raise RuntimeError('Grafana failed readiness: ' + version)

        def check(version):
            ready(version)
            jar = http.cookiejar.CookieJar()
            opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
            request = urllib.request.Request(base_url + '/login',
                data=json.dumps({'user': 'rehearsal', 'password': password}).encode(),
                headers={'Content-Type': 'application/json'})
            with opener.open(request, timeout=10) as response:
                assert response.status == 200
            with opener.open(base_url + '/api/user', timeout=10) as response:
                assert json.load(response)['login'] == 'rehearsal'
            try:
                api('/api/search', auth=False)
                raise AssertionError('Anonymous API access allowed')
            except urllib.error.HTTPError as error:
                assert error.code == 401
            settings = api('/api/admin/settings')
            assert settings['auth.anonymous']['enabled'] == 'false'
            assert settings['users']['allow_sign_up'] == 'false'
            expected_plugins = {'grafana-lokiexplore-app': '1.0.10', 'grafana-pyroscope-app': '1.17.0'} if version == '11.5.2' else {'grafana-lokiexplore-app': '2.5.2', 'grafana-pyroscope-app': '2.3.0'}
            for plugin, expected_version in expected_plugins.items():
                metadata = api('/api/plugins/' + plugin + '/settings')
                assert metadata['info']['version'] == expected_version
                assert metadata['signature'] == 'valid'
            dashboards = api('/api/search?type=dash-db')
            expected = {json.loads(p.read_text())['title'] for p in (root / 'provisioning/dashboards/json').glob('*.json')}
            assert expected <= {d['title'] for d in dashboards}
            assert api('/api/dashboards/uid/rehearsal-persisted')['dashboard']['title'] == 'Migration fixture'
            for uid in ['prometheus', 'P8E80F9AEF21F6940']:
                for attempt in range(60):
                    try:
                        assert api('/api/datasources/uid/' + uid + '/health')['status'] == 'OK'
                        break
                    except (OSError, AssertionError):
                        if attempt == 59:
                            raise
                        time.sleep(1)
            rules = api('/api/v1/provisioning/alert-rules')
            assert {r['uid'] for r in rules} == {'booking-audit-technical-failure', 'booking-audit-missing-run',
                'booking-audit-drift-increase', 'server-error-spike', 'postgres-error-lines'}
            assert any(c['uid'] == 'rehearsal-sink' for c in api('/api/v1/provisioning/contact-points'))
            assert api('/api/v1/provisioning/policies')['receiver'] == 'Rehearsal sink'
            logs = compose('logs', '--no-color', 'grafana', capture=True)
            failures = [line for line in logs.splitlines() if 'level=error' in line and
                        any(logger in line for logger in ['logger=migrator', 'logger=sqlstore', 'logger=provisioning'])]
            if failures:
                raise RuntimeError('Migration/provisioning errors in disposable Grafana: ' + '\n'.join(failures))
            print('PASS', version, 'session login, auth settings, signed plugin versions, dashboards, datasource health, rules, contact point and policy', flush=True)
            return {r['uid']: r['data'] for r in rules}

        try:
            compose('config', '--quiet')
            for overlay in ['intermediate.yml', 'target.yml']:
                compose('config', '--quiet', overlay=HERE / overlay)
            for plugin, version in [('grafana-lokiexplore-app', '1.0.10'), ('grafana-pyroscope-app', '1.17.0')]:
                compose('run', '--rm', '--no-deps', '--entrypoint', 'grafana', 'grafana',
                        'cli', '--pluginsDir', '/var/lib/grafana/plugins', 'plugins', 'install', plugin, version)
            compose('up', '-d')
            ready('11.5.2')
            api('/api/dashboards/db', {'dashboard': {'uid': 'rehearsal-persisted', 'title': 'Migration fixture',
                'schemaVersion': 39, 'panels': []}, 'overwrite': False})
            # Loopback discard destination, never a real email or webhook.
            api('/api/v1/provisioning/contact-points', {'uid': 'rehearsal-sink', 'name': 'Rehearsal sink',
                'type': 'webhook', 'settings': {'url': 'http://127.0.0.1:9'}, 'disableResolveMessage': False})
            api('/api/v1/provisioning/policies', {'receiver': 'Rehearsal sink', 'group_by': ['grafana_folder', 'alertname']}, method='PUT')
            baseline = check('11.5.2')
            compose('stop', 'grafana')
            compose('run', '--rm', '--no-deps', '--user', '0', '--entrypoint', 'sh',
                    '-v', str(root) + ':/backup', 'grafana', '-ec',
                    'tar -czf /backup/data.tar.gz -C /var/lib/grafana .')
            for name, version in [('intermediate.yml', '12.4.10'), ('target.yml', '13.2.1')]:
                compose('up', '-d', '--no-deps', 'grafana', overlay=HERE / name)
                assert check(version) == baseline, 'Alert expressions changed'
            compose('stop', 'grafana', overlay=HERE / 'target.yml')
            compose('run', '--rm', '--no-deps', '--user', '0', '--entrypoint', 'sh',
                    '-v', str(root) + ':/backup', 'grafana', '-ec',
                    'find /var/lib/grafana -mindepth 1 -maxdepth 1 -exec rm -rf {} \\; ; tar -xzf /backup/data.tar.gz -C /var/lib/grafana')
            compose('up', '-d', '--no-deps', 'grafana')
            assert check('11.5.2') == baseline
            print('PASS full-volume rollback to 11.5.2', flush=True)
        finally:
            # Remove only the random project created by this invocation.
            compose('down', '--volumes', '--remove-orphans')


if __name__ == '__main__':
    main()
