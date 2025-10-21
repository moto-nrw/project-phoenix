## 1. Specification
- [x] 1.1 Update scheduler capability spec to describe typed cleanup interfaces and job registry requirements.

## 2. Implementation
- [x] 2.1 Define `AuthCleanup` and `InvitationCleanup` interfaces and `CleanupJob` struct in `services/scheduler`.
- [x] 2.2 Refactor `Scheduler` construction and `RunCleanupJobs` to use typed jobs (no reflection) and update call sites.
- [x] 2.3 Add/adjust unit tests covering job execution and error handling.

## 3. Verification
- [x] 3.1 Run `go test ./services/scheduler ./cmd/...`.
- [x] 3.2 Document reuse guidance (README or inline comments) for the cleanup job registry.
