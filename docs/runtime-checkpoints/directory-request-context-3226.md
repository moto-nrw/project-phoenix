# People Directory request-context migration

## Scope

This is consumer-retirement evidence for #3226, not evidence that its full
Identity/JWT cutover is complete. Production `services/users` no longer imports
the legacy JWT package. Root composition still binds that provider to narrow,
mandatory permission and audit-actor ports.

The changed boundary reads context values only. SQL, tenant scoping,
transactions, audit persistence and authorization decisions remain unchanged.
No endpoint performance measurements were taken or inferred from test timings.

## Observed verification

The following command passed all five packages:

```sh
scripts/run-go-toolchain.sh go -C backend test ./services/users ./services ./services/enrollment ./modules/careplan/legacy/carelifecycle ./workflows/parentportal/legacy
```

Existing tests retain full-name and username-fallback attribution, reject names
from mismatched accounts, and check persisted caregiver actor IDs and raw scope.
The permission-port regression checks that an empty result cannot be bypassed
by legacy admin claims and that the rejected write marks rollback.

Architecture evaluation found precisely one newly stale production debt entry:
`services/users` to `modules/identityaccess/legacy/jwt`. That resolved entry is
removed. Test-only imports and the serving root's provider remain explicit debt.
