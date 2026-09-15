# Account session evidence (#2720)

Issue: [#2720](https://github.com/moto-nrw/project-phoenix/issues/2720).
Measured locally on 2026-09-15, Go 1.27.0, macOS arm64, PostgreSQL 17.11,
isolated test clone, sequential concurrency 1, five warmups and 30 measured
calls per operation. The accepted checkpoint remains #3020; this document
does not accept another checkpoint and is not a staging or production window.
The operator half of the ticket is recorded in
[operator-sessions-2720.md](operator-sessions-2720.md).

## Cutover and ownership

Identity & Access (`modules/identityaccess`) now serves the account refresh
sessions (`auth.tokens`) behind tenant, parent and school login, refresh,
tenant switching, logout, session validation and revocation through one
public `AccountSessionAccess` capability. The retained
`models/auth.TokenRepository` contract is a compatibility adapter in the
legacy composition (`database/repositories/account_sessions.go`) over that
capability, bound at construction for the serving root, the cleanup CLI and
the auth test compositions; the former `database/repositories/auth` token
repository is deleted, and the retained contract shrinks to the methods the
auth service calls (the generic `FindByID`, `Update` and `Count` are gone).

Session statements apply the tenant filter the runtime scoped the caller to,
exactly as the retained repository did: tenant-scoped callers see and revoke
only their school's sessions, the tenantless pre-authentication flows (login,
refresh, logout) resolve every school's rows, and the session cap and family
retirement act across every school inside the administrative login
transaction. Session writes join the caller's transaction and never open one
of their own. `auth.accounts` and `auth.account_tenants` keep their
identity-access-owned repositories; data ownership in `policy.json` is
unchanged.

## Local runtime workload

Reproduce from the repository root:

```sh
scripts/run-go-toolchain.sh go test -C backend ./modules/identityaccess/integration \
  -run TestAccountSessionRuntimeEvidence -count=1 -v
```

Each sample runs the public capability inside one administrative transaction,
mirroring the auth service: login (session mint, hand-off sweep, portal
session cap), refresh (unlocked lookup, locked lookup, family inspection,
successor mint, rotation hand-off, hand-off sweep), switch (retirement of the
presented family, mint, sweep, cap) and revoke (lookup, family delete,
tenant-scoped account revoke). Percentiles use nearest rank. Lossless
evidence: [raw JSON](account-sessions-2720.raw.json).

| Operation | Statements | p50 | p95 | DML rows | Errors |
| --- | ---: | ---: | ---: | ---: | ---: |
| Login | 6 | 1.526 ms | 2.224 ms | 1 | 0/30 |
| Refresh | 9 | 2.301 ms | 2.916 ms | 2 | 0/30 |
| Switch | 7 | 1.819 ms | 2.190 ms | 2 | 0/30 |
| Revoke | 6 | 1.374 ms | 3.409 ms | 3 | 0/30 |

Statement counts include the three transaction statements of the
administrative unit of work. Query counts and DML row counts are identical
across all 30 samples per operation. The harness records a failed sample as
an error instead of aborting, so the 0/30 error rate per operation is an
observation of the run, not a value the harness forces. Zero pool waits in the measured samples,
140 committed transactions and zero rollback events including warmups, and
the database deadlock counter changed by zero. No latency SLO or query budget
is defined for these flows; the values are observations, not thresholds or a
before/after benchmark.

Including warmups, the trace records 140 lock-acquisition statement events
totalling 56.022 ms and 140 pool-wait events totalling 0.147 ms. The existing
hook measures SQL acquisition duration, an upper bound on lock wait, not
wait-only time. This workload is sequential, so it contains no contention:
these values are a floor, not evidence about behaviour under load. Zero
transactions reported a retry, so there were no serialization retries.
Duplicate-prevention conflicts do not apply to these flows: no session write
is an upsert or a conflict-tolerant insert, and `auth.tokens` carries no
unique constraint beyond its primary key (the migrations define only plain
indexes on account, token, expiry, family and rotation), so the family
conflict retry in the auth service login path is a latent branch no write can
reach. The one conflict-shaped case is the rotation hand-off, whose second
attempt is rejected with the stable `ErrAccountSessionRotated` error rather
than absorbed; driver errors otherwise keep their text through the adapter.

## Failure, rollback and contract evidence

1. `TestAccountLoginMintRollsBackAfterEachWrite` injects a failure after each
   of the three authoritative writes of a login or tenant switch in turn
   (family retirement, session mint, cap eviction). Each case proves the
   retired family keeps its expiry, the minted session never existed and no
   session was evicted, then that the full retry commits and evicts the
   retired family first. `TestAccountRefreshRollsBackAfterEachWrite` does the
   same for the successor insert, the rotation hand-off and the
   expired-predecessor sweep. `TestAccountRevocationRollsBackWithItsCaller`
   proves a family revocation rolls back with its caller and that a repeated
   revoke is a no-op, not an error.
2. `TestAccountSessionRotationHandoff` proves a second hand-off on the same
   session returns `ErrAccountSessionRotated` instead of silently
   re-rotating, that rotated predecessors stay as replay evidence until their
   JWT expires, and that a retired family caps its live session.
3. `TestAccountSessionsAreScopedToTheCallerTenant` is the hermetic two-tenant
   test for `auth.tokens`: a caller scoped to one school cannot find, lock,
   rotate, revoke or delete another school's session, while the tenantless
   pre-authentication flows resolve both. `TestAccountSessionRevocations`
   proves the tenant-scoped revocation refuses a tenantless caller with
   `ErrTenantRequired` instead of widening, and keeps the other school's
   session inside an administrative transaction re-scoped to one school.
4. `TestAccountSessionCapKeepsPortalGroupsApart` and
   `TestAccountSessionCapInAdminTransactionSpansSchools` pin the session-cap
   contract: tenant and org share the staff allowance, parent and school
   caps stand alone, unknown legacy rows stay isolated, rotated and expired
   rows never count, the session closest to expiry goes first, a retired
   family goes before a session on another device, and the administrative
   login transaction caps across every school of the account.
5. The adapter tests in `database/repositories/auth/token_adapter_test.go`
   preserve the retained contract shape: validation on the retained model,
   `DatabaseError` on every failure, `IsNoRows` on missing lookups, the
   tenant-only account revoke, the account-wide wipe, the cutoff revoke that
   includes refresh successors, and the cleanup preview matching the sweep.
   `TestTenantIsolation_TokenVisibility` keeps its two-tenant coverage over
   the retained contract.
6. Existing suites cover the unchanged contracts: tenant, parent and school
   login, MFA gate, refresh replay and recovery, tenant switching, logout,
   four-portal session validation, staff offboarding preview and the
   account-wide wipe reconciliation (`services/auth`), the auth routes and
   route goldens (`api/auth`), the cleanup CLI (`cmd`), the operator
   provisioning revocations (`services/platform`) and the composition,
   architecture, query-budget and hermetic-test ratchets.

## Rollback and limits

Reverting the composition restores the previous repository against unchanged
table shapes; no schema changed and nothing is deleted irreversibly, so the
cutover needs no tracer window before its old provider goes.

This delivery completes the `auth.tokens` half of #2720 and is not a full
closure of the ticket. The login, refresh, switch and revocation
orchestration (password check, MFA gate, claims assembly, audit; the
`services/auth` files #3225 attributes to #2720) stays at its legacy path
and keeps its identity-access-owned `auth.accounts` and
`auth.account_tenants` repositories: the architecture policy grants
`identity-access/application` no import of the module's public package, and
adding that rule is a ratchet loosening, so those files can only move into
`modules/identityaccess` as a whole, in the ordering the #2725 re-cut gives
(#3224, then #3225 with the #2720 login files, then #3226 for the
persistence and token layer). That relocation remains open under #2720. The
parent-account surface keeps its public contract:
`auth.accounts_parents` has no target owner, and the architecture README
forbids inventing one to silence the check, so its two ratchet keys
(`tables.unclassified` for the table and the dynamic `tables.unresolved`
`auth.Where` filters of its repository) stay in the baseline.
