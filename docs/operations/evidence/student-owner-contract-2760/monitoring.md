# Local old-access alert verification

Implemented a read-only PostgreSQL collector, atomic Node Exporter textfile
metrics, and Grafana rules for changed/missing/stale observations and SQL errors
naming retired objects. Installation instructions are in
`monitoring/runbooks/student-contract.md`. No production configuration changed.

## Database exercise

An isolated `student_contract_alert_check_2760` database was cloned from the
retained local schema-398 test template on PostgreSQL 17 with
`pg_stat_statements` preloaded. No production data was copied.

1. `observe.sh snapshot` captured a fresh baseline with zero view counters.
2. `observe.sh check` exited 0 and published observation status 1/read status 1.
3. Granted local fixture role `phoenix_admin` SELECT on its isolated archive,
   set that role and ran `SELECT count(*) FROM users.students_legacy`.
4. The following collector check exited 1 and published observation status
   0/read status 1. The fixed baseline remained unchanged. View counters stayed
   zero; direct-query statistics caused the failure.

The first exercise exposed PostgreSQL's JSON encoding of OIDs as strings; the
collector SQL now explicitly casts the database OID to bigint. The exercise
above passed after that correction.

## Automated checks

- `node --test scripts/student-contract-monitor.test.mjs`: 13 passed. Covers
  immutable baseline, healthy observations, direct-query fingerprint changes,
  view reads/writes, epoch/eviction changes, wrong database/cluster, partial
  removal, disabled tracking, failed/malformed reads, and missing baseline. These shell tests use
  a mocked Docker response; the database exercise above checks real SQL.
- Prometheus `prom/prometheus:v3.7.3` promtool: passed all four expression
  scenarios: healthy, old access, stopped collector, missing collector. The
  expression was extracted from the actual Grafana provisioning file and
  converted to a threshold-greater-than-zero Prometheus test rule.
  [Rule](monitoring-promtool-rules.yml), [inputs](monitoring-promtool-tests.yml),
  [output](monitoring-promtool.txt).
- Bash syntax, YAML parsing, environment synchronization and whitespace checks
  passed.

## Follow-up review and verification

Follow-up review found that readable statistics alone do not establish active
collection. Preflight and collector now require `pg_stat_statements.track=all`
and utility tracking, and reject narrowing role/database overrides. The dedicated local PostgreSQL
service was configured accordingly. The uncached `TestStudentContract` run
passed in 11.336 seconds, including a disabled-tracking transaction and a
database-scoped `phoenix_admin` override. Statistics remain readable in both
cases, but validation refuses the observation. See [raw results](tracking-tests.txt).
This detects the current configuration, not an administrator temporarily
changing it between observations; retain configuration-change evidence for the
whole window.

The complete `scripts/run-go-toolchain.sh scripts/test-changed.sh
origin/development` run passed for 166 affected backend packages. It started
before the tracking follow-up edits; a post-edit run is required separately.

The post-edit run exposed a timestamp-derived `requests_status_token_key`
collision in `createSplitRequestChildInPhase`. That helper now uses the existing
`testpkg.UniqueSuffix()` instead of `time.Now().UnixNano()`, without changing
assertions. Its complete package passed uncached in 7.905 seconds. The subsequent
full changed-test command passed for all 166 affected packages after all current
backend edits, with zero leftover clones. No frontend files changed.

`node scripts/student-contract-alert-rehearsal.mjs` then passed against a
disposable local Compose project with the exact checked-in Grafana rules:

1. Grafana 11.5.2 and Loki 3.4.3 became ready; both rules were provisioned.
2. Both alerts were healthy before synthetic failures were injected.
3. The actual LogQL expression matched exactly three missing-old-object errors,
   excluding an unrelated error and a normal log line.
4. Both Grafana alerts changed to firing with healthy evaluation.
5. Both firing notifications arrived at the container-local webhook sink.
6. The test removed its own Compose services and network.

The collector metric was synthetic in this alert-path exercise; the earlier
PostgreSQL exercise separately verified real collector detection. No production
contact point was used. See [rehearsal output](alert-rehearsal.txt).

Production scheduler,
textfile scrape, Loki labels and notification delivery must be verified before
claiming the operational window is monitored. Local green checks do not prove
production monitoring coverage or approve a rollback duration.
