# Student storage Contract observation

The collector uses Docker's local socket and container-local `psql` as the
inspection superuser. It executes only a read-only transaction against catalog,
sequence and query statistics. No student rows, SQL text, or credentials are
exported. Required host tools: Bash, Docker, jq, and standard core utilities.

## Installation before starting the rollback window

Query coverage requires `pg_stat_statements` preloaded and
`pg_stat_statements.track=all` and `pg_stat_statements.track_utility=on`, without role/database overrides disabling or
narrowing it. Configure this before the window, not just for the final check;
retain the configuration and change records with the observation evidence.
Point-in-time checks cannot prove that an administrator never temporarily
disabled and re-enabled tracking between samples.

1. Copy `student-contract/observe.sh` and `observe.sql` together to the monitoring
   directory. Create a private baseline directory and `node-exporter-textfile`.
2. Explicitly capture a baseline, retaining its timestamp and operator review:

   ```sh
   bash student-contract/observe.sh snapshot production-postgres-1 postgres \
     /root/monitoring/student-contract-baseline.json \
     /root/monitoring/node-exporter-textfile/student-contract.prom
   ```

   This refuses nonzero compatibility counters, missing compatibility sequences,
   unavailable statistics, or an existing baseline file. It does not prove that
   historical statements have zero post-cutover calls. Review the raw query
   inventory separately, including external jobs and superuser maintenance.
3. Schedule the same command with `check` instead of `snapshot` every minute
   using the host's existing scheduler. Use absolute script and output paths.
   Never automatically overwrite or rotate the baseline on failure. Retain
   scheduler execution records as window evidence.
4. Apply the Node Exporter textfile mount/collector option from
   `prometheus-compose.yml`, and provision `alerting/student-contract.yml` in
   Grafana. Existing Prometheus node scraping and alert routing are reused.
5. Verify the metric in Prometheus, the rule in Grafana, and actual notification
   delivery to the existing contact point. An absent collector or samples older
   than 180 seconds cause an alert. Do not declare monitoring operational until
   this deployment-specific check has passed.

## Signals and response

The fixed baseline compares database identity, compatibility hits, statistics
reset/eviction counts, and the same old-name query fingerprint as the migration
preflight. Any change or failed read publishes observation status zero and exits
1. Successful samples publish status one and a fresh timestamp atomically.
The rule alerts on any failure or stale/missing sample; it has no pending delay.

Non-superuser calls to quoted or unqualified old names change the fingerprint.
Superuser maintenance is excluded and must be reviewed separately. Applications
and external jobs must not use the inspection superuser. Incidental matching
text can conservatively alert; retain the baseline and investigate, do not
silence it by taking another snapshot. Query-text statistics cannot prove the
absence of all dynamic SQL or dormant callers.

After Contract, both compatibility sequences may disappear together. The
collector keeps checking the database identity and query statistics. Failed
queries are not reliably represented by `pg_stat_statements`; the second rule
therefore alerts on even one PostgreSQL ERROR/PANIC line naming a retired object.
Verify the existing `{env="prod",service="postgres"}` Loki stream before relying
on that rule. This complements, rather than replaces, generic database alarms.

On any alarm, stop Contract rollout, identify the caller or coverage gap, and
retain the baseline and logs. After destructive execution, do not start the old
image by itself: use the coordinated pre-Contract backup/image recovery plan.

These files prepare monitoring; they do not install it in production or choose
the required rollback duration. The observation fingerprint for the final
evidence file is captured at the window endpoint using the documented
`query-fingerprint.sql`. Preserve the start baseline and continuous monitoring
records as separate evidence.

## Local rehearsal

`node scripts/student-contract-alert-rehearsal.mjs` creates a disposable Compose
project on the local Docker socket. It provisions the exact checked-in rules in
Grafana 11.5.2, uses Loki 3.4.3 and Prometheus 3.7.3, and sends synthetic alerts
only to a container-local webhook sink. All published ports bind loopback.
It verifies healthy initial state, exact old-error matching, firing states and
both webhook deliveries, then removes only its own project. It requires Node,
Docker Compose and yq. It neither reads production data nor contacts production.
