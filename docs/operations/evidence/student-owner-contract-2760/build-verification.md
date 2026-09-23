# Local image and review follow-up, 2026-09-20

The production Dockerfile completed successfully with synthetic build argument
`RELEASE_COMMIT=4aa36c315fc19e8ae8dda50193ffa34843e25575` and local tag
`student-contract-buildcheck:2760`. This was a dirty-worktree check, not a
release built from that commit. No image was pushed or deployed.

Both checks used `docker run --rm --network none`, a read-only mount containing
only synthetic release-identity JSON, and no database credentials:

1. `./main migrate preflight --student-contract-evidence /evidence/mismatch.json`
   exited 1 with `student contract: evidence does not match the embedded release commit`.
2. The same command with `/evidence/matching.json` exited 1 with
   `DB_DSN environment variable is required`. The embedded identity check
   therefore accepted the matching identity before database configuration.
   This does not establish that the incomplete synthetic evidence would pass
   operational validation.

The image predates the subsequent SHA-256 fingerprint correction. The correction
passed `scripts/run-go-toolchain.sh go -C backend test -count=1 -parallel 8
./database/migrations -run 'TestStudentOwnerContract'` in 13.561 seconds.
Previously recorded runtime measurements and their source hashes remain evidence
of their earlier source state, not measurements of the SHA-256 revision.

## Standards review

The new Contract fingerprints used MD5, contrary to the repository security
rule. Replaced both hashing levels with PostgreSQL's built-in SHA-256, preserving
all-column and per-school comparison. The focused Contract tests passed.

## Spec review follow-up

1. The live preflight now also compares a required SHA-256 fingerprint of
   retained old-object query statistics against `old_query_fingerprint_end`.
   It includes database-local calls by all non-superuser roles, query identity,
   top-level/nested execution, call and row counts, and per-entry statistics
   start. Quoted and unqualified names are included. Existing statements being
   executed again change the fingerprint. The same check runs under the DDL
   locks. Superuser inspections are excluded and require separate operator
   review; no application or external job may use the inspection superuser.
   Incidental old-name text conservatively blocks execution too. The fingerprint
   does not replace review of the full observation window or external consumers.
   [Capture SQL](query-fingerprint.sql) produces the evidence field.
   An uncached local PostgreSQL 17 run with `pg_stat_statements` preloaded passed
   all `TestStudentContract` tests in 11.162 seconds. The database regression
   proves that a direct quoted archive query under `phoenix_admin` changes the
   fingerprint while view counters remain unchanged, and that superuser
   inspection does not change it. The live-state validator rejects the changed
   fingerprint. The operational test also completes Contract with matching
   evidence. See [raw test output](live-query-tests.txt).
   The subsequent pinned `golangci-lint run --timeout 10m` completed with
   zero issues, and `scripts/backend-architecture.sh check` passed.
2. A read-only collector, stale/missing-observation alarm and retired-object
   SQL-error alarm are now implemented. The collector was exercised against
   local PostgreSQL and the metric alarm with promtool. A subsequent disposable
   Loki/Grafana exercise verified both firing states and delivery to a local
   webhook sink; production installation remains outside this task. See
   [monitoring evidence](monitoring.md).

The policy was zero at the time of this build check. It has since been set by
the user's explicit decision to at least 24 hours plus a complete regular school
day and relevant jobs; see the current runbook. Build tests do not themselves
provide operating evidence or authorize production deletion.
