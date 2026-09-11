# Grafana upgrade verification

Date: 2026-09-05. Scope: implementation, disposable rehearsal, and explicitly
authorized live deployment. Grafana 13.2.1 is deployed and server-local checks
passed. Browser/proxy checks and real notification delivery remain unverified.

## Gate 1: configuration and patch

- `docker compose ... config --quiet`: passed for the local base, checkpoint
  and target; passed against the authoritative server configuration over SSH.
- `patch --dry-run --batch --fuzz=0 -p1`: passed against the server-local base.
  No server file was written.
- Compared parsed before/after Compose in memory on the server. Only the image,
  explicit false anonymous/sign-up settings and pinned app plugins differ.
  Ports, volume, mount, root URL, credentials, networks and all other services
  are unchanged. No expanded secret-bearing configuration was printed or copied.
- Applied the patch in memory and parsed the result with Compose: it exactly
  matches the target overlay. This verifies the persistent authoritative patch,
  not just a disposable override.
- `git diff --check` and `git diff --cached --check`: passed.

## Gate 2: security substitute

Docker Hub resolved the exact target and checkpoint digests recorded in the
runbook. Official advisories and support guidance informed selection of 13.2.1.
The rehearsal's `/api/health` verifies the actual running version after each
migration. Unauthenticated `/api/search` returns 401; effective settings keep
anonymous access and public sign-up false.

No crafted-link XSS reproduction was attempted. The original issue is addressed
by replacing the affected binary with a vendor-patched version in the rehearsal,
not by claiming a successful exploitation attempt followed by a failed one.
The live instance subsequently received the same pinned 13.2.1 image after
explicit owner authorization; its image digest and health version were checked.

## Gate 3: legitimate behavior and rollback

`python3 monitoring/grafana-upgrade/rehearse.py` exercises:

1. Grafana 11.5.2 with the two original plugin versions, repository provisioning,
   a synthetic login, a database-created dashboard, contact point and policy.
2. Migration to 12.4.10 with pinned plugin upgrades, then 13.2.1. Checks verify
   cookie login, signed plugin versions, two provisioned dashboards plus the
   database-created dashboard, both data-source health endpoints, five alert
   rules with unchanged expressions, and the contact point/policy.
3. Cold backup of the complete fixture volume, full restoration after the
   target migration, then all the same checks on the old image and old plugins.
   This is a database-and-plugin rollback, not a binary-only downgrade.

The fixture creates an empty optional `provisioning/plugins` directory to avoid
Grafana's missing-directory startup error. Production provisioning is unchanged;
an equivalent pre-existing warning there is not a new migration failure.
The script checks startup logs for migration, SQL-store and provisioning errors.
All fixture services and volumes are removed after each run.

## Review and coverage limits

A separate read-only reviewer checked the candidate patch and rollback process.
The runbook now creates the backup parent directory if absent. No other
actionable defect was reported. No application code or environment schema
changed, so backend/frontend test suites and app environment-sync checks are
not relevant to this patch.

During preparation, no live service, account, Caddy config or provisioning file
was modified. The authorized release below subsequently restarted only Grafana.

## Authorized live release

Owner authorization: “then yes deploy it please”. The authoritative server base
was patched after the 12.4.10 checkpoint passed, then Grafana moved to 13.2.1.
No Caddy, application or other monitoring container was restarted. Container
identities/start times and Caddy's ActiveEnterTimestamp were compared.

Private cold backup and baseline:
`/root/monitoring/backups/grafana-20260904T225853Z`.
The UTC suffix is September 4; the local release date is September 5 (Berlin).
Full volume, configuration and old-image archives passed SHA-256 checks after
release. Roughly 7.1 GB remained free on the server filesystem.

`server-check.py` ran over SSH at both versions. Credentials stayed in server
memory; baselines and observations stayed in the root-only backup directory.
Results:

- Exact target RepoDigest and `/api/health`: 13.2.1, database OK.
- Seven dashboard UIDs and functional content unchanged. The earlier count of
  eight came from the legacy dashboard table, which includes folders.
- All sixteen alert rules retain expressions, conditions, labels, annotations,
  paused states and folder/group placement. All sixteen evaluation health values
  report `ok` after the target upgrade.
- Both data sources retain configuration and return healthy. Only the vendor
  icon asset paths changed in the API response, so comparison excludes that
  display-only metadata field.
- Contact point and notification policy unchanged. No test notification sent.
- Both pinned plugin versions have valid signatures. Anonymous access/sign-up
  remain false; authenticated API access succeeds, unauthenticated search gets
  401, and local `/login` returns 200.
- Same persistent volume, read-only provisioning mount and loopback binding.
  Startup logs have no migration/SQL-store/provisioning failures beyond the
  existing missing optional plugin-provisioning-directory warning.

The public-hostname HTTPS check using local resolution was rejected by automatic
approval review: its rule permits only a human to target production hostnames.
No alternate-host bypass was attempted. Browser rendering, end-to-end proxy
login, CSP compatibility and actual notification delivery remain unverified.
