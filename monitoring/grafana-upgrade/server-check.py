"""Run over SSH on the authorized server; keep baseline and credentials server-local."""
import base64
import collections
import datetime
import json
import os
import re
from pathlib import Path
import subprocess
import sys
import urllib.error
import urllib.request

os.umask(0o077)
mode = sys.argv[1]
container = json.loads(subprocess.check_output(['docker', 'inspect', 'monitoring-grafana-1']))[0]
env = dict(value.split('=', 1) for value in container['Config']['Env'])
headers = {'Authorization': 'Basic ' + base64.b64encode(
    (env['GF_SECURITY_ADMIN_USER'] + ':' + env['GF_SECURITY_ADMIN_PASSWORD']).encode()).decode()}


def get(path, authenticated=True):
    request = urllib.request.Request('http://127.0.0.1:3003' + path,
                                     headers=headers if authenticated else {})
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


state = {key: get(path) for key, path in {
    'dashboards': '/api/search?type=dash-db&limit=5000',
    'rules': '/api/v1/provisioning/alert-rules',
    'contacts': '/api/v1/provisioning/contact-points',
    'policies': '/api/v1/provisioning/policies',
    'datasources': '/api/datasources',
}.items()}
state['dashboard_content'] = {d['uid']: get('/api/dashboards/uid/' + d['uid'])['dashboard']
                              for d in state['dashboards']}
containers = json.loads(subprocess.check_output(['docker', 'inspect'] +
    subprocess.check_output(['docker', 'ps', '-q'], text=True).split()))
state['containers'] = {d['Name']: {'id': d['Id'], 'started': d['State']['StartedAt']} for d in containers}
pointer = Path('/root/monitoring/grafana-upgrade/backup-path')

if mode == 'baseline':
    root = Path('/root/monitoring/backups') / ('grafana-' +
        datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ'))
    root.mkdir(mode=0o700, parents=True)
    (root / 'baseline.json').write_text(json.dumps(state))
    (root / 'image-id').write_text(container['Image'] + '\n')
    pointer.write_text(str(root) + '\n')
    print('Backup directory:', root)
else:
    root = Path(pointer.read_text().strip())
    baseline = json.loads((root / 'baseline.json').read_text())
    (root / ('observed-' + mode + '.json')).write_text(json.dumps(state))
    health = get('/api/health', False)
    assert health['database'] == 'ok' and health['version'] == mode
    assert set(state['dashboard_content']) == set(baseline['dashboard_content'])
    # Grafana can update schema metadata during migration; retain dashboard behavior.
    for uid, dashboard in state['dashboard_content'].items():
        previous = baseline['dashboard_content'][uid]
        for key in ['title', 'panels', 'templating', 'annotations', 'time', 'tags', 'links']:
            assert dashboard.get(key) == previous.get(key), 'Dashboard field changed: ' + uid + '/' + key
    rule_fields = ['uid', 'title', 'condition', 'data', 'noDataState', 'execErrState',
                   'for', 'labels', 'annotations', 'isPaused', 'folderUID', 'ruleGroup']
    def rules(items):
        return {r['uid']: {key: r.get(key) for key in rule_fields} for r in items}
    assert rules(state['rules']) == rules(baseline['rules']), 'Rule configuration changed'
    assert state['policies'] == baseline['policies'], 'Notification policy changed'
    assert sorted(state['contacts'], key=lambda c: c['uid']) == sorted(baseline['contacts'], key=lambda c: c['uid']), 'Contact points changed'
    def sources(items):
        # Grafana 12 relocates built-in plugin icons; this is not source configuration.
        return {s['uid']: {k: v for k, v in s.items() if k != 'typeLogoUrl'} for s in items}
    assert sources(state['datasources']) == sources(baseline['datasources']), 'Data-source configuration changed'
    for source in state['datasources']:
        assert get('/api/datasources/uid/' + source['uid'] + '/health')['status'] == 'OK'
    settings = get('/api/admin/settings')
    assert settings['auth.anonymous']['enabled'] == 'false'
    assert settings['users']['allow_sign_up'] == 'false'
    for plugin, version in [('grafana-lokiexplore-app', '2.5.2'), ('grafana-pyroscope-app', '2.3.0')]:
        metadata = get('/api/plugins/' + plugin + '/settings')
        assert metadata['info']['version'] == version and metadata['signature'] == 'valid'
    try:
        get('/api/search', False)
        raise AssertionError('Anonymous access allowed')
    except urllib.error.HTTPError as error:
        assert error.code == 401
    with urllib.request.urlopen('http://127.0.0.1:3003/login', timeout=15) as response:
        assert response.status == 200
    assert container['HostConfig']['PortBindings']['3000/tcp'] == [{'HostIp': '127.0.0.1', 'HostPort': '3003'}]
    assert any(m.get('Name') == 'monitoring_grafana-data' and m['Destination'] == '/var/lib/grafana' for m in container['Mounts'])
    assert any(m['Source'] == '/root/monitoring/grafana/provisioning' and not m['RW'] for m in container['Mounts'])
    for name, previous in baseline['containers'].items():
        if name != '/monitoring-grafana-1':
            assert state['containers'].get(name) == previous, 'Unrelated container changed: ' + name
    logs = subprocess.check_output(['docker', 'logs', 'monitoring-grafana-1'], stderr=subprocess.STDOUT, text=True)
    (root / ('startup-' + mode + '.log')).write_text(logs)
    errors = [line for line in logs.splitlines() if 'level=error' in line]
    print('Startup error counts by logger:', dict(collections.Counter(
        re.search(r'logger=([^ ]+)', line).group(1) if 'logger=' in line else 'other' for line in errors)))
    failures = [line for line in errors if any(key in line for key in
        ['logger=migrator', 'logger=sqlstore', 'logger=provisioning']) and
        'Failed to read plugin provisioning files from directory' not in line]
    assert not failures, 'Migration/provisioning errors; inspect private startup log'
    evaluation = get('/api/prometheus/grafana/api/v1/rules')
    (root / ('evaluation-' + mode + '.json')).write_text(json.dumps(evaluation))
    evaluated_rules = [rule for group in evaluation['data']['groups'] for rule in group['rules']]
    print('Rule evaluation health:', dict(collections.Counter(rule.get('health') for rule in evaluated_rules)))
    assert not any(rule.get('health') == 'error' for rule in evaluated_rules), 'Alert evaluation errors'
    print('PASS', mode, 'inventory/content, data-source health, auth, plugins, mounts, unrelated containers')

print('Inventory:', len(state['dashboards']), 'dashboards,', len(state['rules']),
      'rules,', len(state['datasources']), 'data sources,', len(state['contacts']), 'contact points')
