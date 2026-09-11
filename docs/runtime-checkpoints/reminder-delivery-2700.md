# Reminder Delivery migration evidence

Issue: [#2700](https://github.com/moto-nrw/project-phoenix/issues/2700).
Implementation baseline: `5d6c3dba59`. Measured locally on 2026-09-05.

This flow-specific evidence supplements the accepted runtime checkpoint
[#3019](https://github.com/moto-nrw/project-phoenix/issues/3019#issuecomment-5552473019).
It does not claim a staging or production observation window.

## Cutover

`workflows/reminderdelivery` owns the public `Query` and scheduler `Command`.
The HTTP handler consumes `CallerQuery`; scheduler wiring consumes the same
module's `Command`. The staff evaluator and guardian delivery preparation live
in the workflow's internal application package. Appointment candidate planning
and the existing recurrence engine live in the appointments owner. Calendar
supplies checked guardian projections and notification adapters, not a second
reminder evaluator or command.

Email enqueue and push claims share one detached tenant UnitOfWork. Push
dispatch runs after commit, with current appointment revision, occurrence,
guardian access, and preference rechecks. No schema migration, fallback read
chain, or dual-write provider was introduced.

Static cleanup evidence:

- All 30 ratchet keys listed in #2700 are absent.
- Legacy violations: **2447 → 2416**, with 31 removals and zero additions.
- Composition field/setter targets: **847 → 847**.
- Policy `owners` and `data_objects` are unchanged.
- `services/reminders` and `services/calendar/reminders.go` are deleted.
- No Go source imports `github.com/moto-nrw/project-phoenix/services/reminders`.
- The only production `EnqueueDueAppointmentReminders` implementation is the
  workflow command. Its only production caller is the scheduler.
- Generated composition inventories retain immutable historical evidence and
  refresh current locations; they do not add legacy callers.

## Contracts and isolation

The production-router golden, HTTP reminder tests, scheduler tests, calendar
integration tests, and appointments recurrence tests exercise the cutover.
They retain the existing JSON, authorization/error mapping, half-open reminder
window, midnight/DST behavior, moved occurrences, revision-sensitive claims,
channel preferences, and permission rechecks.

`TestCalendarServiceIntegration_ReminderPreparationRollsBackEmailAndPushClaim`
injects an error after the real claim insertion, which follows real email
enqueue. Neither row survives and no push is dispatched. A successful retry
persists both; replay duplicates neither row nor push. The production email
compatibility facade returns the existing row ID on duplicate enqueue, so the
historical command count remains one on that replay. The test checks actual
row counts independently of that return value.

A regression subtest verifies a push-only binding without either email
callback. It failed before independent preference resolution was fixed and
passes afterward, including post-commit revalidation.

`TestReminderNamedTablesIsolateTwoTenants` seeds both tenants and verifies:

| Table | Preserved boundary and evidence |
| --- | --- |
| `calendar.appointments` | Unfiltered tenant-role reads expose only own rows; cross-tenant updates affect zero rows. |
| `calendar.appointment_recipients` | Same RLS read/write checks. |
| `platform.email_outbox` | Same RLS read/write checks. |
| `iot.push_subscriptions` | Same RLS read/write checks. |
| `calendar.appointment_reminder_push_deliveries` | Direct tenant-role access is denied. Existing SECURITY DEFINER claim/release functions enforce tenant scope: foreign claims fail, foreign releases do not remove the owning tenant's claim. |

The claim table intentionally has no direct tenant grants or table RLS. Its
existing function-mediated boundary is tested, not replaced with a different
policy. Read-failure tests preserve the underlying error through candidate,
recurrence, override, lock/reload, and recipient lookup failures. Command tests
also cover preparation and commit failure without post-commit dispatch.

## Local runtime sample

Reproduce from `backend/`:

```sh
CGO_ENABLED=0 ../scripts/run-go-toolchain.sh go test -p 4 -parallel 8 \
  ./services/calendar -run TestReminderRuntimeEvidence -count=1 -v
```

The hermetic test uses an isolated PostgreSQL clone, five warmups and 30
sequential measured calls per operation. The final run overlapped the repository
changed-package check and lint, so host load affected latency. Percentiles use
nearest rank. Query
counts exclude fixture setup and lock-sampling queries. Each command sample
uses a fresh appointment revision, one reachable guardian, the real durable
email/claim stores, and a deterministic local push sink. External transport
and production load are outside this sample; checkpoint #3019 covers the
delivery worker separately.

| Stable operation | Statements min/max | p50 | p95 | Error rate | Pool waits / duration |
| --- | ---: | ---: | ---: | ---: | ---: |
| `reminder-delivery.query.admin` | 2 / 2 | 16.481 ms | 22.620 ms | 0/30 | 0 / 0 ms |
| `reminder-delivery.command.prepare` | 29 / 29 | 110.449 ms | 265.761 ms | 0/30 | 0 / 0 ms |

The query fixture has eight rooms and no present students. It measures that
read path, not a populated attendance cohort. Separate existing query-budget
tests cover 3 → 8 supervised rooms and 3 → 8 due appointments without read-count
growth. The appointment scan measured 11 reads for both cohort sizes.

PostgreSQL `wait_event_type = 'Lock'` sampling observed zero waiting backends:
50 query samples (maximum sample gap 56.925 ms) and 764 command samples (maximum
gap 60.545 ms). Sampling is not proof that no shorter wait occurred. These are
local smoke measurements, not before/after performance or production SLO claims.

## Rollback

Revert the complete migration commit, restoring its provider, wiring, and
policy changes together. Do not revert wiring alone after deleting the old
provider. Existing rows remain readable because table shapes, idempotency
keys, ownership, and isolation behavior are unchanged. Keep durable outbox rows
and claims; deleting them would permit duplicate delivery. Stop the worker
before rollback and restart it only after the previous provider is restored.

Local contract and runtime tests are the observed window for this artifact.
A deployment tracer window has not been observed here. Retain this commit and
its parent as the rollback pair and monitor scheduler failures, outbox retries,
and duplicate-claim behavior during rollout before discarding rollback assets.

## Verification

- Full backend lint after the final review correction: zero issues.
- Full backend suite: 24,773 tests, five existing environment/time-dependent
  skips; four architecture/inventory failures were identified and repaired.
  Their packages passed in the final changed-package run.
- Focused calendar and reminder workflow suites pass after the review fix.
- Architecture check passes with the counts above.
- `scripts/test-changed.sh origin/development` without `--fast`: all 104
  affected packages pass, including `internal/architecture`, `test`, production
  router and worker coverage, and calendar/timetable end-to-end tests.
- `git diff --check`: clean.

Review: Standards found zero actionable issues. Spec found one push-only
dependency defect, reproduced by the regression test and fixed; follow-up
review found no remaining actionable findings.
