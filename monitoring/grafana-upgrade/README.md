# Grafana upgrade and rollback

Prepared and deployed on 2026-09-05 with explicit owner authorization.
**Grafana 13.2.1 is live.** See [verification.md](verification.md) for results
and the remaining browser/proxy check. This change does not alter Caddy,
application services, data-source URLs, dashboards or alert rules.

## Ownership and observed layout

The server owner maintains `/root/monitoring/docker-compose.yml` outside Git.
The existing `monitoring/README.md` identifies that file as authoritative;
`prometheus-compose.yml` is additive. Do not replace the base with a copied
repository file or commit the server's `.env` or expanded Compose output.

Read-only inspection confirmed:

| Item | Observed value |
| --- | --- |
| Grafana | `grafana/grafana:11.5.2`, healthy |
| Compose project | `monitoring` |
| Compose sources | `/root/monitoring/docker-compose.yml`, `/root/monitoring/prometheus-compose.yml` |
| Persistent volume | `monitoring_grafana-data` at `/var/lib/grafana` |
| Host volume path | `/var/lib/docker/volumes/monitoring_grafana-data/_data` |
| Database | SQLite, `grafana.db`, 1,650,688 bytes |
| Provisioning mount | `/root/monitoring/grafana/provisioning` mounted read-only |
| Listener | `127.0.0.1:3003` to container port 3000 |
| Root URL | `https://grafana.moto-app.de` |
| Inventory | 8 legacy dashboard-table rows; release API baseline: 7 dashboards, 16 alert rules, 2 data sources, 1 contact point |
| Plugins | `grafana-lokiexplore-app` 1.0.10; `grafana-pyroscope-app` 1.17.0 |
| Auth / CSP | Anonymous access, sign-up and CSP disabled in effective file defaults; no environment overrides |
| Disk | 25 MB Grafana volume, 9.6 GB free on its filesystem |

The live provisioning directory contains seven dashboard JSON files and only
the booking-consistency alert file. The repository contains two dashboard files
and five alert rules. **Do not sync or delete live provisioning in this release.**
Database-managed dashboards, rules, notification policies and contact points
must survive through the existing volume. No production login was attempted.

## Version decision

Target: `grafana/grafana:13.2.1@sha256:f772d434e8fab0049deb2b1b30abd43342bcfca1537614aa8d36080232cf4283`.
The manifest's linux/amd64 digest is
`sha256:1dec240d14e232597dce9bfa56dae55f4397b138cdc91e3ee92ac6b157e2fc49`.

Intermediate checkpoint:
`grafana/grafana:12.4.10@sha256:c132a683b2430fff9115a29b2a79c8ab97540cdcc90846e3c81878c778ca3596`.
Both manifest lists were resolved from Docker Hub. The target was released
September 1, 2026; the 13.2 branch receives patch support until May 18, 2027.
See [official download](https://grafana.com/grafana/download?edition=oss) and
[support policy](https://grafana.com/docs/grafana/latest/upgrade-guide/when-to-upgrade/).

Reviewed the [vendor advisory index](https://grafana.com/security/security-advisories/)
through September 2, 2026. Relevant fixed-version references include:

- [CVE-2025-4123](https://grafana.com/security/security-advisories/cve-2025-4123/): original frontend XSS finding; target supersedes its fixed releases.
- [CVE-2026-17033](https://grafana.com/security/security-advisories/cve-2026-17033/): Alertmanager XSS, fixed from 13.1.0. Do not treat the 12.4 checkpoint as the final remediation.
- [CVE-2026-19197](https://grafana.com/security/security-advisories/cve-2026-19197/): snapshot access control, fixed in the preceding supported branches before 13.2.1.
- [CVE-2026-19475](https://grafana.com/security/security-advisories/cve-2026-19475/): SQL macro DoS, September patch cycle. The observed data sources are Loki and Prometheus, not SQL.

This is version-based remediation, not proof of compromise or an exploit test.
Recheck advisories before a delayed deployment.

## Migration and compatibility

Use **11.5.2 → 12.4.10 → 13.2.1**. The intermediate checkpoint is a conservative
validation step, not a claim that every intervening minor release is mandatory.
Grafana's [v13 guide](https://grafana.com/docs/grafana/latest/upgrade-guide/upgrade-v13.0/)
calls for a current pre-v13 version and updated plugins before React 19.
The two existing app plugins are pinned to `2.5.2` and `2.3.0` respectively,
installed synchronously on the checkpoint, then retained at the target.
Their vendor-declared Grafana compatibility includes 12.4 and 13.2:
[Logs Drilldown metadata](https://grafana.com/api/plugins/grafana-lokiexplore-app/versions/2.5.2),
[Profiles Drilldown metadata](https://grafana.com/api/plugins/grafana-pyroscope-app/versions/2.3.0).

The [v12 guide](https://grafana.com/docs/grafana/latest/upgrade-guide/upgrade-v12.0/)
requires valid data-source UIDs and warns of annotation-table rewrite space.
Both observed UIDs are valid. Reserve at least three times the SQLite file size
plus space for images, the cold backup and migration temporary files. Recheck
disk immediately before release. No annotation cleanup is part of this change.

Grafana 13 migrates folders/dashboards to unified storage. **A binary downgrade
does not roll back that state. Restore the complete cold volume and old config.**
Avoid 13.0.0, which had a Git Sync migration defect. No custom entrypoint,
image-renderer plugin or non-core dashboard panel type was observed. The
rehearsal uses UID-based data-source APIs, not the removed numeric-ID API.

CSP remains unchanged. Enabling it needs a separate browser compatibility pass
covering both app plugins, dashboards and Explore behind the actual proxy.
No claim is made that CSP replaces upgrading or that browser rendering has
been verified by the API rehearsal.

## Rehearsal

From the repository root, with Python 3 and a local Docker daemon:

```bash
python3 monitoring/grafana-upgrade/rehearse.py
```

The script uses Docker Compose with a random project name, a loopback-only
ephemeral port, disposable volumes, synthetic credentials, local Prometheus
and Loki fixtures, and a notification destination at container loopback port 9.
It does not access the server or production credentials. It downloads official
images/plugins, requires local Unix-socket Docker, and removes its project on exit.

Checks cover cookie-session login, protected-API rejection without auth,
anonymous/sign-up settings, signed plugin versions, two provisioned dashboards,
a database-created dashboard, both data-source health APIs, all five repository
alert rules and unchanged expressions, a database-created contact point and
notification policy, and full-volume restoration to 11.5.2.

These fixtures do not prove migration of every live rule or dashboard, real
notification delivery, app-plugin browser rendering, or production performance.
Those remain release checks. No crafted-link exploit is sent anywhere.

## Explicit release step: server owner only

**Do not run this section until live rollout is authorized.** Commands below
use Bash on the server. Freeze Grafana edits during the window. Preserve the
backup directory until the owner accepts the release. Never run Compose `down`
against the live monitoring project.

### 1. Stage and validate

Copy this directory to `/root/monitoring/grafana-upgrade` without changing the
base or provisioning. In a server Bash shell:

```bash
set -euo pipefail
cd /root/monitoring
dc=(docker compose -p monitoring -f docker-compose.yml -f prometheus-compose.yml)
checkpoint=("${dc[@]}" -f grafana-upgrade/intermediate.yml)
target=("${dc[@]}" -f grafana-upgrade/target.yml)
"${dc[@]}" config --quiet
"${checkpoint[@]}" config --quiet
"${target[@]}" config --quiet
patch --dry-run --batch --fuzz=0 -p1 < grafana-upgrade/base.patch
docker inspect monitoring-grafana-1 --format '{{.Config.Image}} {{json .Mounts}} {{json .HostConfig.PortBindings}}'
df -h /var/lib/docker
du -sh /var/lib/docker/volumes/monitoring_grafana-data/_data
"${checkpoint[@]}" pull grafana
"${target[@]}" pull grafana
```

Abort on layout drift or patch failure. Do not print expanded Compose config.
Before stopping, record the dashboard/rule/data-source inventory and contact
point/policy configuration through the owner's existing authenticated session,
keeping any exports private on the server. Record paused rule states too.

### 2. Cold backup

```bash
umask 077
backup_dir="/root/monitoring/backups/grafana-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p /root/monitoring/backups
mkdir -m 700 "$backup_dir"
docker inspect monitoring-grafana-1 --format '{{.Image}}' > "$backup_dir/image-id"
docker image save "$(cat "$backup_dir/image-id")" | gzip > "$backup_dir/old-image.tar.gz"
"${dc[@]}" stop grafana
test "$(docker inspect monitoring-grafana-1 --format '{{.State.Running}}')" = false
tar -czpf "$backup_dir/config.tar.gz" docker-compose.yml prometheus-compose.yml .env grafana/provisioning
tar -czpf "$backup_dir/data.tar.gz" -C /var/lib/docker/volumes/monitoring_grafana-data/_data .
tar -tzf "$backup_dir/data.tar.gz" > /dev/null
(cd "$backup_dir" && sha256sum config.tar.gz data.tar.gz old-image.tar.gz > SHA256SUMS)
printf 'Backup directory: %s\n' "$backup_dir"
```

If backup fails, restart the unchanged old Grafana and stop the release.
The archives contain credentials and must stay root-only. Back up the whole
volume, not just `grafana.db`, to retain plugin files and encryption state.

### 3. Checkpoint and target

```bash
"${checkpoint[@]}" up -d --no-deps grafana
curl --fail --silent --show-error http://127.0.0.1:3003/api/health
```

Wait until health reports **12.4.10** and `database: ok`. Review startup logs
privately for migration/provisioning/plugin errors. Check the inventory and
plugin versions through the owner's session. If anything is missing, roll back.

After the checkpoint passes, persist the target in the authoritative base so a
future two-file Compose invocation cannot revert to the old image:

```bash
patch --batch --fuzz=0 -p1 < grafana-upgrade/base.patch
"${dc[@]}" config --quiet
"${dc[@]}" up -d --no-deps grafana
curl --fail --silent --show-error http://127.0.0.1:3003/api/health
```

Health must report **13.2.1**. Run the checks below before accepting the release.
No other monitoring service or Caddy is restarted.

### 4. Rollback after either migration

Use the same Bash session and its recorded `backup_dir`, or explicitly set it
to the verified backup directory. This discards Grafana changes made after the
backup. Do not restore onto a running database.

```bash
: "${backup_dir:?Set the recorded Grafana backup directory}"
(cd "$backup_dir" && sha256sum --check SHA256SUMS)
tar -tzf "$backup_dir/data.tar.gz" > /dev/null
"${dc[@]}" stop grafana
test "$(docker inspect monitoring-grafana-1 --format '{{.State.Running}}')" = false
test "$(docker inspect monitoring-grafana-1 --format '{{range .Mounts}}{{if eq .Destination "/var/lib/grafana"}}{{.Name}}{{end}}{{end}}')" = monitoring_grafana-data
# Keep the failed state for investigation before restoring the cold archive.
tar -czpf "$backup_dir/failed-data-$(date -u +%Y%m%dT%H%M%SZ).tar.gz" -C /var/lib/docker/volumes/monitoring_grafana-data/_data .
find /var/lib/docker/volumes/monitoring_grafana-data/_data -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
tar -xzpf "$backup_dir/data.tar.gz" -C /var/lib/docker/volumes/monitoring_grafana-data/_data
tar -xzpf "$backup_dir/config.tar.gz" -C /root/monitoring
gzip -dc "$backup_dir/old-image.tar.gz" | docker image load
docker image tag "$(cat "$backup_dir/image-id")" grafana/grafana:11.5.2
"${dc[@]}" config --quiet
"${dc[@]}" up -d --no-deps --pull never grafana
curl --fail --silent --show-error http://127.0.0.1:3003/api/health
```

Verify the old version and pre-release inventory. A rollback restores service
but also restores the vulnerable version; the remediation remains open.

## Post-release read-only checklist

The deployed release passed server-local image, health, login-page,
authenticated API, inventory/content, data-source health, settings, plugin and
alert-evaluation checks. All 16 evaluated rules report `ok`. Browser/proxy
validation and real notification delivery remain unverified. Automatic approval
review blocked the production-hostname HTTPS check, including local resolution.

1. Inspect container image and image RepoDigests; require the pinned 13.2.1
   image and `/api/health` with `database: ok`. Confirm the port remains
   `127.0.0.1:3003`, the same volume and read-only provisioning mount remain.
2. Fetch local `/login` successfully and require 401 from unauthenticated
   `/api/search`. Through the owner's session, verify anonymous access and
   public sign-up are false in `/api/admin/settings` and existing login works.
3. Compare all dashboard UIDs/content, rule UIDs/expressions/paused states,
   data-source UIDs, contact points and notification policy against the private
   baseline. Confirm both data-source health checks and normal alert evaluation.
4. Review startup logs for migration, provisioning and plugin failures. Open
   dashboards, Explore and the two app plugins in the owner's browser. Check
   ordinary notification delivery without sending synthetic production alerts.

The release is not complete until these checks pass. Record evidence privately;
do not attach credentials, contact-point destinations or raw database exports.
