---
status: accepted
---

# Student Deletion is an application workflow

The child lifecycle cutover in [#2710](https://github.com/moto-nrw/project-phoenix/issues/2710)
implements the second irreversible lifecycle workflow required by
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580), after the
staff offboarding in [ADR 0013](0013-staff-offboarding-is-an-application-workflow.md).
Add `student-deletion` as a workflow owner, with no runtime data ownership.
It coordinates the permanent deletion and anonymization of a child: the
authorization boundary, the preview fingerprint, the blocker-defined lock
order and one UnitOfWork. People Directory, Care Plan, Timetable & Activities,
School Structure, Student Presence, Communication, Enrollment, Appointments,
Feedback, Identity & Access and Audit retain their reads and mutations.

The three entry points (the confirmed deletion of an active child, the
purge of a graduate from the Abgänge view, and the deletion behind a pending
care withdrawal) call one workflow. The previous coordinating service in
`services/users`, its cross-schema repository in `database/repositories/users`
and the handler-side transaction of the purge route are removed; no second
runtime provider remains.

Putting the orchestration inside People Directory or Care Plan would make that
domain responsible for other owners' lifecycle rules. The separate workflow
keeps the 18 domain and 10 platform owners unchanged and transfers no table.

## Lock order and consistency

The workflow locks in this order inside one tenant transaction: the tenant
care-booking gate and the withdrawal task (withdrawal path only); the subject
and every companion far end through the People Directory student read in
ascending id order, late lower ids with NOWAIT; the Communication message
threads; the People Directory person row. It then re-reads every owner count
and compares the confirmed fingerprint before the first mutation. A linked
child whose departure plan would be left without "mit wem" refuses the
deletion, exactly as every companion-list edit does.

## Policy registration

Policy epoch 8 registers the workflow owner under the reviewed data-less
workflow path of ADR 0013 and, under [ADR 0015](0015-owners-adopt-existing-unowned-tables.md),
adopts `users.persons_guardians` into People Directory: the only accessing
package was already classified there, and the deletion unlinks the legacy
guardian rows through that owner. The activity-enrollment count still reads
the timetable legacy projection through a compatibility binding of the
workflow composition; it is rebound when the Timetable owner exposes the
count publicly.

## Durable cleanup

Care Plan records the document cleanup intents before the cascade removes the
document rows, inside the same transaction, and the existing document
cleanup worker removes the bytes after commit and retries failures. The photo
unlink and the companion broadcast are after-commit hints; a lost hint is
recovered by the existing sweeps. The person row is anonymized and
tombstoned rather than removed on both the ordinary and the purge path, so a
deployment rollback never restores a name, tag or account link; deleted
credentials and files are never resurrected implicitly. Audit tombstones,
prior deletion audits and anonymized grade-transition history are retained.
