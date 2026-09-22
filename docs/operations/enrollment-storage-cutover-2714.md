# Request-child storage cutover and rollback window

**Historical.** The rollback window closed with migration 1.15.414
([cleanup, #2719](enrollment-storage-contract-2719.md)), which removed the
compatibility view, archive, functions, counters and the repair CLI described
below. This page records the cutover release as it was operated.

Migration 1.15.385 applies the final delta under a bounded write lock,
verifies each school, and replaces the old table name with a compatibility
view. Enrollment owns submitted choices and notes. Care Plan owns effective
bookings. Current application code never accesses the compatibility shape.
Keep the archive, view, functions, triggers, and counters for rollback.

## Release and rollback

1. Save the previous image identifier and completed
   [backfill evidence](enrollment-storage-backfill-2713.md). Stop application
   writers before migrating, as in the normal deployment procedure.
2. Apply migrations. Missing checkpoints, unequal checksums, or lock timeouts
   abort the cutover transaction. If the switch did not commit, fix the cause
   and resume the existing backfill before retrying migration.
3. Deploy the new image. Exercise submission, effective-date changes,
   rollover, capacity, care exit, and deletion. Save the evidence below with
   the image identifiers and sampling times.
4. If compatibility or checksum verification fails after the switch, follow
   the [complete release rollback](release-backup-rollback.md). The retained
   view supports the old Enrollment contract, but does not make the whole old
   image compatible with other changed schemas, notably the email outbox.
   Do not run a down migration or rename the archive over current bookings.

After a committed switch, use the resumable compatibility repair, not the
pre-cutover source-to-target copy:

    go run . migrate backfill request-child-compatibility --verify-only
    go run . migrate backfill request-child-compatibility --batch-size 500

Run these through the guarded maintenance connection. The repair reinstalls
the versioned view/functions/trigger and corrects only inconsistent archived
day payloads from authoritative bookings. It preserves owner data, all notes,
healthy historical JSON shapes, frozen cutover checkpoints, and hit counters.
Each bounded batch commits separately. Corrected rows leave the pending-work
set, so rerunning after interruption resumes without rewriting completed rows.
A failed batch rolls back in full. JSON output reports committed batches,
repaired rows, and per-school live checksums; remaining drift returns nonzero.
Use --tenant to limit metadata repair and verification to selected school IDs;
schema definitions are shared and are repaired for every school.

The old request-child-storage backfill still refuses to copy from a view:
restart must never overwrite current targets with the stale archive. Repair
does not invent missing submission history or modify owner data to conceal a
checksum problem. If verification still fails, keep the application stopped
until repaired or restore the complete previous release. This release retains the
rollback schema.

## Capture database evidence

Run [the observation SQL](enrollment-storage-cutover-2714.sql) through the
guarded maintenance connection with psql, ON_ERROR_STOP=1, and its file flag.
Save output before rollout, after smoke tests, and at the end of the rollback
window. The script uses one repeatable-read snapshot and bounded timeouts.
It changes no application rows, but its view probe advances the read counter,
so the transaction cannot be declared READ ONLY.

| Evidence | Required result |
| --- | --- |
| Final-delta checkpoint | Every reported school has complete and cutover_checksums_equal true. These checksums describe the switch, not later changes. |
| Live effective-state checksum | Every reported school has effective_checksums_equal true. Empty schools have no effective rows to report. |
| Compatibility counters | Application-attributable read and write deltas trend to zero on the new image. |
| Deadlocks and current lock waits | Save changes with stats_reset; investigate increases against operation errors. |

The read counter counts executed view probes, not returned rows. The write
counter counts trigger attempts, including rolled-back attempts. Both are
durable sequence counters, not commit counts. The observation SQL itself
probes the view. To measure application-only deltas, sample just its first
counter query twice during an interval with no compatibility probes or
previous-image traffic. Never reset counters to hide a nonzero trend.

The live checksum independently normalizes days without trusting the
compatibility functions. It compares effective identity, manual/automatic days,
validity, timestamps, and normalized selected days. It excludes rollback-only
historical notes. New submissions add immutable choices and booking commands
change effective state. Neither target should retain its cutover checksum
forever.

## Capture application evidence

The existing Prometheus endpoint exposes these metric prefixes:

- phoenix_enrollment_offering_storage
- phoenix_care_offering_booking

Labels are fixed operation names, never school IDs or child data. Validation
failures are included. A successful owner call can still be rolled back by
its enclosing workflow. For each prefix, record query/command p95, errors,
and mean input items per call with these PromQL templates:

    histogram_quantile(0.95,
      sum by (le, operation, kind) (rate(<prefix>_duration_seconds_bucket[5m])))

    sum by (operation, code) (rate(<prefix>_operations_total{outcome="error"}[5m]))

    sum by (operation) (rate(<prefix>_rows_sum{direction="input"}[5m]))
    / sum by (operation) (rate(<prefix>_rows_count{direction="input"}[5m]))

Input counts supplied booking rows or lookup IDs, not persisted changes.
Output records returned rows or an affected-row count when available.
Record/replace/schedule do not invent affected-row counts for retries.
Deadlock, serialization_failure, and lock_timeout are stable error codes
(the deadlock label is lowercase).

Also save rates of phoenix_db_wait_count_total and
phoenix_db_wait_duration_seconds_total; p95 from
phoenix_unit_of_work_pool_wait_seconds_bucket and
phoenix_unit_of_work_lock_wait_seconds_bucket; and rates of
phoenix_unit_of_work_rollbacks_total and phoenix_unit_of_work_retries_total.
Explicit lock-acquisition duration is an upper bound, not wait-only time.
Database-wide deadlock/lock snapshots are context, not owner attribution.

Compare measurements with the pre-release baseline under comparable traffic.
Keep raw evidence, image identifiers, intervals, probe activity, and database
statistics resets together. Tests prove instrumentation and drift detection;
they do not establish production latency or a completed rollback window.
