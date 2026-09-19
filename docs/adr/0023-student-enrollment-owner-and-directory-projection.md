---
status: accepted
---

# Student enrollment is School Membership's, the directory read is a projection

[#3386](https://github.com/moto-nrw/project-phoenix/issues/3386) asks where the
student enrollment queries and commands live after
[#2759](https://github.com/moto-nrw/project-phoenix/issues/2759) split
`users.students` into `users.student_profiles` (People Directory),
`users.student_school_memberships` (School Membership) and
`users.student_care_profiles` (Care Plan). The database half landed in
[#3384](https://github.com/moto-nrw/project-phoenix/issues/3384); the
application half could not follow, because the policy forbids one owner reading
and writing another's columns and a domain adapter may not host a read
projection.

The decision is a projection owner for the joined read and a School Membership
capability for the writes. Neither half fits the other: a projection owner can
never be a write owner (`policy.go:217`), and composing the read in Go would
cost a statement on every student list at a moment when no student budget has
any slack.

## The projection owner

`student-directory-view`, kind `projection`, one package
`modules/studentdirectoryview` with role `postgres`, and one grant:

```json
{
  "id": "student-directory-view",
  "package": "modules/studentdirectoryview",
  "data_objects": [
    "users.student_profiles",
    "users.student_school_memberships",
    "users.student_care_profiles",
    "users.persons"
  ],
  "tenant_safe": true
}
```

`users.persons` is in the grant because the student reads join it today
(`student_store.go:63`, `student_records.go:229`, `student_roster.go:54`). The
list is final: `readProjectionLoosenings` (`policy_compare.go:255`) accepts a
new grant only for an owner that does not exist at the base SHA, and has no
epoch escape, so a table left out here needs a reviewed policy decision to add
later.

The owner is named after the screen it serves, as `class-day-view`,
`group-live-view`, `request-review-view` and `operator-dashboard-view` are. The
split named three domain concepts; their joined view is not a fourth concept,
it is the Kinderliste, and naming it after the enrollment would claim a concept
the read model cannot write.

The projection carries care data because `GET /students` already returns
`health_info` and `supervisor_notes` (`api/students/types.go:85`) from the same
statement as the class and lifecycle columns. This makes it a full-access read:
a caller without `has_full_access` narrows before rendering, exactly as it does
today. Splitting care out would not remove the wide read, only the single
statement that serves it.

## The enrollment capability

School Membership grows `Enroll`, `ChangeClass`, `Graduate`, `Reactivate`,
`EndCare`, `ResumeCare` and `AssignGroup`, and takes over the tenant-wide
class-writes gate that People Directory holds today. Commands, not row writes:
the current guards have already drifted apart under a row-shaped interface, as
`Promote` refuses an alumnus (`student_store.go:192`), `RevertClass` does not
(`:208`) and `SetCareEnd` checks no status at all
(`student_lifecycle.go:85`). Each command owns its guard, and #2759 resolves
the three inconsistencies deliberately rather than carrying them across the
boundary.

`GraduateByClasses` is deleted rather than moved. It has no production caller.

The multi-owner writes that span the three tables, creating a child, applying
an enrollment profile, the parent-portal enrollment insert and renew, and
deleting a child, each become one UnitOfWork.

## The care exit is a port, not a workflow

The care exit writes `enrolled_until`, `enrolled_from` and `status`, which
School Membership now owns, but is driven entirely by Care Plan, which already
owns the `CareExit` record including `PreviousEnrolledUntil`. Care Plan reaches
`EndCare` and `ResumeCare` through a consumer-owned port inside its existing
transaction.

[ADR 0016](0016-student-deletion-is-an-application-workflow.md) says that
putting orchestration inside Care Plan would make that domain responsible for
another owner's lifecycle rules, which reads as requiring a fourth workflow
owner here. The distinguishing line is reversibility: staff offboarding,
student deletion and grade transition all coordinate irreversible work, which
is why no single domain may rule them. The care exit is reversible by
construction, since `Cancel` and `Resume` sit on the same service. A reversible
multi-owner write is a port; an irreversible one is a workflow.

## Columns without an owner

Eight columns have no target table: `sick`, `sick_since`, `excused`,
`excused_since`, `guardian_name`, `guardian_contact`, `guardian_email` and
`guardian_phone`. The expand migration left them behind on the reasoning that
`active.student_status_days`, `users.guardian_profiles` and
`users.guardian_phone_numbers` are the authorities, and the backfill verifies
the equivalent current state
(`001015393_student_owner_storage_expand.go:48`).

That reasoning is right about the data and wrong as grounds for dropping the
columns, because the code is not migrated. Five paths still write them
(`student_status_day_write.go:397`, `visit_helpers.go:217`,
`excused_requests.go:115`, `parent_write_service.go:1795`,
`student_handlers.go:1116`) and four still read them rather than the status day
(`response_helpers.go:145` and `:262`,
`supervisiondashboard/legacy/legacy.go:530`,
`grouplive/legacy/legacy.go:216`), while `grouplive.go:496` and
`status_days_response.go:122` compute the same values from the status day. Two
live paths in parallel.

So the columns survive: `guardian_*` is adopted into `users.student_profiles`
and `sick`/`excused` into `users.student_care_profiles`, by a migration in
#2759's window, before
[#2760](https://github.com/moto-nrw/project-phoenix/issues/2760) drops
`users.students_legacy`. Neither addition changes the grant above, because both
tables are already in it. `users.student_care_profiles` holds one row per
enrollment regardless of the school's Betreuungsmodell, so this is not a
dependency on the Betreuungsplan being in use.

Choosing between the two live paths, so that every reader takes its absence
state from `active.student_status_days` and its guardian contact from the
guardian tables, is deliberately deferred past the migration. Changing a
feature while its ownership is still moving means two uncertain things at once.

## Corrected evidence

#3386's evidence predates
[#3349](https://github.com/moto-nrw/project-phoenix/issues/3349) and overstates
the room available. The legacy `database/repositories/users` violation it
records no longer exists: that file is a stub returning `errStudentWritesMoved`
and the SQL already sits in the People Directory owner, so there is one
candidate path, not two. `api.students.list` is `{max: 30}` and
`modules.emergencysnapshot.snapshot` is `{max: 9, exact: true}`, not 31 and 10,
and neither `api.students.list` nor `api.students.ogs_group_live` has the
headroom the issue credits them with: `ede9370276` lowered both to their
measured counts on the grounds that the register may not carry headroom at all.

Against that, raising a `max` is not mechanically blocked. `TestQueryBudgetRatchet`
never compares numbers, and the deviation clause has a reviewed-raise precedent
recorded in the #3065 pull request. Composing the read in Go was therefore
possible, at the price of reviewed raises across `api.students.list`,
`api.students.ogs_group_live` and `api.class_list_entries.list` plus a
hand-edited exact budget on the Notfallliste. The projection avoids all four.

## Consequences

[ADR 0017](0017-grade-transition-is-an-application-workflow.md) records the
class-writes gate as People Directory's and fixes an acquisition order around
it. The gate moves to School Membership with the writes it fences, and #2759
carries a superseding note on that ADR. Leaving the gate behind would put the
lock on one side of the owner boundary and the write on the other.

Three existing projection grants name `users.students`:
`parent-message-inbox`, `parent-announcement-audience` and `care-exit-view`.
Each reads the compatibility view and needs a different table before #2760
drops it, and by the immutability above none of the three can be amended in PR
mode. That work is unclaimed and belongs on #2760.

In this codebase "angemeldet" on a child means "kein Abgänger" and not "inside
the Betreuungszeitraum": `ListEnrolled` filters `status <> alumnus` and reads
neither `enrolled_from` nor `enrolled_until` (`student_store.go:100`). The
enrollment capability keeps the two meanings apart by name, and `CONTEXT.md`
records the distinction.
