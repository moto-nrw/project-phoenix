---
paths:
  - "backend/modules/settings/**"
  - "frontend/src/**/settings/**"
  - "frontend/src/**/*settings*"
---

# Tenant-Scoped Settings System

**RULE: When adding, editing, or deleting settings, follow the patterns below.** The settings system is registry-driven — all definitions are declared at init time, validated at startup, and served to the frontend as an auto-generated schema.

## Architecture

```
Registry (init-time definitions)
  ↓
Schema Builder → Frontend settings page (auto-generated)
  ↓
SettingsService → Resolve/Set/Reset per tenant
  ↓
SettingValueRepository → config.setting_values (RLS-enforced)
  ↓
SettingAuditRepository → config.setting_audit (append-only)
```

### Key Files

| File | Role |
|------|------|
| `modules/settings/records/keys.go` | Key constants (`KeySessionEndTime`, etc.) |
| `modules/settings/records/registry.go` | `Definition` struct, `Register()`, field types, `AccessPolicy` |
| `modules/settings/internal/defaults/*.go` | Registry definitions grouped by category |
| `modules/settings/records/settings_interface.go` | The `SettingsService` interface (authoritative method list) |
| `modules/settings/internal/application/settings_service.go` | Resolve, SetValue, ResetValue, validation |
| `modules/settings/internal/application/schema_builder.go` | Builds schema response for frontend (tabs are dynamic — built from whatever definitions declare) |
| `modules/settings/internal/adapters/configpostgres/` | DB access (setting_values + setting_audit) |
| `modules/settings/http/settings_api.go` | HTTP handlers (GET /schema, PUT/DELETE /values/{key}, GET /values/{key}/reveal for passwords) |
| `frontend/src/lib/settings-api.ts` | Frontend API client + TypeScript types |
| `frontend/src/components/settings/` | Settings page, field components, auto-save (plus hand-written exceptions: `personalization-tab.tsx`, `enrollment-link-panel.tsx`) |
| `frontend/src/app/api/settings/` | Next.js proxy routes to backend (key regex `/^[a-z0-9_.]{1,255}$/`) |

### Registered Settings

**The authoritative list is the code**: `config.Register()` calls in `backend/modules/settings/internal/defaults/*.go`, guarded by `defaults_test.go`. Do not use a hand-maintained setting count:

| File | Group | Tab(s) |
|------|-------|--------|
| `operations.go` | attendance, checkout, presence mode, session lifecycle | operations, system |
| `enrollment.go` | public enrollment (phases, care offerings, captcha, legal texts, outbox) | enrollment, system |
| `gdpr.go` | retention windows, attendance log, cleanup jobs | gdpr, system |
| `timetable.go` | timetable feature toggles + retention | operations, gdpr |
| `security.go` | device PIN, MFA (`security.mfa_*`), account lockout | security, devices |
| `devices.go` | NFC/device behavior, device online window | devices |
| `tracking.go`, `feedback.go`, `invitations.go` | indicators, feedback, guardian invite expiry | operations, gdpr, system |

Tabs in use today: `operations`, `enrollment`, `system`, `gdpr`, `devices`, `security` (dynamic — a new `Tab:` value creates a new tab).

### Value Resolution

The settings service resolves values in **three tiers** (ADR 0086, #4652);
the later tier wins:

```
1. Registry default    (Definition.Default)
2. Träger value        (config.traeger_setting_values row of the school's Träger; only Levels traeger/beide)
3. School override     (config.setting_values row; not for Levels traeger)
```

This is the complete resolution chain for services and their consumers.
Resolution is live on every read, so a new school of a Träger inherits its
values without a copy. `Definition.Levels` is `schule` (default), `beide` or
`traeger`; the registry and `ValidateTraegerLevels` admit a Träger level only
for shared or admin_only, non-password keys without side effect, lock or
cross-field check. The Träger-Büro writes Träger values in the Träger
transaction (`tenant.WithinTraeger`, ADR 0105/0107) through `/traeger/settings`,
checked against `Definition.TraegerWriteRight`; every change lands in
`config.traeger_setting_audit` in the same transaction. Schema items carry
`levels`, `source` (`default`/`traeger`/`school`) and `traeger_changed_at`;
`is_default` means the school has no override of its own. The Träger level is
read only in the school's own tenant transaction or through the batch read
(`ResolveManyForTenants`); any other ambient unit fails instead of skipping it.
Environment variables are not a third tier, including for legacy compatibility.
An explicit tenant value remains authoritative even when it equals the registry
default or is empty, false, or zero where the definition permits that value.
Resolution errors must be handled as errors, not converted into defaults.

### Request-Scoped Resolution Cache (#2065)

Every `Resolve*`/`HasTenantOverride` call funnels through `ResolveMany`, which
consults (in order): an explicit context snapshot (`WithSettingsSnapshot`,
used by scheduler/prefetch paths), then a **request-scoped memo cache**
(`WithSettingsRequestCache`, attached by `RequestSettingsCacheMiddleware`
router-wide in `api/base.go` and group-wide in `ProtectedTenantGroup`), then
the repository. Within one HTTP request each (tenant_id, key) pair is loaded
from PostgreSQL at most once; a full cache hit in `Resolve*ForTenant` skips
the tenant transaction entirely.

Consistency model:
- Cache lifetime is one request; other transactions' committed writes become
  visible at the latest with the next request.
- `SetValue`/`ResetValue` evict the written key, so same-request reads
  (including side-effect hooks in the same transaction) see the new value.
- The advisory-lock helpers (`LockSlotListCutoffPair[Shared]`,
  `LockClassCollectionPair`) flush the whole tenant bucket. **Any new
  cross-field guard MUST take one of the `Lock*` helpers** — the lock is the
  freshness barrier that makes post-lock re-reads observe a concurrent
  writer's committed state (#1565/#1663).
- Explicit snapshots are never copied into the request cache (a scheduler
  minute-snapshot value must not be promoted into request scope).

Handlers that resolve several known keys should still batch explicitly:
`api/common.PrefetchSettings(ctx, settingsService, keys...)` attaches a
snapshot so downstream single-key reads (including in services) are free.
Read paths only — prefetched snapshots are immutable and not evicted by
writes. There is deliberately NO process-wide cache.

`Resolve*()` returns the registry default when no tenant override exists.
Consumers use that result directly; they must not append an environment lookup.
`HasTenantOverride()` reports whether an override exists, not whether an env
fallback should run. See [Step 3](#step-3-add-consuming-code).

### SettingsService Interface

Authoritative list: `modules/settings/records/settings_interface.go`. Highlights:

```go
GetSchema(ctx, permissions)            // Tenant schema — EXCLUDES operator-only settings
GetSchemaForOperator(ctx, permissions) // Operator schema (/api/operator/schools/{id}/settings)
Resolve / ResolveString / ResolveBool / ResolveInt(ctx, key)
ResolveStringForTenant / ResolveBoolForTenant / ResolveIntForTenant(ctx, tenantID, key)
HasTenantOverride(ctx, key)
SetValue / ResetValue(ctx, key, [value,] changedBy *int64, userPermissions)  // nil permissions = system caller, skips permission check
GetLoginImageURL / SetLoginImageURL / ClearLoginImageURL(ctx, tenantID, ...) // school login image (JSONB on school)
```

`ResolveInt` rejects fractional values and checks overflow. `Resolve*ForTenant` wraps its own `tenant.WithTenantTx` — use it outside tenant middleware (device auth, scheduler per-tenant loops, MFA verify during login).

### Access Control — Two Dimensions

1. **Permissions** (`ReadPermission` / `WritePermission`): `config:update` for operational settings, `config:manage` for GDPR/security. Route-level auth accepts either; service-level `checkWritePermission` is wildcard-aware (`admin:*` works).
2. **AccessPolicy** (`shared` | `admin_only` | `operator_only` | `operator_managed`, default `shared`): who may see/change the setting at all. `operator_only` settings (e.g. `operations.presence_mode`, the session-lifecycle settings, `gdpr.data_cleanup_enabled`, `gdpr.data_cleanup_timeout_minutes`) are hidden from the tenant schema and managed via the operator portal; `admin_only` (e.g. `security.ogs_device_pin`) is hidden from operators. `operator_managed` (ADR 0048) is for the retention windows: operators write them, the tenant schema shows them with `writable: false`, and tenant set/reset answers 403. Every retention window of school data uses it, and so does every module switch `module.<key>` ([module switches](../../docs/reference/modulschalter.md)). Settings without an explicit policy are `shared` (e.g. `gdpr.data_cleanup_time` stays tenant-writable).

### Frontend

The settings page is auto-generated from the backend schema: field components render by `type`, auto-save (debounced for text/number/time, immediate for boolean/select, explicit save for password), conditional visibility via `depends_on`, passwords masked. Exceptions that are hand-written: the personalization tab (login image upload) and the enrollment link panel. For embedding settings tabs in other layouts use `useSettingsTabs()` (exported from `settings-page.tsx`; returns `null` without settings access, else `{ tabs, renderTab }`).

### When to Use Settings vs Environment Variables

| Use a **setting** when... | Use an **env var** when... |
|---------------------------|---------------------------|
| Value differs per school/tenant | Value is the same across all tenants |
| Admins should be able to change it at runtime | Only infrastructure operators should change it |
| It's a business rule (times, thresholds, toggles) | It's infrastructure config (DB DSN, JWT secret, SMTP host) |

**RULE: New per-tenant runtime configuration MUST use the settings system, not environment variables.**

## Adding a New Setting

### Step 1: Add Key Constant

```go
// modules/settings/records/keys.go
const KeyMyNewSetting = "category.my_new_setting"  // Key + PascalCase; value lowercase dot-separated
```

### Step 2: Register Definition

Create or edit `modules/settings/internal/defaults/{category}.go`:

```go
config.Register(config.Definition{
    Key:             config.KeyMyNewSetting,
    Label:           "German UI Label",
    Description:     "German description of what this setting does",
    Type:            config.FieldNumber,       // see field types below
    Default:         30,                        // Must match the Type
    ReadPermission:  "config:read",
    WritePermission: "config:update",           // "config:manage" for GDPR/security settings
    Tab:             "operations",
    Category:        "sessions",                // Groups settings visually
    SortOrder:       4,
    AccessPolicy:    config.AccessShared,       // or AccessAdminOnly / AccessOperatorOnly / AccessOperatorManaged
    Validation:      &config.ValidationRules{Min: &minVal, Max: &maxVal},  // Optional
    DependsOn:       &config.Dependency{Key: config.KeyParent, Condition: "eq", Value: true},  // Optional
})
```

### Step 3: Add Consuming Code

Use `Resolve*(ctx, key)` inside tenant middleware, or
`Resolve*ForTenant(ctx, tenantID, key)` outside it (device auth, scheduler,
login). Handle the error and use the resolved value directly.

**Target rule:** privacy, permission and token-lifetime reads propagate lookup
errors; HTTP readers return the shared error envelope without internal details
or replacement data, and scheduler jobs skip the affected tenant. Require
settings ports at construction, not on the first request. The history, export,
consent, tenant-shell, enrollment edit/token, MFA lifetime/lockout and retention
paths migrated in #4543 follow this contract. Parent invitation lifetimes never
use the deployment lifetime configured for staff invitations.

**Trusted-device policy (#4641):** fail closed for remembering the device,
not for an otherwise verified login. In both tenant and school portals, errors
reading `security.mfa_trusted_device_enabled` or
`security.mfa_trusted_device_days` suppress device issuance and its cookie;
the verified login still creates its normal session. Preparation propagates
either read error, and issuance requires a readable, positive lifetime.
Availability/display hints stay restrictive on read errors. Lockout-policy
errors remain fatal before code comparison: no consumed challenge, session,
tokens or cookie. These entries supersede the trusted-device rows in
[PR #4631's inventory](https://github.com/moto-nrw/project-phoenix/pull/4631#literal-fallback-inventar);
other inventoried debt remains unchanged.

**Known exceptions, not patterns for new code:**

- Existing fail-closed feature/display readers (for example staff-name
  visibility and offering-change availability) still
  suppress errors. Some older constructors still permit nil settings and the
  affected consumers answer restrictive values. These are migration debt,
  not proof that every required port is already checked at startup.
- Existing operational readers still use literals on errors or nil resolvers.
  The complete per-reader classification, including nil-only cases, lives in
  [PR #4631, Literal-Fallback-Inventar](https://github.com/moto-nrw/project-phoenix/pull/4631#literal-fallback-inventar).
  The #3841 device-PIN chain is now removed; other inventoried debt remains.
  The inventory is not permission to add a fallback.

`Resolve*OrDefault` resolves the registry default without an override; its
caller literal applies only to a failed read. New consumers use `Resolve*`
and handle the error; touching an inventoried consumer should remove its fallback.

Require the settings service at composition time. A missing service is a
configuration error, not a reason to read env vars or invent a local default.
Do not compare a result with `Definition.Default` to infer whether the tenant
set it; an explicit override may intentionally equal that default.

Existing consumer env-fallback chains are migration debt, not an exception to
this policy. When removing one, preserve any intended per-school configuration
as explicit settings before cutover and test the resulting behavior. This rule
does not claim that every existing runtime consumer has already been migrated.

### Step 4: Update Tests

Update affected registry tests in `modules/settings/internal/defaults/defaults_test.go`
(registration, types, dependencies, validation, defaults). Consumer tests must
cover explicit overrides, absent overrides using the registry default, and
resolution failures. Include an override equal to the default and valid
empty/false/zero values where applicable; conflicting env values must not
change the result. Follow the backend test-fixture rules.

### Step 5: Verify

```bash
cd backend && ../scripts/run-go-toolchain.sh go test -tags=testhooks ./modules/settings/... -v
```

Also run tests for affected consumers and the backend architecture checks.

## Editing a Setting

| Change | What to update |
|--------|---------------|
| Default / validation / permissions / label | Registry definition in `defaults/{category}.go` (frontend auto-updates) |
| Renaming a key | `keys.go` + all consumers + data migration for existing `config.setting_values` rows |

## Deleting a Setting

- [ ] Remove from `modules/settings/records/keys.go` and the `config.Register()` call
- [ ] Remove all consuming code references
- [ ] Update `defaults_test.go`
- [ ] Data migration: `DELETE FROM config.setting_values WHERE setting_key = 'old.key'` (same for `config.setting_audit`)

## Consuming Code Patterns

- **Tenant requests:** use typed `Resolve*` methods with the tenant context.
- **Scheduler, device auth, and login:** use typed `Resolve*ForTenant` methods
  with an explicit tenant ID; preserve isolation across per-tenant loops.
- **Unset business values:** model them in the registry definition and consume
  them according to that setting's contract, not through an env lookup.

## Key Rules

- **Per-tenant runtime config uses school overrides, Träger values and registry defaults only** — no env fallback, including legacy compatibility chains
- **Handle settings-service errors explicitly** — do not hide them behind env values or local defaults
- **NEVER hardcode key strings** — use constants from `modules/settings/records/keys.go`
- **`config:manage`** for sensitive settings (GDPR, security); **`config:update`** for operational ones
- **Password fields are redacted** to `[REDACTED]` in the audit trail; reveal only via the dedicated endpoint
- **Labels and descriptions must be in German** — the settings UI is German-only

## Available Field Types

9 types (`modules/settings/records/registry.go`): `boolean`, `number`, `time` (HH:MM), `date`, `text`, `textarea`, `password`, `select` (requires `Options.Static`), `multiselect` (requires `Options.Static`; the value is the list of chosen option values, stored in option order). Boolean/select/multiselect save immediately; text/number/time debounce; password has an explicit save.

## Conditional Visibility (DependsOn)

```go
DependsOn: &config.Dependency{Key: config.KeyParentSetting, Condition: "eq", Value: true}  // eq, neq, not_empty
```

The child setting is hidden in the UI when the condition is not met. **DependsOn is UI-only** — the backend still accepts writes regardless of visibility.

## Error Types

| Error | HTTP Status | When |
|-------|-------------|------|
| `DefinitionNotFoundError` | 404 | Key not in registry |
| `InvalidValueError` | 400 | Value fails type/validation checks |
| `PermissionDeniedError` | 403 | User lacks WritePermission for the setting |

## Database Tables (Migrations 1.15.25 and 1.15.552)

- **`config.setting_values`** — tenant overrides; UNIQUE(tenant_id, setting_key); RLS-isolated; `value` is jsonb
- **`config.setting_audit`** — append-only change log (`action`: set/reset/delete, old/new jsonb, passwords redacted)
- **`config.traeger_setting_values`** — Träger values; UNIQUE(organization_id, setting_key); RLS on `app.current_org_id` for the Träger, read-only for the Träger's own schools
- **`config.traeger_setting_audit`** — append-only Träger change log (`action`: set/reset), kept three years like the school log
