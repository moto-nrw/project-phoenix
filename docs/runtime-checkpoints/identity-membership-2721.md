# School membership and role read evidence (#2721)

Issue: [#2721](https://github.com/moto-nrw/project-phoenix/issues/2721).
Measured locally on 2026-09-17, Go 1.27.0, macOS arm64, PostgreSQL 17.11,
isolated test clone, sequential concurrency 1, five warmups and 30 measured
calls per operation. The accepted checkpoint remains #3020; this document
does not accept another checkpoint and is not a staging or production window.

## Cutover and ownership

The People Directory (`database/repositories/users`), Organisation & Tenancy
(`database/repositories/platform`) and parent portal
(`database/repositories/parent`) repositories no longer name
`auth.account_tenants`, `auth.account_roles` or `auth.roles`. They filter
through owner queries of `database/repositories/auth`, bound once at
construction in `database/repositories/identity_ports.go`:

- `AccountTenantRepository.ActiveMemberships` selects `(account_id,
  tenant_id)` of every ACTIVE mapping. Consumers join it or use it in a
  row-value `IN` predicate paired with their own tenant column, because
  `auth.account_tenants` has no row-level security and the parent portal,
  operator dashboard and login read across schools.
- `AccountRoleRepository.GuardianRoleHolders` selects `(account_id,
  tenant_id)` of every guardian base role assignment.
- `AccountRoleRepository.ClassifySchoolRoles` returns the admin and
  Lehrkraft classification the staff messaging picker ranks by.

The consumers keep their interfaces and statement shapes; an unbound owner
query fails closed with an error. The pattern is the #2720 active-account
query. The five `tables.foreign-read` keys still listed under #2721 at the
base commit are removed. The other twelve keys the issue lists were already
resolved by earlier waves: the three `filestore` keys by #2707, the six `iot`
keys by the leased outbox, and the three `users` permission keys
(`auth.account_permissions`, `auth.permissions`, `auth.role_permissions`) by
the staff membership cutover. Table shapes, RLS policies and policy
ownership are unchanged.

## Reproduce

From the repository root:

```sh
scripts/run-go-toolchain.sh go -C backend test ./database/repositories \
  -run '^TestIdentityMembershipRuntimeEvidence$' -count=1 -v
```

The baseline is the merge base `4be8d0f858492c0552096bc85c8d254b9420f303`.
The harness only uses constructors that exist on both sides
(`NewFactoryWithPeopleDirectory`, `NewStaffMessagingTestRepositories`), so the
unchanged file `backend/database/repositories/identity_membership_runtime_evidence_test.go`
was copied into a temporary worktree of that commit and run with the same
command; the worktree was removed afterwards. Both runs happened back to back
on an idle machine. Raw records:
[baseline](identity-membership-2721.baseline.raw.json),
[candidate](identity-membership-2721.raw.json).

Each sample opens its own transaction: a school transaction for the tenant
reads and an administrative transaction for the parent portal, login school
list and operator dashboard reads. Fixtures (one guardian chain with an
active mapping and the guardian role, one staff member with the Lehrkraft
system role) are outside the timer.

## Results

Milliseconds, nearest-rank p50/p95 over 30 samples. Statements include the
transaction scope statements and were identical in every sample.

| Operation | Statements base/cand | p50 base/cand | p95 base/cand | Returned rows |
|---|---:|---:|---:|---:|
| `guardian_account_permission` | 5 / 5 | 1.291 / 1.140 | 1.968 / 1.338 | 1 |
| `guardian_email_permission` | 5 / 5 | 1.321 / 1.100 | 1.668 / 1.418 | 1 |
| `guardian_access_filter` | 5 / 5 | 1.213 / 1.077 | 1.784 / 1.498 | 1 |
| `portal_profile_reachability` | 5 / 5 | 1.733 / 1.397 | 2.203 / 1.897 | 1 |
| `messageable_guardians` | 5 / 5 | 1.298 / 1.158 | 1.733 / 1.781 | 1 |
| `messageable_staff` | 7 / 7 | 1.773 / 1.618 | 2.161 / 2.140 | 1 |
| `is_messageable_staff` | 7 / 7 | 1.799 / 1.589 | 2.544 / 2.169 | 1 |
| `staff_role_kinds` | 5 / 5 | 1.173 / 1.055 | 1.560 / 1.426 | 2 |
| `parent_children` | 7 / 7 | 1.900 / 1.723 | 2.466 / 2.245 | 1 |
| `parent_submit_status` | 7 / 7 | 1.862 / 1.635 | 2.601 / 1.999 | 1 |
| `schools_of_account` | 4 / 4 | 1.049 / 0.899 | 1.727 / 1.269 | 1 |
| `operator_stats` | 6 / 6 | 1.698 / 1.569 | 2.392 / 2.109 | 1 |
| `operator_school_summaries` | 9 / 9 | 2.251 / 1.924 | 3.573 / 2.338 | 2 |

- Errors: 0/30 per operation on both sides; every measured sample and every
  warmup succeeded.
- Written rows: zero for every sample; all operations are reads.
- Pool wait: zero pool waits in every measured sample on both sides. The
  unit-of-work observer recorded 490 zero-wait pool acquisition events
  including warmups (5.323 ms base, 3.957 ms candidate in total).
- Lock wait: the lock-wait hook recorded no lock acquisition event; none of
  the operations takes a lock. The workload is sequential, so this is not
  contention evidence.
- Transactions: 455 commits, zero rollbacks and zero retries on both sides.
- Deadlocks: the database counter did not move on either side.
- Worker duration, retries and backlog: not applicable, no worker path.

The latency differences are local samples within laptop noise, not a claimed
improvement. The owner subqueries keep every consumer statement a single
round trip, so the statement counts are unchanged by construction.

## Failure and isolation evidence

- `TestMembershipQueries_TwoTenantIsolation` (`database/repositories/auth`)
  proves that a school transaction sees only its own role assignments and
  role classes, that another school's admin role is not classified, and that
  an inactive mapping is never a membership.
- `TestIdentityMembership_PairsTheRelationshipSchool`
  (`database/repositories/users`) proves that an active mapping or guardian
  role at another school does not authorize the permission checks, the
  portal reachability or the parent message recipients at the relationship's
  school. Replacing one row-value predicate by an account-only predicate
  makes it fail.
- `TestIdentityMembership_UnboundQueriesFailClosed` and
  `TestIdentityMembership_RoleClassFailureIsNotSwallowed` prove that a
  missing owner query or a failing owner read surfaces as an error.
- Rollback: the operations only read. A failing owner read returns its error
  to the caller, whose transaction then rolls back unchanged; no write path
  changed.
