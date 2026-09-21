---
status: accepted
---

# Care Lifecycle retirement may replace its legacy permissions

Accepted for #3427 in the implementing change, following the precedent of
ADR 0030. The pull request review is where it can still be rejected; the
exception activates only in the policy epoch that review merges.

## Context

#3427 removes `modules/careplan/legacy/carelifecycle` (care exit, withdrawal
tasks, booking authority, companions, child documents) and the tenant-safe
projection `modules/careplan/legacy/careexitview`. Their behavior now sits
behind the existing Care Plan public contract, its native application and
domain, the Timetable owner and the `student-directory-view` projection.

The PR-mode strictness check rejects every new permission between package-role
points that exist at the base SHA. Two consumers that only ever reached the
retired adapter, the operator booking-authority setting in `api/operator`
(`inbound-operator/http`) and in `modules/settings/inbound/operator`
(`settings-platform/http`), must now name Care Plan's public contract. The
retired adapter's behavior suites also moved into Care Plan's contract suite
(`modules/careplan/contracttest`, `care-plan/e2e-test`), which exists at the
base, and keep the fixtures they used before. Deleting the adapter's
compatibility permissions does not change either result, and converting them
to exact debt is what the ticket rules out.

ADR 0030 settled the same situation for the care-schedule cutover (#3351)
with a one-time, epoch-gated exception. This decision applies that shape to
the care-lifecycle adapter.

## Decision

Permit this one cutover in a reviewed policy epoch, under these conditions:

1. The immutable base contains `modules/careplan/legacy/carelifecycle`,
   classified as `inbound-students/adapter` with the test roles
   `module-internal-test` and `module-behavior-test`.
2. The candidate classifies no package at that path or below it. Ordinary
   graph checks also reject a retained but unclassified source package.
3. A consumer gains access only to Care Plan's `public` or `contract` role,
   and only in a scope where its own owner-role point could already reach the
   retired adapter. In `internal_test` and `external_test` scope only, that
   same prior permission may instead reach Care Plan's `compose` role, so a
   suite that constructed the retired service constructs its native
   replacement. No production construction, no borrowing between scopes, no
   `application`, `port`, `postgres` or `domain` access.
4. Care Plan's contract suite (`care-plan/e2e-test`, `external_test` scope
   only) may reach a target that the retired adapter's own external tests
   (`inbound-students/module-behavior-test`) could reach at the base. The
   retired owner itself (`inbound-students`) is never a target. This carries
   the moved suites' fixtures, not new reach: the rules stay explicit in
   `policy.json`, one per target, each naming #3427 and this ADR.

The exception applies to import strictness only. It cannot change data
ownership, forgive a new debt key, expose an implementation role, permit an
external dependency, or bypass composition and runtime checks. Rules still
need to be narrow and live. The base package and epoch conditions prevent a
reuse after the cutover reaches the base.

Epoch 20 uses it for exactly these rules:

| Scope | Source owner/role | Target owner/role |
|---|---|---|
| production | inbound-operator/http | care-plan/public |
| production | settings-platform/http | care-plan/public |
| external_test | care-plan/e2e-test | audit-platform/domain |
| external_test | care-plan/e2e-test | enrollment/domain |
| external_test | care-plan/e2e-test | enrollment/test-support |
| external_test | care-plan/e2e-test | people-directory/application |
| external_test | care-plan/e2e-test | student-presence/adapter |

The same epoch deletes the 46 rules that carried #3427 (the
`inbound-students.adapter.*`, consumer `*.inbound-students-adapter` and
`care-exit-view.*` families), the two retired packages, and the
`care-exit-view` owner and read projection.

## Verification

`internal/architecture/care_lifecycle_cutover_test.go` covers the consumer
replacement (public and contract granted; composition and private roles
refused in production; no lending between scopes; unrelated owners and
sources without the historical permission refused), test-scope composition
(granted in the historical scope only; application refused), the moved suites
(granted target; other scopes, other Care Plan points, targets the retired
suites never had, and the retired owner refused), and incomplete retirement
(same epoch, retained adapter or subpackage, reclassified base, foreign module
path). `scripts/backend-architecture.sh check` runs the strictness comparison
against the merge base and stays green with the baseline unchanged.

## Readings of the ticket this cutover settles

- Child documents stay Care Plan records. #3427 item 4 places document
  metadata with File Storage (ADR 0010), but the same ticket also rules out any
  write-owner change and any DDL. The binding constraint wins: File Storage
  keeps moving the bytes through its coordinator, whose store port composition
  now binds straight to Care Plan's records, and the category authority stays
  with Care Plan. Moving `users.student_documents` to File Storage is a
  separate ownership change.
- "Policy ownership unchanged" is read as data-object ownership, which does
  not change. The `care-exit-view` projection owner holds no data; its reads
  moved into `student-directory-view`, which grants exactly the same four
  tables, so keeping an empty owner would leave a stale classification.

## Consequences

This does not authorize general rule loosening; other legacy-to-owner cutovers
still need their own decision. The five moved-suite targets are permanent test
permissions of the contract suite. `student-presence/adapter` is itself
legacy, so that rule falls away with the presence adapter's own retirement.
