# Student owner/caller cutover (#2759)

Base: development at `a06d58a5d7`, after #3384 and #3432.
Decisions: [ADR 0023](../../docs/adr/0023-student-enrollment-owner-and-directory-projection.md),
[ADR 0024](../../docs/adr/0024-retire-legacy-student-contact-fields.md) and
[ADR 0025](../../docs/adr/0025-replace-student-read-projection-grants.md). The previously merged inbox, announcement-audience
and care-exit projections are not replaced again.

## Caller inventory

| Former access | Current application path |
| --- | --- |
| People Directory student lists, records, names, roster, enrollment snapshot, care bounds and departure plans | `modules/studentdirectoryview` |
| Retained generic Student repository list/count, birthdays and operator membership | Embedded Directory query, with the existing tenant transaction and predicates |
| Guardian linked children, payment assignments and relationship reads | Embedded Directory query; relationship permissions remain at the caller |
| Care request queue name search | Directory subquery correlated with the queue tenant |
| Student identity, address, consents, photos and deletion | People Directory writes `users.student_profiles` |
| Enrollment, class/group assignment, graduation, explicit reactivation and care interval | School Membership commands write the current `users.student_school_memberships` row |
| Health/notes, departure plan and live absence flags | Care Plan commands write `users.student_care_profiles` |
| Grade-transition apply/revert | School Membership commands under its exclusive class-write gate |
| Care exit/cancel/resume | Consumer-owned `CareStudents` port, bound to Directory reads and Membership commands |
| Presence live-status mirror and Care status-flag clearing | Narrow Care Plan commands; no generic `UpdateColumns` |
| Parent enrollment, profile application and import | Same-tenant multi-owner unit of work, with command-local savepoint rollback |

**Old application table callers: zero.** Reproduce the literal inventory:

```sh
scripts/run-go-toolchain.sh go -C backend test ./database/repositories/studentintegration \
  -run '^TestStudentCutoverCallerInventory$' -count=1 -v
```

The test parses application Go source literals, excluding Go/SQL comments,
migrations, migration/repair CLI commands, architecture-checker sources and
test support. Relationship table `users.students_guardians` is not the old
student table. The sole documented DTO tag in `models/users.Student` remains
for retained model compatibility; no generic Student repository is embedded.
`TestStudentOwnersWorkWithoutRollbackView` drops the view in an isolated
database and checks current create, list, count, update, read and delete paths.
This dynamic check complements, rather than replaces, the source inventory.

Removed providers include People Directory's promotion/reversion/graduation/
reactivation commands, care interval/status writes, status-flag clearing,
exclusive class gate and the unused `GraduateByClasses`.

## Invariants

- The Directory projection reads exactly profiles, memberships, care profiles
  and persons. It joins membership by tenant/profile ID and care by
  tenant/membership ID, requires a live membership and preserves inner-join
  selection. Public IDs remain profile IDs.
- Alumni cannot change class, end care or resume care. Only `Reactivate`
  restores a non-alumni status. Exact unchanged enrollment/group writes remain
  idempotent; they cannot change an alumnus's membership.
- Resume requires an ended interval, using the caller's frozen calendar day.
- The class gate belongs to School Membership. Enrollment identity inserts
  consume its materialized gate CTE before locking the same-tenant, non-deleted
  person. Other writers retain gate-before-row ordering.
- Composite writes share the ambient RLS transaction. A command-local
  savepoint undoes earlier owner writes even if the caller catches the error
  and commits its surrounding transaction.
- Import supplies the initial profile with enrollment creation, avoiding a
  second full owner rewrite. The ten-child runtime scenario uses 244 statements
  against its unchanged limit of 250. BUN reports raw SQL as SELECT, so its
  separate driver DML counter is not a count of physical owner-table writes.
- The composition surface remains 667 field/setter targets. Projection grants,
  registered query budgets and RLS policies are not broadened.

## Rollback

`users.students`, its routing trigger, compatibility counters and repair
command remain. Migration 1.15.398 moves the four live sick/excused fields from
the archive into Care Plan without changing public modification timestamps.
The view and old-image writes address these same fields.

Guardian copies remain only in the rollback archive. Current application
creation does not populate that archive. Compatibility tests cover old-image
updates of newly owner-created children and reinsertion after current-image
deletion. Rollback remains deployment of the previous image against the view,
not destructive reversal of owner storage.
