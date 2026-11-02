---
status: accepted
---

# Restored care plan is re-planned in the care-exit workflow

Issue [#5235](https://github.com/moto-nrw/project-phoenix/issues/5235):
cancelling a planned care end, and confirming a changed one, put the removed
offering-sourced roster rows back verbatim, with the planning period recorded
when they were removed. When the template's schedules had moved to another
period meanwhile, the child was missing on the template's new occurrences
until some unrelated resync touched the template. PR #5188 (ADR 0067
addendum, #5176) requires a resync after every path that copies sourced rows;
the restore was the documented exception ("corrected on the next resync").
The school decided option A: re-plan right after the restore, in the same
transaction.

## Decision

1. Care Plan owns the rule. `careplan.SourcedRosters` gains
   `ResyncStudentSourcedRosters(studentIDs, effectiveFrom)`: the ordinary
   template resync, scoped to the students' request children on every
   sourced template whose sources they hold or on which they carry a row.
   The period formula of #5176 (`sourcedRowPeriodID`) is unchanged and stays
   the only place that decides a sourced row's period. A template whose
   source drifted invalid is skipped with a warning, so the forgiving restore
   is not blocked.
2. The trigger lives in the existing `care-exit` workflow, not in the Care
   Plan lifecycle. `workflows/careexit.PlanRestoration` opens the tenant
   unit, runs the unchanged lifecycle command (`Cancel`, `Confirm`,
   `ConfirmWithdrawalCareEnd`; their transaction joins the unit) and then the
   student resync from today. Cancel re-plans every cancelled child; a
   confirmation re-plans the children it reports in
   `CareExitResult.ReappliedStudentIDs`, i.e. those whose earlier exit it
   restored. The lifecycle cannot call the resync itself: the booking
   materialization is built after, and from, the lifecycle (withdrawal
   follow-up), so the dependency would close a cycle or need late binding.
3. E2's protocol is unchanged: no lock, ordering, guard or owner is added to
   the lifecycle. The re-plan runs after the complete restore, including the
   care boundary, inside the gates the lifecycle already holds (class shared,
   recurrence, child rows). A failed re-plan rolls back the whole command.
4. The HTTP routes reach the three commands through the adapter's
   `careplanhttp.CareExitRestoration` port, which the root binds to the
   workflow (as `CareResumption` is bound to its workflow). Route-sized test
   graphs without a booking materialization bind the lifecycle directly.
5. Command effects: the port's three bindings to
   `workflows/careexit.PlanRestoration` are `workflow`; the lifecycle's
   recurrence lock reached through the port keeps the classification of the
   identical direct lifecycle entries (`debt`, protocol review left open by
   ADR 0098); the transaction runner entries are `platform`. This ADR is
   their approval; a changed closure needs a new one.

## Consequences

- Restored sourced rows carry the materialization period immediately after a
  successful cancellation or changed end; unchanged rows keep their id.
  A row whose period changed is replaced by the existing resync rule (deleted
  and re-created when not yet started, capped and re-seeded when running),
  from today on; history before today is untouched.
- Manual and legacy-fed rows, other children's rows and occurrences with
  recorded or hand-decided attendance keep the protection of the ordinary
  resync. The re-plan is scoped to the restored children; it is not a
  tenant-wide resync.
- Logging of the lifecycle command happens when its joined unit returns,
  before the workflow's re-plan; a later rollback leaves that log line
  without effect, as for the other non-durable notices.
