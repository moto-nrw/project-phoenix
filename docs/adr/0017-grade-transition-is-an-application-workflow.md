---
status: accepted
---

# Grade Transition is an application workflow

The school-year rollover cutover in [#2711](https://github.com/moto-nrw/project-phoenix/issues/2711)
implements the last irreversible lifecycle workflow required by
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), after the
staff offboarding in [ADR 0013](0013-staff-offboarding-is-an-application-workflow.md)
and the student deletion in [ADR 0016](0016-student-deletion-is-an-application-workflow.md).
Add `grade-transition` as a workflow owner, with no runtime data ownership.
It coordinates the draft, the preview, the apply and the revert of a grade
transition: the authorization boundary, the preview fingerprint, the
blocker-defined lock order, the cohort resolution under those locks and one
UnitOfWork. School Structure, People Directory, School Membership,
Timetable & Activities, Student Presence, Enrollment and Audit retain their
reads and mutations.

School Structure becomes the owner capability for the transition rows it
already owned on paper: `education.grade_transitions`,
`education.grade_transition_mappings`, `education.grade_transition_history`,
`education.grade_transition_class_teachers` and
`education.grade_transition_class_list_entries` are read and written only
through its public transition capability. The previous coordinating service
in `services/education`, its cross-schema repository in
`database/repositories/education` and the repository contract in
`models/education` are removed; no second runtime provider remains.

Putting the orchestration inside School Structure or People Directory would
make that domain responsible for other owners' lifecycle rules. The separate
workflow keeps the 18 domain and 10 platform owners unchanged and transfers
no table.

## Lock order and consistency

Apply and revert take the three tenant-wide gates in the project-wide order
inside one tenant transaction: the People Directory class-writes gate
exclusively, then the timetable recurrence gate, then the School Structure
transition gate. The exclusive class-writes gate closes the window a row
lock cannot reach: a child created in, or moved into, a mapped class either
lands before the locked cohort re-read (and is refused as a late arrival) or
waits for the commit. The apply then locks every child of the mapped
classes in ascending id order through the People Directory, re-validates
each row under its lock, re-reads the cohort and compares the confirmed
fingerprint before the first mutation. The revert additionally locks the
latest applied transition FOR UPDATE and refuses any other target. Draft
edits and deletions take only the transition gate, keeping the acquisition
order acyclic against apply and revert.

## Policy registration

Policy epoch 10 registers the workflow owner under the reviewed data-less
workflow path of ADR 0013 with only candidate-created packages. The School
Structure capability grows inside its existing owner and roles; no table
changes owner. The workflow's ports over the Timetable roster reconciliation
and the enrollment offering-roster resync are consumer-owned and bound by
the legacy composition from the retained application services of those
owners; they are rebound when the owners expose them publicly.

## Durable cleanup

A grade transition performs no irreversible work: graduation is a soft
delete (the student row stays, the released bracelet is ledgered), every
class-teacher and class-list rewrite is ledgered for the revert, and the
archived roster rows are replayed by the revert. Every owner write commits
or rolls back together with the transition status; nothing is enqueued after
commit, so there is no cleanup intent to retry. Only the student deletion
workflow removes a graduate for good, through its own owner-specific policy.
