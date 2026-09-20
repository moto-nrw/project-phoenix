# Current verification state

Base: `4aa36c315fc19e8ae8dda50193ffa34843e25575` (development including #3441).
No production Contract or monitoring installation has been performed.

## Completed checks

- `scripts/run-go-toolchain.sh scripts/test-changed.sh origin/development`:
  passed, 166 affected backend packages, no frontend changes, zero leftover
  clones. This final pass includes all tracking-gate and fixture corrections.
- Pinned `golangci-lint run --timeout 10m`: zero issues after the last Go edit.
- `scripts/backend-architecture.sh check`: passed, composition targets unchanged
  at 667 and existing legacy findings unchanged at 682.
- Schema-3 evidence validation against the immutable base: passed, 15 files,
  zero raw additions/removals.
- Real PostgreSQL regressions reject disabled tracking, disabled utility
  tracking and narrowing application-role overrides.
- SHA-256 Contract core measurement: 105.311875 ms, 26 BUN query events,
  1,448 complete owner records preserved, 2,830 history rows preserved,
  16 sick flags with timestamps preserved, zero pool waits/deadlock delta.
- Local Loki/Grafana exercise: both exact provisioned rules initially healthy,
  three matching synthetic old-object errors, both rules firing, both webhook
  notifications delivered only to a local container, disposable project removed.
- Environment synchronization, agent-context checker and whitespace checks passed.
- `node --test scripts/release-backup.test.mjs scripts/student-contract-monitor.test.mjs`:
  all 36 deployment/recovery/collector tests passed.

After the agreed policy and final calendar-boundary correction,
`scripts/run-go-toolchain.sh scripts/test-backend.sh` passed: 26,714 tests,
two explicitly reported skips, 103.000 seconds, zero leftover clones.
Valid Go test cache results were reused. The two skips require a configured
Web Push environment and a seeded stack respectively; those integrations are
not established by this run. Final linter and architecture checks
also passed. A preceding run correctly failed the forbidden migration calendar
import; the final implementation uses PostgreSQL's Berlin conversion and adds
no architecture exception.

The required non-fast changed-test command also passed after this final policy
implementation, covering 166 affected packages with zero leftover clones.

## Standards

The MD5 violation was corrected to SHA-256. Follow-up review reported no new
confirmed standards violations. Optional maintenance observation: the query
fingerprint SQL appears in migration, collector and operator capture SQL and
must be kept aligned.

## Spec

The live direct-query gate and continuous alerting findings are resolved.
The follow-up tracking-configuration finding is resolved, including role/database
overrides. The bounded follow-up reported no further concrete findings.
The user subsequently approved at least 24 hours without old access, including
one complete regular school day and the relevant background jobs. That policy
is now implemented with school-day interval/artifact and job-artifact checks.
Both review axes reported no concrete defects in the final policy delta.
The calendar check is implemented in the live PostgreSQL gate using
`AT TIME ZONE 'Europe/Berlin'`, rather than importing a domain calendar package
across the migration boundary. It is repeated under the Contract locks.

## Operational handoff

1. The user's operating decision is recorded and implemented. Focused policy
   tests passed, including exact 24-hour acceptance and rejection of partial,
   missing, weekend or out-of-window school evidence and missing job evidence.
2. No production execution is included. The ordinary CI deployment invocation
   has no reviewed evidence file; the later Contract deployment must explicitly
   supply that file to the documented deployment script interface.
3. Later production execution requires fresh database/release-bound evidence,
   backup/restore proof and deployed monitoring. Local synthetic results are
   not substitutes for that operational evidence.

Initialized databases require the approved operating evidence before execution.
This record does not authorize destructive production execution.
