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
cleanup preview. The orphaned `auth.accounts_parents` model, repository,
service methods, `/auth/parent-accounts` routes and frontend client are removed;
no production code reads or writes that table. Data ownership in
`policy.json` is unchanged.

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

## Failure, rollback and contract evidence

1. `TestOperatorSessionWritesJoinAmbientTransaction` injects a failure after a
   session insert and login stamp inside the administrative transaction; both
   writes roll back and the retry succeeds from a clean state.
2. `TestOperatorSessionRotationHandoff` proves a second hand-off on the same
   session returns `ErrOperatorSessionRotated` instead of silently re-rotating,
   and that rotated predecessors stay as replay evidence until their JWT expires.
3. `TestOperatorAuditLogRepository_AppendsWithoutTenantAndJoinsTransaction`
   proves the operator ledger appends without a tenant and rolls back with the
   caller's transaction.
4. `TestPersonRepository_FindWithAccountRequiresAccountLookup` and the staff
   messaging and guardian profile reads fail closed without the owner query.
5. Existing suites cover the unchanged contracts: operator login, MFA lockout,
   refresh replay detection and password-change revocation
   (`services/platform`), session validation across all four portals
   (`services/auth`), parent dashboard listing (`modules/careplan/integration`)
   and staff messaging reachability (`modules/communication`).

## Rollback and limits

Reverting the composition restores the previous adapters against unchanged
table shapes. The removed parent-account endpoints had no frontend caller and
the table is empty in production, so no data is affected.
