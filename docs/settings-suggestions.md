# Settings Migration: .env → Per-Tenant Settings

**Author:** yungweng
**Date:** 2026-03-27T15:00:00+02:00

Settings that should move from static environment variables to the new per-tenant settings system.

---

## Security / Auth

| Setting | Env Var | Default | Scope | Notes |
|---------|---------|---------|-------|-------|
| OGS Device PIN | `OGS_DEVICE_PIN` | `1234` | tenant | **Critical.** Currently all schools share one global PIN. Read via `os.Getenv()` in `auth/device/device_auth.go:224`. Each school needs its own PIN. |

## Daily Operations

| Setting | Env Var | Default | Scope | Notes |
|---------|---------|---------|-------|-------|
| Session End Time | `SESSION_END_TIME` | `18:00` | tenant | Time to auto-end all active sessions. Read in `services/scheduler/scheduler.go:548`. Schools close at different times. |
| Student Daily Checkout Time | `STUDENT_DAILY_CHECKOUT_TIME` | `15:00` | tenant | Time after which students can check out from home room. Read via `os.Getenv()` in `api/iot/checkin/helpers.go:17`. |
| Session End Enabled | `SESSION_END_SCHEDULER_ENABLED` | `true` | tenant | Toggle automatic session termination. Some schools may want manual control. |
| Session End Timeout | `SESSION_END_TIMEOUT_MINUTES` | `10` | tenant | Max duration for session end operation. Read in `services/scheduler/scheduler.go:652`. |

## Abandoned Session Cleanup

| Setting | Env Var | Default | Scope | Notes |
|---------|---------|---------|-------|-------|
| Session Cleanup Enabled | `SESSION_CLEANUP_ENABLED` | `true` | tenant | Toggle abandoned session cleanup. |
| Cleanup Interval | `SESSION_CLEANUP_INTERVAL_MINUTES` | `15` | tenant | How often to check for abandoned sessions. Read in `services/scheduler/scheduler.go:712`. |
| Abandoned Threshold | `SESSION_ABANDONED_THRESHOLD_MINUTES` | `60` | tenant | Minutes of inactivity before a session is considered abandoned. Read in `services/scheduler/scheduler.go:719`. |

## GDPR / Data Cleanup

| Setting | Env Var | Default | Scope | Notes |
|---------|---------|---------|-------|-------|
| Data Cleanup Enabled | `CLEANUP_SCHEDULER_ENABLED` | `true` | tenant | Toggle automated cleanup of expired visit data. Read in `services/scheduler/scheduler.go:192`. |
| Cleanup Time | `CLEANUP_SCHEDULER_TIME` | `02:00` | tenant | When to run daily data cleanup. Read in `services/scheduler/scheduler.go:198`. |
| Cleanup Timeout | `CLEANUP_SCHEDULER_TIMEOUT_MINUTES` | `30` | tenant | Max duration for cleanup operation. Read in `services/scheduler/scheduler.go:302`. |

---

## Architectural Impact: Scheduler Refactor

The scheduler currently reads all these values once at startup via `os.Getenv()`. Moving to per-tenant settings means:

- The scheduler must iterate over all active tenants for each scheduled job
- Each tenant's config is queried at runtime, not boot time
- Jobs like session-end and cleanup run per-tenant with that tenant's specific times and thresholds
- The scheduler needs access to the settings service (new dependency)

This is the biggest refactor required to support per-tenant settings.
