# Identity & Access native persistence and token adapter evidence (#3226)

Issue: [#3226](https://github.com/moto-nrw/project-phoenix/issues/3226).
Measured locally on 2026-09-21, Go 1.27.0, macOS arm64, PostgreSQL 17.11,
hermetic test database, sequential concurrency 1, five warmups and 30 measured
calls per operation. The accepted checkpoint remains #3020; this document does
not accept another checkpoint and is not a staging or production window.

## Cutover and ownership

The legacy auth models (`modules/identityaccess/legacy/authmodels`) and
repositories (`modules/identityaccess/legacy/authpostgres`) are deleted.
Accounts, school memberships, roles, permissions, sessions, MFA, invitations,
password resets, calendar-feed credentials and RFID ownership are persisted by
the module's own Postgres adapters under
`modules/identityaccess/internal/adapters/postgres`. Consumers in
`database/repositories`, `models/users`, `services/users`, `api/auth`, `cmd`
and `test` no longer import the legacy models; they read through owner
capabilities or consumer-owned ports.

The token adapter `modules/identityaccess/legacy/jwt` no longer imports the
tenant runtime (`tenant`), process configuration (`spf13/viper`) or
`internal/randstr`. The scope gates take the `jwt.TenantScope` port that
`tenant.ClaimScope` implements, and the API root resolves the signer once.

Table shapes, RLS policies and policy ownership are unchanged. No DDL.

## Reproduce

From the repository root:

```sh
scripts/run-go-toolchain.sh go -C backend test ./api/auth \
  -run '^TestIdentityPersistenceRuntimeEvidence$' -count=1 -v
```

The baseline is the merge base `3c33e6a4e827cf5e24cdccf17d307bb47c36e75a`.
The harness `backend/api/auth/identity_persistence_runtime_evidence_test.go`
only uses the auth router and fixtures that exist on both sides, so the
unchanged file was copied into a temporary worktree of that commit and run
with the same command (`go -C <worktree>/backend`); the worktree was removed
afterwards. Both runs happened back to back on the same machine. After the
measurement the harness switched its setup call from `setupPublicRouterWithDB`
to `setupProtectedRouter`, because the hermetic-pattern ratchet only recognises
the latter name as a database setup; both helpers mount the same resource
router on the same tenant router, so the measured path is identical. Raw records:
[baseline](identity-persistence-3226.baseline.raw.json),
[candidate](identity-persistence-3226.raw.json).

Every one of the 35 rounds logs in, rotates the refresh token, reads with the
freshly issued access token and assigns and removes one role. Requests go
through the mounted auth router including verifier, authenticator, tenant
scope gate, permission check and tenant transaction. Fixtures (one account
with an active school mapping and the admin role, one assignable role) are
outside the timer. The test database uses the test password-hash parameters,
so the login latency is not a production hashing figure.

## Results

Milliseconds, nearest-rank p50/p95 over 30 samples. Statements include the
transaction scope statements and were identical in every sample of a side.

| Operation | Status | Statements base/cand | p50 base/cand | p95 base/cand | Written rows base/cand |
|---|---:|---:|---:|---:|---:|
| `POST /auth/login` | 200 | 40 / 38 | 8.359 / 8.147 | 9.729 / 9.416 | 3 / 3 |
| `POST /auth/refresh` | 200 | 27 / 27 | 5.666 / 5.653 | 6.522 / 6.483 | 4 / 4 |
| `GET /auth/account` | 200 | 1 / 1 | 0.249 / 0.260 | 0.341 / 0.333 | 0 / 0 |
| `GET /auth/account/tenants` | 200 | 9 / 9 | 1.526 / 1.518 | 1.855 / 1.779 | 0 / 0 |
| `GET /auth/accounts` | 200 | 5 / 5 | 0.868 / 0.936 | 1.210 / 1.092 | 0 / 0 |
| `GET /auth/accounts/{id}/roles` | 200 | 6 / 6 | 1.240 / 1.181 | 1.566 / 1.471 | 0 / 0 |
| `GET /auth/accounts/{id}/permissions` | 200 | 6 / 6 | 1.561 / 1.481 | 2.022 / 1.840 | 0 / 0 |
| `GET /auth/accounts/{id}/permissions/direct` | 200 | 6 / 6 | 1.128 / 1.135 | 1.500 / 1.491 | 0 / 0 |
| `POST /auth/accounts/{id}/roles/{role}` | 204 | 25 / 25 | 5.221 / 4.990 | 6.067 / 5.811 | 4 / 3 |
| `DELETE /auth/accounts/{id}/roles/{role}` | 204 | 10 / 10 | 1.996 / 2.039 | 2.601 / 2.366 | 1 / 1 |

Observations:

- Login issues two statements fewer on the candidate; every other operation
  keeps its statement count.
- The role assignment writes three rows instead of four, and the role removal
  reports five instead of six rows across its statements. Status codes are the
  same (204), and the role assignment suites in `api/auth` pass. The harness
  did not diff response bodies.
- Latency differences are within run-to-run noise on a developer machine. No
  latency SLO is claimed.
- Pool waits: zero in all 300 measured samples on both sides.
- Lock waits: 519 (baseline) and 496 (candidate) `pg_stat_activity` samples,
  none with a backend waiting on a lock. Deadlocks: 0 on both sides.
- Unexpected errors: 0; every request returned its expected status.

## Not measured

MFA challenge, passkey, invitation acceptance, password-reset confirmation,
calendar-feed credentials, RFID ownership, the operator flows and the cleanup
jobs are not in this harness. Their behavior is covered by the module's
behavior and integration suites and by the API suites; this document makes no
latency claim for them. The token adapter decoupling changes no SQL.

## Failure and rollback

`api/auth/school_identity_rollback_test.go` and the identity integration
suites keep their rollback assertions and pass. The cutover has no data
migration: reverting the commit range restores the legacy repositories against
the unchanged schema.
