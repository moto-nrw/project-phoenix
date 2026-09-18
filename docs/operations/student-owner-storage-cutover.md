# Student owner storage Cutover (#2759)

Migration `1.15.397` applies the last backfill delta under one bounded write
lock, proves per school that the three owner tables reproduce `users.students`,
moves every student foreign key onto `users.student_profiles`, archives the old
base table as `users.students_legacy` and republishes the old name as a
compatibility view with routing triggers.

After it commits, `users.student_profiles` (People Directory),
`users.student_school_memberships` (School Membership) and
`users.student_care_profiles` (Care Plan) are authoritative. The compatibility
shape exists for one purpose only: deploying the previous image again. No
current provider reads or writes it, there is no fallback read chain and no
application dual write. [Contract #2760](https://github.com/moto-nrw/project-phoenix/issues/2760)
removes it after the rollback window.

Prerequisite: [the completed backfill](student-owner-storage-backfill.md).
`backfill student-owner status` must exit zero shortly before the release —
`care_state_mismatch_count` is measured against today, so an older green run is
not evidence.

## What the switch does

1. Takes the student backfill's advisory session lock, so a resumable run, its
   reset and this switch can never interleave, and refuses if `users.students`
   is no longer a base table.
2. Opens one transaction with a 5 s lock timeout and a 60 s statement timeout,
   and locks `users.students` and the three targets `ACCESS EXCLUSIVE`.
3. Per school: removes target profiles whose source row is gone, copies the
   remaining delta with the backfill's own statement, then re-verifies counts,
   canonical checksums, the row-wise mismatch, the guardian reconciliation and
   the care-state equivalence with the backfill's own projections. Any failing
   verdict aborts the whole transaction. A school with no completed backfill
   pass and at least one student aborts it before the copy. More than 100,000
   rows still to copy under the lock aborts it as well: that is an unfinished
   backfill, not a delta.
4. Hands the identity allocation over: both target sequences continue above the
   last id `users.students` issued, so every foreign key keeps the number it
   already names.
5. Repoints all foreign keys that reference `users.students` onto
   `users.student_profiles`, `NOT VALID`, so the switch does not scan the
   dependent tables while it holds the write lock. They are enforced for new
   and changed rows immediately.
6. Renames `users.students` to `users.students_legacy`, adds the `updated_at`
   triggers the old table had to the three targets, creates the two hit
   counters, and creates the `users.students` view plus its routing trigger.
   `users.expired_privacy_consents` is redefined over the owner storage; it
   would otherwise have followed the rename onto the archive and stopped seeing
   children enrolled after the switch.
7. Commits, then validates the repointed constraints one at a time outside the
   transaction. `VALIDATE CONSTRAINT` takes a share-update-exclusive lock on the
   dependent table, so ordinary reads and writes continue. Interrupting it
   leaves `users.students` as the compatibility view and `1.15.397` unrecorded.
   The next `phoenix migrate` sees the view, skips the switch, resumes
   `VALIDATE CONSTRAINT` (already-valid constraints are a no-op) and records
   the version.

The down step is deliberately an error. Rollback deploys the previous image
against the retained compatibility shape; it never renames the archive back over
live owner rows.

## What the compatibility view is

`users.students` returns the old column set from
`users.student_profiles JOIN users.student_school_memberships JOIN
users.student_care_profiles`, with `security_invoker = true`, so every read and
write stays inside the reader's tenant policy.

- The joins are inner. Every profile owns exactly one live membership and every
  membership one care profile, and `SELECT ... FOR UPDATE` — which the previous
  image uses — cannot be applied to the nullable side of an outer join.
- A membership with `deleted_at` set leaves the view. `users.students` had no
  soft deletion, so a retired enrollment has to read as a row that is gone.
- `updated_at` is the newest of the three rows, so a write to any one owner
  still moves the single timestamp the old shape exposed.
- The eight columns without a target — `guardian_name`, `guardian_contact`,
  `guardian_email`, `guardian_phone`, `sick`, `sick_since`, `excused`,
  `excused_since` — are read from `users.students_legacy`. A child enrolled
  after the switch has no archive row, so they read NULL (`sick`/`excused`
  read `false`). Those values are not authoritative anywhere: guardians live in
  `users.guardian_profiles` / `users.guardian_phone_numbers` and absences in
  `active.student_status_days`.

Writes through the view go to the owner of each column and mirror the whole old
row into the archive, so a previous image finds its legacy columns unchanged.
`INSERT` allocates the profile id from `users.student_profiles_id_seq` (the
view's column default) and lets the membership sequence allocate the membership
id — nothing references a membership id, and the profile id is what every
foreign key names. `UPDATE` locks profile, then the archive row, then live
membership and care so it cannot deadlock against `SELECT … FOR UPDATE`. It
re-reads and applies only the columns the statement assigned, including the
archive-only `sick` / `excused` / `guardian_*` fields the view still serves.
A concurrent `SET extra_info` is kept; `SET status` against a membership whose
status moved (the activate-students / `TransitionStatus` case) affects 0 rows,
the way heap `EvalPlanQual` would. `UPDATE` refuses to
change `id` or `tenant_id` (`SQLSTATE 23514`). `DELETE` removes the profile,
whose cascade takes the membership and the care profile, and the archive row
with it. A stale archive row still claiming the per-person unique key of a
child the current providers deleted is released by the same trigger rather than
rejecting the write.

Reading a child through the view costs more than reading the old table did: two
joins plus one primary-key lookup in the archive for the legacy columns, per
row. That is the price of the rollback window and another reason the
compatibility counters have to trend to zero. Watch the p95 of the student list
endpoints against the pre-release baseline while callers still go through it.

## Release and rollback

1. Save the previous image identifier and the completed
   [backfill evidence](student-owner-storage-backfill.md). Stop application
   writers before migrating, as in the normal deployment procedure.
2. Apply migrations. A missing checkpoint, an unequal checksum, an unreconciled
   guardian value, a stale absence flag or a lock timeout aborts the cutover
   transaction and changes nothing. Fix the cause, resume the existing backfill
   and retry `migrate`.
3. Deploy the new image. Exercise student creation, edit, class and group
   changes, enrollment approval, graduation and reactivation, deletion, the
   care plan and the parent portal. Save the evidence below with the image
   identifiers and sampling times.
4. If verification fails after the switch, follow the
   [complete release rollback](release-backup-rollback.md). The retained view
   serves the old student contract, but it does not make the whole old image
   compatible with every other changed schema. Do not run a down migration and
   do not rename the archive over current owner rows.

After a committed switch, use the compatibility repair, never the pre-cutover
copy:

    go run . backfill student-owner compatibility --verify-only
    go run . backfill student-owner compatibility

Run these through the guarded maintenance connection. The repair reinstalls the
versioned view, its routing and the redefined `users.expired_privacy_consents`,
resumes the foreign-key validation, and deletes archive rows whose child the
owner storage no longer holds. It never writes the three owner tables and never
resets the hit counters. `--tenant` limits the report and the archive cleanup to
selected school IDs; the schema definitions are shared and always repaired.

The student backfill and its reset still refuse to run against a view, so a
restart can never overwrite the authoritative targets with the stale archive.

## Capture database evidence

Run [the observation SQL](student-owner-storage-cutover.sql) through the guarded
maintenance connection with psql, `ON_ERROR_STOP=1` and its file flag. Save
output before rollout, after smoke tests and at the end of the rollback window.
The script uses one repeatable-read snapshot with bounded timeouts and changes
no application rows.

| Evidence | Required result |
| --- | --- |
| Final-delta checkpoint | Every school reports `stable`, equal counts and checksums, and all three mismatch counters at zero. These describe the switch, not later changes. |
| Compatibility counters | Read and write deltas attributable to the application trend to zero on the new image. |
| Rollback completeness | `broken` is zero for every school: no live enrollment without a care profile. `hidden` counts children whose enrollment was retired after the switch and is expected to grow slowly. |
| Unvalidated foreign keys | Empty. A non-empty list means the post-commit validation was interrupted; rerun the compatibility command. |
| Archive coverage | `stale_archived_rows` is zero after a repair run. `archived_missing` grows with every child enrolled after the switch and is expected. |
| Deadlocks and current lock waits | Save changes with `stats_reset`; investigate increases against operation errors. |

The read counter counts executed view probes, not returned rows; the write
counter counts routing-trigger attempts, including rolled-back ones. Both are
durable sequence counters. The observation SQL and the compatibility command
deliberately avoid querying the view, so neither inflates the very counter the
Contract ticket gates on. Never reset the counters to hide a non-zero trend.

## Capture application evidence

Record the existing per-owner Prometheus series for the three owners over the
window, using the templates in
[the request-child cutover record](enrollment-storage-cutover-2714.md#capture-application-evidence),
plus the rates of `phoenix_db_wait_count_total` and
`phoenix_db_wait_duration_seconds_total`, p95 from
`phoenix_unit_of_work_pool_wait_seconds_bucket` and
`phoenix_unit_of_work_lock_wait_seconds_bucket`, and the rates of
`phoenix_unit_of_work_rollbacks_total` and
`phoenix_unit_of_work_retries_total`. Compare with the pre-release baseline
under comparable traffic, and keep raw evidence, image identifiers, sampling
intervals, probe activity and database statistics resets together.

## Still open on #2759: the application caller switch

The database switch above is complete. The ticket's exit criterion also requires
that every current caller reads and writes the owner interfaces instead of
`users.students`, that exactly one application provider writes each field, and
that a caller-inventory test proves zero old application reads and writes. That
part is **not** in this change, and it is not a small edit, because the split
moves fields across owners that the current code reads from one place:

1. **School Membership has to own the enrollment queries and commands.**
   `modules/peopledirectory`'s student store serves `school_class`, `group_id`,
   the lifecycle `status` and the `enrolled_from`/`enrolled_until` window — all
   School Membership columns after the split. A People adapter reading
   `users.student_school_memberships` is a `tables.foreign-read` violation, and
   the policy admits a named read projection only for a package owned by a
   *projection* owner, never for a domain adapter. So `ListByClasses`,
   `ListEnrolled`, `ListClasses`, `Promote`, `RevertClass`, `GraduateByClasses`,
   `GraduateByIDs` and `Reactivate` move into a School Membership capability,
   and `ListByIDs`/`ListByPersonIDs` compose the two owners. Composing them in
   Go costs a second statement per list endpoint, which the shrink-only query
   budgets in `backend/test/query_budgets.go` do not allow, so the joined read
   belongs in a named tenant-safe read projection instead — the mechanism
   `class-day-view` and `group-live-view` already use. That needs a new
   projection owner, and a new owner needs an architecture decision linked from
   #2580 before it is added. **That decision is
   [#3386](https://github.com/moto-nrw/project-phoenix/issues/3386), which
   blocks #2759.**
2. **Care Plan has to own the care columns.** `supervisor_notes`, `health_info`,
   `pickup_status`, the departure plan and the companion note are written today
   through People's `ApplyEnrollmentProfile` and through the legacy student
   repository. They become Care Plan commands, and the multi-owner writes
   (create student, apply enrollment profile, delete student) become one
   UnitOfWork per write.
3. **The legacy `sick`/`excused` flags have to go.** They exist only in the
   rollback archive from here on. Every write path already mirrors them into
   `active.student_status_days` (`persistStudentStatusHistory` for the staff
   toggle, `recordStudentStatusForClear` for the next-check-in clear, the parent
   portal's report), and `ResolveEffectiveStatus` over those days is already the
   read the group-live projection uses. What is left is deleting the flag half:
   `peopledirectory.StudentStatusFlagCapability`, Care Plan's
   `ArchiveStudentStatusFlags`, the scheduler's end-of-day
   `ArchiveAndClearStatusFlag` task, and the four fields on
   `models/users.Student`, `peopledirectory.Student` and
   `careplan.StatusStudent` — with their remaining readers moved onto the
   effective status.
4. **The legacy guardian columns have to go, but they are a wire contract.**
   `guardian_name`, `guardian_contact`, `guardian_email` and `guardian_phone`
   have no owner storage; `users.guardian_profiles` and
   `users.guardian_phone_numbers` do, and the backfill's
   `guardian_mismatch_count: 0` gate is what makes losing the columns safe. But
   the four are required fields on the student list and detail wire types
   (`frontend/src/app/api/students/route.ts`,
   `frontend/src/app/api/students/[id]/route.ts`), so removing them means either
   re-sourcing from the guardian owners or changing the contract and the
   frontend. Split out as
   [#3387](https://github.com/moto-nrw/project-phoenix/issues/3387) so the
   caller switch can land without a frontend change.

Until that lands, current callers keep reading and writing through the
compatibility view, so the counters in the evidence table below will not trend
to zero and #2760 stays blocked. Nothing about the database switch changes when
the caller switch lands: the view, its routing and the counters are exactly the
rollback shape #2760 removes.

## Staging acceptance record

**Not yet executed for this revision.** Normal deployment stops the application
before running the migration. Record that path on staging, then run the
compatibility verification and attach its JSON. Separately, on an isolated
staging copy, run the migration and then serve the *previous* image against it:
genuine student reads plus create, edit, enroll, graduate and delete writes must
all still work through the view. Use dedicated synthetic identities and captured
mail.

| Evidence | Result |
| --- | --- |
| Deployment SHA / previous image digest | Pending |
| Cutover transaction wall time and held lock duration | Pending |
| Foreign-key validation wall time after commit | Pending |
| Per-school final-delta checkpoint (counts, checksums, three mismatch counters) | Pending |
| Previous-image read/write parity through the view on the isolated copy | Pending |
| Compatibility read/write counters before, during and after the smoke test | Pending |
| `broken` / `stale_archived_rows` per school | Pending |
| New-image p95 and error rate for student queries and commands | Pending |
| Pool waits, measured lock waits, deadlocks, serialization retries | Pending |
| Interrupted foreign-key validation resumed by the compatibility command | Pending |

## Local verification (2026-09-18)

Migration tests cover the contract golden (the previous image reads the same
columns, NULLs and JSON shapes through the view), routed previous-image
insert/update/delete including the immutable identity and the retained
`FOR UPDATE`, a retired enrollment leaving the view, the refusal without a
completed backfill pass, the refusal on an unreconciled guardian value, a
target-only edit reconciled by the final delta, full rollback of the delta and
its checkpoint when the switch fails afterwards followed by an idempotent
retry, the foreign-key repoint with its post-commit validation and a dependent
row of a child created after the switch, two-tenant RLS on the view in both
directions, the hit counters moving for previous-image traffic and not for
owner storage, the refusal to run twice, and the compatibility repair restoring
the rollback shape without touching owner rows or the counters.

These results do not replace the staging record above.
