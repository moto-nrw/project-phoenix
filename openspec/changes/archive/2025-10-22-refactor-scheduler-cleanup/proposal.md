# Refactor Scheduler Cleanup Interfaces

## Why
- `services/scheduler/scheduler.go` currently relies on reflection (`MethodByName`) plus `interface{}` fields to call cleanup routines. Renaming or changing a cleanup method compiles but silently breaks the scheduler.
- The reflection helper obscures dependencies between the scheduler and the auth/invitation services, so editors, tests, and code search cannot surface the coupling.
- We now have multiple cleanup entry points (scheduler, CLI commands, potential future cron jobs). A typed, reusable cleanup registry will keep them consistent and easier to share.

## Current Behavior & Pain Points
- Scheduler accepts generic `interface{}` service handles and invokes cleanup methods by string name, coercing return values at runtime.
- Errors such as missing methods or signature drift become runtime failures logged only when the job executes.
- Adding a new cleanup routine requires duplicating string constants and reflection glue instead of declaring intent in code.

## Proposed Changes
- Define narrow interfaces in the scheduler package (e.g., `AuthCleanup`, `InvitationCleanup`) that declare the exact cleanup methods the scheduler needs.
- Replace the `interface{}` fields on `Scheduler`/`NewScheduler` with those interfaces to enforce compile-time conformance.
- Introduce a `CleanupJob` struct that captures `Description` and `Run(context.Context) (int, error)` and build a slice of jobs during scheduler construction.
- Update `RunCleanupJobs` to iterate over the job slice and remove all reflection logic while preserving logging and error aggregation.
- Adjust call sites (`api/server.go`, tests) to pass services that already satisfy the new interfaces.
- Document the new contract in the scheduler spec (scheduler cleanup must use the typed job registry, no reflection) and surface the reusable job slice for other runners if needed.

## Out of Scope
- Adding or removing individual cleanup operations (token types, invitation policies).
- Changing the scheduling cadence or configuration flags.
- Refactoring unrelated scheduler tasks (session end, checkout processing).

## Risks & Mitigations
- **Risk:** Additional interfaces could drift from service implementations. *Mitigation:* Locate interfaces in the scheduler package and rely on Go's compile-time checks; add a small compile-time assertion in tests if useful.
- **Risk:** Consumers outside the scheduler may depend on the reflective behavior. *Mitigation:* Audit usages (CLI already calls services directly) and document the new helper for reuse.

## Validation Plan
- Run `go test ./services/scheduler` and existing scheduler-related integration tests.
- Add/extend unit coverage to ensure the job slice is built correctly and logs counts/errors as expected.
- Verify manual execution (`cmd/cleanup` or a new helper) can reuse the typed job registry without reflection.
