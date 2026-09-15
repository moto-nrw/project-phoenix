# Operator identity and session evidence (#2720)

Issue: [#2720](https://github.com/moto-nrw/project-phoenix/issues/2720).
Measured locally on 2026-09-10, Go 1.27.0, macOS arm64, PostgreSQL 17.11,
isolated test clone, sequential concurrency 1, five warmups and 30 measured
calls per operation. The accepted checkpoint remains #3020; this document
does not accept another checkpoint and is not a staging or production window.

## Cutover and ownership

Identity & Access (`modules/identityaccess`) now serves platform operators
(`platform.operators`) and their revocable refresh sessions
(`platform.operator_refresh_tokens`) through one public capability. The
retained `platform.OperatorRepository` and
`platform.OperatorRefreshTokenRepository` contracts are compatibility adapters
in the legacy composition over that capability; the former
`database/repositories/platform` operator, refresh-token and audit-log
repositories are deleted. `platform.operator_audit_log` is appended through
the Audit owner's appender (`models/audit.OperatorAuditEntry`), so revocation
evidence joins the same administrative transaction as the session delete.

The foreign `auth.accounts` reads of the People Directory, Care Plan parent and
CLI packages are replaced by owner queries: the account lookup and the
active-account subquery of `database/repositories/auth`, the public
`FindAccount` for the parent dashboard, and `CountExpiredTokens` for the
cleanup preview. The legacy `auth.accounts_parents` model, repository, service
methods, the six `/auth/parent-accounts` routes and the frontend client stay
unchanged. Data ownership in `policy.json` is unchanged.

## Local runtime workload

Reproduce from the repository root:

```sh
scripts/run-go-toolchain.sh go test -C backend ./modules/identityaccess/integration \
  -run TestOperatorRuntimeEvidence -count=1 -v
```

Each sample runs the public capability the operator auth service calls:
login (address lookup, session mint, login stamp), refresh (the rotation
sequence inside one administrative transaction) and revoke (family delete in
one administrative transaction). Percentiles use nearest rank. Lossless
evidence: [raw JSON](operator-sessions-2720.raw.json).

| Operation | Statements | p50 | p95 | DML rows | Errors |
| --- | ---: | ---: | ---: | ---: | ---: |
| Login | 3 | 0.778 ms | 1.085 ms | 2 | 0/30 |
| Refresh | 9 | 2.279 ms | 3.126 ms | 2 | 0/30 |
| Revoke | 4 | 0.832 ms | 1.172 ms | 2 | 0/30 |

Query counts are identical across all 30 samples per operation. Zero pool
waits in the measured samples, 70 committed transactions and zero rollback
events including warmups, and the database deadlock counter changed by zero.
No latency SLO or query budget is defined for these flows; the values are
observations, not thresholds or a before/after benchmark.

Including warmups, the trace records 70 lock-acquisition statement events
totalling 21.724 ms and 70 pool-wait events totalling 0.044 ms. The existing
hook measures SQL acquisition duration, an upper bound on lock wait, not
wait-only time. This workload is sequential, so it contains no contention:
these values are a floor, not evidence about behaviour under load.
Zero transactions reported a retry, so there were no serialization retries.
Duplicate-prevention conflicts do not apply to these flows: no operator or
session write is an upsert or a conflict-tolerant insert. The one
conflict-shaped case is the rotation hand-off, whose second attempt is
rejected rather than absorbed, and is counted as an error in test 2 below,
not as a conflict.

## Failure, rollback and contract evidence

1. `TestOperatorRefreshRollsBackAfterEachWrite` injects a failure after each of
   the four authoritative writes of a refresh in turn (successor insert,
   rotation hand-off, expired-predecessor sweep, login stamp). Each case
   proves the predecessor survives un-rotated, the successor never existed,
   the family still ends at the predecessor and no login stamp was recorded,
   then that the full retry commits.
   `TestOperatorRevocationRollsBackWithItsCaller` does the same for family
   revocation and proves a repeated revoke is a no-op, not an error.
   `TestOperatorSessionWritesJoinAmbientTransaction` covers the login pair.
2. `TestOperatorSessionRotationHandoff` proves a second hand-off on the same
   session returns `ErrOperatorSessionRotated` instead of silently re-rotating,
   and that rotated predecessors stay as replay evidence until their JWT expires.
3. `TestOperatorAuditLogRepository_AppendsWithoutTenant` proves the operator
   ledger appends and reads back without a tenant in the runtime, which is
   what makes it a platform-scoped ledger rather than a tenant one.
4. `TestPersonRepository_FindWithAccountRequiresAccountLookup` and the staff
   messaging and guardian profile reads fail closed without the owner query.
5. `TestOperatorRowsAreVisibleFromEveryTenantContext` pins the isolation
   contract of the two migrated tables: they carry no tenant column, so an
   operator and its session resolve identically from the test's own tenant
   and from a foreign one. The tenant-scoped tables this owner also touches
   keep their existing two-tenant coverage in
   `database/repositories/auth` (`account_tenant`, `account_role`,
   `token`, `invitation_token` repository tests).
6. Existing suites cover the unchanged contracts: operator login, MFA lockout,
   refresh replay detection and password-change revocation
   (`services/platform`), session validation across all four portals
   (`services/auth`), parent dashboard listing (`modules/careplan/integration`)
   and staff messaging reachability (`modules/communication`).

## Rollback and limits

Reverting the composition restores the previous adapters against unchanged
table shapes; no schema changed and nothing is deleted irreversibly, so the
cutover needs no tracer window before its old provider goes.

This is a partial delivery of #2720. The parent-account surface keeps its
public contract: `auth.accounts_parents` has no target owner, and the
architecture README forbids inventing one to silence the check, so its two
ratchet keys (`tables.unclassified` for the table and the dynamic
`tables.unresolved` `auth.Where` filters of its repository) stay in the
baseline and #2720 stays open. Tenant, parent and school login, refresh,
switching and revocation are not part of this change.
