# Care Schedule retirement may replace its legacy permissions

Status: accepted for #3351. The task owner approved the scoped ratchet
amendment during implementation on 2026-09-20.

## Context

#3351 removes `modules/careplan/legacy/careschedule` and moves its behavior
behind the existing Care Plan public contract. The current strictness check
rejects new permissions between existing package-role points, including
`api/students` to `modules/careplan`. Deleting the old permission does not
change that result. Creating another compatibility package would preserve
the debt this ticket must remove.

## Decision

Permit this one cutover in a reviewed policy epoch, under these conditions:

1. The immutable base contains the exact care-schedule package, classified
   as `inbound-schedules/adapter` with its existing test roles.
2. The candidate removes that package, its descendants, and every package
   classified under `inbound-schedules`. Normal graph checks also reject
   any retained but unclassified source package.
3. A consumer gains access only to Care Plan's `public` or `contract` role,
   and only in a scope where its same owner-role point could already reach
   the retired adapter. Production permissions cannot authorize test imports.
   In internal or external test scope only, that same prior permission may
   instead reach Care Plan's native composition entry point to construct the
   replacement service. It cannot authorize production construction, borrow
   another scope's permission, or expose private application code.
4. The native implementation may bind its own contracts: Care Plan's
   `compose` and `application` roles may reach its `contract` role, and its
   `port` role may reach its `public` or `contract` role. This applies only
   in production scope. It grants neither private implementation access nor
   new cross-owner or test-support dependencies.
5. Care Plan's value contracts and compose/application/domain/port code may use
   `sharedkernel/calendar` instead of `internal/timezone`. The base must
   already permit the retired service to read that wrapper in the same
   scope. The canonical calendar classification stays unchanged, and no
   other shared-kernel contract may gain permission through this exception.
6. The retired package also exposed Timetable scheduling errors and contracts,
   and aliased Student Presence's room-capacity error. Their consumers may name
   the actual public owners at these four points only, each requiring the same
   scope's historical adapter permission:

   | Scope | Consumer owner/role | Public target owner |
   |---|---|---|
   | production | timetable-activities/compose | student-presence |
   | internal_test | inbound-timetable/module-internal-test | timetable-activities |
   | external_test | inbound-timetable/module-behavior-test | timetable-activities |
   | external_test | enrollment/module-behavior-test | timetable-activities |

   This does not grant another owner's composition or implementation role.
   Moving these aliases onto Care Plan would preserve false ownership.
7. Two retained arrival-integration fixture points may construct Timetable's
   native class projections: `inbound-timetable/adapter-test` in
   `internal_test`, and `enrollment/module-behavior-test` in `external_test`.
   Both require their same-scope historical adapter permission. The only
   target is `timetable-activities/compose`. The fixtures bind the projections
   through Care Plan composition themselves. `legacy-composition/compose` is
   not a target, because that role also covers `database/repositories`; a
   grant would turn the tracked repository-import debt of these tests into a
   permanent permission. No production scope or private implementation role
   is authorized.

The exception applies to import strictness only. It cannot change data
ownership, forgive a new debt key, expose an implementation role, permit
an external dependency, or bypass composition and runtime checks. Rules
still need to be narrow and live. The base package and epoch conditions
prevent this exception from being reused after the cutover reaches the base.

## Verification

Architecture CLI tests create a real Git base and check the candidate against
it. They cover a completed retirement, canonical-calendar use, a missing
epoch, retained source, a relocated adapter, an unrelated module, private
implementation access, owner-kind wildcards, an unapproved test scope, extra
kernel access, and attempted reuse after retirement reaches the Git base.
Existing policy-strictness tests remain mandatory.

The four native public replacements also have positive tests and negative
cases for absent historical permission, other scopes, unrelated owners,
non-public roles, missing epoch, and incomplete retirement.
The arrival-fixture cases separately verify the four construction pairs and
reject another scope, missing historical permission, missing epoch, unrelated
composition owners, and private or test-support roles.

The native-implementation cases reproduce the rejected owner-internal
contract bindings and the composition-to-calendar binding before enabling
them. Negative cases reject private access, borrowing that permission for
tests, and extending it to production-scope test helpers.

## Consequences

This does not authorize general rule loosening. Other legacy-to-owner
cutovers still need an architecture decision. The amendment cannot make an
intermediate state green while the care-schedule package remains.
