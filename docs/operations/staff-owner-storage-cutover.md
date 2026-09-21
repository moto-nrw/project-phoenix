# Staff owner storage Cutover (#2753)

Migration `1.15.409` applies the last backfill delta under one bounded write
lock, proves per school that the two owner tables reproduce `users.staff`,
moves every staff foreign key onto `users.staff_school_memberships`, archives
the old base table as `users.staff_legacy` and republishes the old name as a
compatibility view with a routing trigger. The same release switches every
application caller to the owners.

After it commits, `users.staff_school_memberships` (School Membership: tenant,
person, lifecycle, soft deletion) and `users.staff_employment_profiles`
(Workforce: notes, employment type, work-time model, personnel number, rotation
anchor, birthday opt-out) are authoritative. The compatibility shape exists for
one purpose only: deploying the previous image again. No current provider reads
or writes it, there is no fallback read chain and no application dual write.
[Contract #2754](https://github.com/moto-nrw/project-phoenix/issues/2754)
removes it after the rollback window.

Prerequisite: [the completed backfill](staff-owner-storage-backfill.md).
`phoenix backfill staff-owner status` must exit zero shortly before the release.
The migration's preflight refuses a staff row whose work-time model belongs to
another school; no copy can close that gap, only a data correction can.

## What the switch does

1. Takes the staff backfill's advisory session lock, so a resumable run, its
   reset and this switch can never interleave, and refuses if `users.staff` is
   no longer a base table.
2. Opens one transaction with a 5 s lock timeout and a 60 s statement timeout
   and locks `users.staff` and both targets `ACCESS EXCLUSIVE`.
3. Per school: removes memberships whose source row is gone, copies the
   remaining delta with the backfill's own statement in one pass (retired
   memberships before their replacements), then re-verifies counts, canonical
   checksums and the row-wise mismatch with the backfill's own projections. Any
   failing verdict, or a school with staff but no completed backfill pass,
   aborts the whole transaction. The verdict is written into the school's
   checkpoint row.
4. Hands identity allocation over: the membership sequence continues above the
   last id `users.staff` issued, so every foreign key keeps the number it names.
5. Repoints all 63 foreign keys that reference `users.staff` onto
   `users.staff_school_memberships`, `NOT VALID`. The list is static; the switch
   fails if the database still holds a staff foreign key the list does not name.
6. Renames `users.staff` to `users.staff_legacy` and drops the archive's
   work-time-model key, so the frozen archive cannot keep a detached template
   alive. Renames the archive's indexes and gives the live-person index of the
   membership the old name `idx_staff_tenant_person`. Adds the `updated_at`
   trigger the old table had to the membership, the personnel-number rule
   (below), the two hit counters, the `users.staff` view and its routing
   trigger, and redefines `audit.time_tracking_audit_log`, whose actor join
   would otherwise have followed the rename onto the archive.
7. Commits, then validates the repointed constraints outside the transaction.
   Interrupting that leaves the view in place and `1.15.409` unrecorded; the
   next `phoenix migrate` sees the view, skips the switch, resumes
   `VALIDATE CONSTRAINT` and records the version.

The down step is deliberately an error: rollback deploys the previous image
against the retained compatibility shape.

## The compatibility view

`users.staff` returns the old columns in the old order from
`users.staff_school_memberships JOIN users.staff_employment_profiles`, with
`security_invoker = true`. Unlike students, staff had soft deletion, so a
retired membership stays visible with its `deleted_at`.

- The join is inner: every membership owns one profile, and `SELECT ... FOR
  UPDATE`, which the previous image uses, cannot lock the nullable side of an
  outer join.
- `INSERT` writes the membership (id from the membership sequence) and the
  profile. `DELETE` removes the membership; the profile follows by cascade.
- `UPDATE` refuses to change `id` or `tenant_id`, locks membership then
  profile, and applies only the columns the statement assigned onto the locked
  rows, so a concurrent owner write is not overwritten by the statement's
  snapshot. A row whose `deleted_at` changed under the statement counts as no
  row updated, as the heap recheck of `WHERE deleted_at IS NULL` did.
- The membership trigger bumps `updated_at` on every routed update, as the old
  table's trigger did.
- A duplicate live staff member for a person still fails on
  `idx_staff_tenant_person`, and a taken personnel number on
  `uq_staff_tenant_personnel_number`, so the previous image classifies both
  conflicts exactly as before.

`users.staff_compatibility_reads` counts queries against the view and
`users.staff_compatibility_writes` counts routed rows. Both must trend to zero
on the new image.

## Personnel numbers

`users.staff` enforced one live staff member per personnel number and school
with a partial unique index. "Live" is now the membership's `deleted_at` and the
number is the profile's, so no single-table index can express it. The trigger
`staff_employment_profiles_personnel_number` checks it on insert and on a change
of number, serialised per school and number by a transaction advisory lock, and
raises `23505` with the old constraint name. A retired holder frees the number.

## What the new image does

- School Membership stores staff memberships only. `CreateStaff` and
  `UpdateStaff` write the membership and then the Workforce profile through a
  consumer-owned port, inside a savepoint: a failed owner write undoes both,
  even when a caller catches the error and commits its own transaction. Staff
  reads compose the profile in one batched read.
- Workforce owns the profile through `workforce.StaffEmployments`: notes,
  birthday opt-out, work-time detach, template binding and rotation anchor.
  Template refreshes read the binding from the profile and ask School
  Membership only which assignees are still live.
- Schedule resolution (`services.StaffScheduleAssignments`) reads the binding
  from Workforce in one statement.

Behaviour differences, all deliberate:

- Employment-only writes (notes, opt-out, work-time detach, anchor rebase) no
  longer move the membership's `updated_at`; `UpdateStaff` still does.
- The schedule binding of an offboarded staff member stays readable; offboarding
  clears the template itself.

## Evidence

Local, 2026-09-21, disposable PostgreSQL clones:

| Measurement | Result |
|---|---|
| Switch with 3 schools, 600 staff, 25-row delta | lock-holding transaction 122 ms, schema switch 85 ms, constraint validation 27 ms |
| Deadlocks, pool wait during the switch | 0, 0 s |
| `create_staff` / `find_staff` / `list_staff` / `update_staff`, 30 samples after 5 warm-ups | p50 3.9 / 8.4 / 12.3 / 14.3 ms, p95 14.2 / 73.1 / 25.2 / 26.8 ms; 8 / 6 / 6 / 9 statements including transaction control |
| Compatibility counters after those 120 owner calls | 0 reads, 0 writes |
| Caller inventory (`TestStaffCutoverCallerInventory`) | 0 application literals reach `users.staff`, the archive or the counters; one documented DTO tag on `models/users.Staff` for test fixtures |

Latencies are local and relative, not an SLO. Staging acceptance (switch wall
time on real data, previous image against the switched schema, counter trend)
is still to be recorded below.

| Staging | Result |
|---|---|
| Switch wall time | |
| Previous-image smoke against the view | |
| Counters after 24 h on the new image | |

Observation queries: [staff-owner-storage-cutover.sql](staff-owner-storage-cutover.sql).

## Rollback and cleanup

Deploy the previous image. It reads and writes `users.staff` through the view;
nothing needs to be undone first. Do not run a down migration, do not rename the
archive back, and do not drop the view, its routing, the personnel-number
trigger or the counters: #2754 removes them after the rollback window. If the
view's checksum drifts from the owners, deploy the new image again and fix the
data in the owner tables; the pre-cutover backfill and its reset refuse to run
against the view.
