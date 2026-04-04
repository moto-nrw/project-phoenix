# Review: `feat/settings` RFC

**Author:** yungweng (via Codex)
**Date:** 2026-03-27T15:00:00+02:00

Reviewed against `origin/feat/settings` (`docs/settings-system-rfc.md`) and the current Phoenix codebase.

## Summary

The RFC has one very strong core idea: a backend registry that defines settings metadata and a frontend that renders from schema. That part is worth keeping.

The rest is too ambitious for the problem we actually have right now. The immediate need is much smaller: move the tenant-specific `.env` values into tenant settings, make the scheduler consume them correctly, and stop hardcoding special OGS behavior like the shared device PIN. The RFC designs a full settings framework before solving that narrower problem.

## What is good

- **Schema-driven registry is the right backbone.**
  A Go `Register()`-style registry plus a schema endpoint gives good DX and avoids hand-building every future settings form.

- **Keeping backend as source of truth is correct.**
  Types, defaults, permissions, and grouping should live on the backend, not be duplicated in the frontend.

- **Audit support is reasonable.**
  For GDPR-sensitive configuration, an append-only audit table is a good idea, even if the UI for it can wait.

- **Compatibility wrapper is pragmatic.**
  Keeping the existing config service alive while migrating readers is much safer than a big-bang cutover.

- **Tenant-level settings are clearly needed.**
  This part is not speculative. We already have tenant-specific behavior trapped in `.env`.

## What is bad

- **The scope model is solving problems we do not currently have.**
  The RFC starts with seven scopes: system, organization, tenant, user, device, group, room. The current migration target is mostly tenant-scoped settings, with maybe a small amount of system defaulting later. Group/user/room/device inheritance is not required to solve the real pain points you identified.

- **The most expensive part of the real work is underplayed.**
  The scheduler currently reads config from environment variables at boot and stores some of it in memory. That means per-tenant settings are not just a CRUD/settings-page task. They require scheduler redesign. The RFC does not center that enough.

- **Action buttons are mixed into the settings abstraction too early.**
  “Create Schulhof”, “Destroy WC”, and similar operations are admin actions, not settings. They are useful, but coupling them to a generic settings framework makes every layer more complex for little immediate gain.

- **Dynamic selects, dependency graphs, custom renderers, bulk operations, and search are premature.**
  The settings we actually need from `.env` are mostly booleans, times, text, and integers. The RFC spends a lot of complexity budget on generalized UI machinery that does not help ship the first useful version.

- **The open questions exist because the design is too broad.**
  Multi-group conflict resolution is a good example. That is not a real blocker for moving OGS PINs and scheduler values out of `.env`; it only appears because the proposal tries to support group-scoped resolution now.

- **There is a mismatch between “settings” and “manual infrastructure.”**
  Schulhof/WC are currently created from hardcoded constants and lazy auto-create paths. That is important, but it is not the same problem as “editable tenant settings.” The RFC does not separate these concerns clearly enough.

## Current code realities the RFC should anchor on

- `OGS_DEVICE_PIN` is global today in [backend/auth/device/device_auth.go](/Users/yonnock/Developer/moto/project-phoenix/backend/auth/device/device_auth.go:224). That is the highest-priority setting to fix because all schools currently share one PIN.

- `STUDENT_DAILY_CHECKOUT_TIME` is read directly from env in [backend/api/iot/checkin/helpers.go](/Users/yonnock/Developer/moto/project-phoenix/backend/api/iot/checkin/helpers.go:17).

- Cleanup and session-end behavior are env-driven in [backend/services/scheduler/scheduler.go](/Users/yonnock/Developer/moto/project-phoenix/backend/services/scheduler/scheduler.go:192) and [backend/services/scheduler/scheduler.go](/Users/yonnock/Developer/moto/project-phoenix/backend/services/scheduler/scheduler.go:548), with configuration parsed once at startup in the cleanup path.

- Schulhof and WC infrastructure are currently created from hardcoded constants in [backend/constants/activities.go](/Users/yonnock/Developer/moto/project-phoenix/backend/constants/activities.go:7), [backend/api/iot/checkin/schulhof.go](/Users/yonnock/Developer/moto/project-phoenix/backend/api/iot/checkin/schulhof.go:14), and [backend/api/iot/checkin/wc.go](/Users/yonnock/Developer/moto/project-phoenix/backend/api/iot/checkin/wc.go:14).

## Simplified v1 recommendation

Keep the RFC’s backbone. Cut the framework features that do not help with the first delivery.

### 1. Start with two levels only

- `system` default in the registry
- `tenant` override in the database

That gives you the only fallback you actually need right now:

1. look for tenant override
2. otherwise use registry default

No organization, user, group, room, or device resolution in v1.

### 2. Restrict field types to the real set

For the `.env` migration work, v1 only needs:

- `boolean`
- `time`
- `number`
- `text`

Skip `select`, `date`, `color`, `password`, `textarea`, `json`, dynamic options, and custom renderers until a real setting requires them.

### 3. Separate settings from admin actions

Do not put “Create Schulhof” or “Destroy WC” into the generic settings system in v1.

Instead:

- ship tenant settings as settings
- ship system-room lifecycle as explicit facility/admin operations

Those actions can still use the same service layer later, but they should not block or complicate the first settings rollout.

### 4. Keep the backend API tiny

V1 only needs:

- `GET /api/settings/schema`
- `PUT /api/settings/values/{key}`
- `DELETE /api/settings/values/{key}`

Maybe include current resolved tenant values in the schema response so the frontend only does one fetch.

Skip bulk endpoints, action endpoints, resolve-one-off endpoints, and audit endpoints in v1.

### 5. Keep the frontend tiny too

V1 does not need a full renderer ecosystem. It only needs:

- a settings page/container
- a category group component
- a single setting row component
- 3-4 field components

That preserves the schema-driven idea without building a mini form framework.

### 6. Move the scheduler refactor into phase 1, not phase 6

This is the real hard part.

The scheduler currently assumes global process-level config. For tenant settings to matter, scheduled jobs need to evaluate tenant-specific values at runtime. That means the first implementation plan should explicitly include:

- loading all active tenants for relevant jobs
- resolving each tenant’s settings inside the execution path
- handling per-tenant run windows and timeouts
- deciding whether per-tenant jobs run in sequence or with bounded concurrency
- defining failure isolation so one tenant’s bad config does not break all others

If this is not designed first, the settings UI becomes a lie.

## Concrete v1 scope

The first version should only solve this:

- tenant-specific OGS device PIN
- tenant-specific student daily checkout time
- tenant-specific session-end enablement, time, and timeout
- tenant-specific session cleanup enablement, interval, and abandoned threshold
- tenant-specific cleanup enablement, cleanup time, and timeout

That is enough to eliminate the real `.env`-driven tenant behavior.

## Suggested trimmed implementation plan

### Phase 1: Backend settings core

- Add a small registry definition model
- Add `config.setting_values`
- Add optional `config.setting_audit` write support, but no audit UI yet
- Add tenant-only resolver with registry-default fallback
- Add the minimal settings endpoints
- Register only the `.env`-migration settings

### Phase 2: Scheduler and IoT migration

- Replace direct `os.Getenv()` reads for tenant-specific behavior
- Make scheduler jobs tenant-aware at runtime
- Make device auth use tenant-specific OGS PIN resolution
- Make checkout helper use tenant-specific checkout time resolution

### Phase 3: Frontend page

- Render tenant settings from schema
- Support edit + reset
- Group by tab/category
- No dependency engine, no dynamic select system, no action framework

### Phase 4: Manual infrastructure follow-up

- Add explicit admin/facilities operations for Schulhof and WC create/destroy
- Consider making names/capacities configurable only after the lifecycle is explicit and safe

## What to keep as “later if needed”

- extra scopes beyond tenant
- scope cascade and inherited-from UI
- dependency-based visibility
- action buttons inside settings
- dynamic select options
- custom field renderers
- bulk operations
- import/export
- search
- audit history UI

## Bottom line

Chris’s RFC is a good north star, but a bad v1.

If you keep the registry plus schema-driven rendering, and aggressively cut everything not needed for the current `.env` migration, you can get to a useful settings system much faster without throwing away the architecture. The correct first milestone is not “universal settings platform.” It is “tenant-specific operational settings actually work in production.”
