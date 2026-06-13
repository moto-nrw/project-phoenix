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
| `models/config/keys.go` | Key constants (`KeySessionEndTime`, etc.) |
| `models/config/registry.go` | `Definition` struct, `Register()`, field types |
| `services/config/defaults/*.go` | Registry definitions grouped by category |
| `services/config/settings_service.go` | Resolve, SetValue, ResetValue, validation |
| `services/config/schema_builder.go` | Builds schema response for frontend |
| `database/repositories/config/` | DB access (setting_values + setting_audit) |
| `api/config/settings_api.go` | HTTP handlers (GET /schema, PUT/DELETE /values/{key}) |
| `frontend/src/lib/settings-api.ts` | Frontend API client + TypeScript types |
| `frontend/src/components/settings/` | Settings page, field components, auto-save |
| `frontend/src/app/api/settings/` | Next.js proxy routes to backend |

### Registered Settings

The authoritative list lives in `backend/services/config/defaults/*.go` and
is guarded by `backend/services/config/defaults/defaults_test.go`. The tables
below document the main tenant-facing groups; update them when adding visible
settings.

**Operations Tab** — WritePermission: `config:update`

| Key | Type | Default | DependsOn |
|-----|------|---------|-----------|
| `operations.student_daily_checkout_time` | time | `""` | — |
| `operations.per_student_checkout_enabled` | boolean | `false` | — |
| `operations.per_student_checkout_delta_minutes` | number | `15` (min:0, max:120) | per_student_checkout_enabled eq true |
| `operations.admin_supervision_overview` | boolean | `false` | — |
| `operations.time_tracking_account_start_date` | date | `""` | none |
| `operations.status_flag_clear_time` | time | `"18:00"` | — |
| `operations.sick_clear_mode` | select | `"next_checkin"` | — |
| `operations.excused_clear_mode` | select | `"end_of_day"` | — |
| `operations.presence_mode` | select | `"detailed"` | — |
| `operations.student_photos_enabled` | boolean | `false` | — |
| `attendance.web_checkin_access` | select | `"group_supervisors"` | — |
| `attendance.web_spontaneous_activities_enabled` | boolean | `false` | — |
| `tracking.indicators_enabled` | boolean | `false` | — |
| `tracking.indicator_1` | text | `""` | tracking.indicators_enabled eq true |
| `tracking.indicator_2` | text | `""` | tracking.indicators_enabled eq true |
| `tracking.indicator_3` | text | `""` | tracking.indicators_enabled eq true |

**GDPR Tab** — WritePermission: `config:manage`

| Key | Type | Default | DependsOn |
|-----|------|---------|-----------|
| `gdpr.attendance_log_enabled` | boolean | `false` | — |
| `gdpr.attendance_visible_days` | number | `30` (min:1, max:365) | attendance_log_enabled eq true |
| `gdpr.room_detail_visible_days` | number | `7` (min:1, max:365) | attendance_log_enabled eq true |
| `gdpr.attendance_log_scope` | select | `"group_supervisors_only"` | attendance_log_enabled eq true |
| `gdpr.student_data_scope` | select | `"group_supervisors_only"` | — |
| `feedback.enabled` | boolean | `false` | — |
| `feedback.data_retention_days` | number | `90` (min:7, max:365) | feedback.enabled eq true |
| `gdpr.timetable_retention_days` | number | `365` | timetable.enabled eq true |

**System Tab** — WritePermission: `config:update` or `config:manage`

| Key | Type | Default | DependsOn |
|-----|------|---------|-----------|
| `operations.session_end_enabled` | boolean | `true` | — |
| `operations.session_end_time` | time | `"18:00"` | session_end_enabled eq true |
| `operations.session_end_timeout_minutes` | number | `10` (min:1, max:60) | session_end_enabled eq true |
| `operations.session_cleanup_enabled` | boolean | `false` | — |
| `operations.session_cleanup_interval_minutes` | number | `15` (min:5, max:120) | session_cleanup_enabled eq true |
| `operations.session_abandoned_threshold_minutes` | number | `60` (min:10, max:480) | session_cleanup_enabled eq true |
| `gdpr.data_cleanup_enabled` | boolean | `true` | — |
| `gdpr.data_cleanup_time` | time | `"02:00"` | data_cleanup_enabled eq true |
| `gdpr.data_cleanup_timeout_minutes` | number | `30` (min:5, max:120) | data_cleanup_enabled eq true |

**Enrollment Tab**: WritePermission `config:manage` for legal text settings

| Key | Type | Default | DependsOn |
|-----|------|---------|-----------|
| `enrollment.legal_terms_enabled` | boolean | `false` | enrollment.enabled eq true |
| `enrollment.legal_agb_text` | textarea | `""` | enrollment.enabled eq true |
| `enrollment.legal_dsgvo_enabled` | boolean | `false` | enrollment.enabled eq true |
| `enrollment.legal_dsgvo_text` | textarea | `""` | enrollment.enabled eq true |
| `enrollment.legal_photo_enabled` | boolean | `false` | enrollment.enabled eq true |
| `enrollment.legal_photo_text` | textarea | `""` | enrollment.enabled eq true |
| `enrollment.legal_email_contact_enabled` | boolean | `false` | enrollment.enabled eq true |
| `enrollment.legal_email_contact_text` | textarea | `""` | enrollment.enabled eq true |

**Security / Devices Tabs** — WritePermission: `config:manage` or `config:update`

| Key | Type | Default | Validation |
|-----|------|---------|------------|
| `security.ogs_device_pin` | password | `"1234"` | Pattern: `^\d{4}$` (4-digit PIN) |
| `checkout.raumwechsel_enabled` | boolean | `true` | — |
| `checkout.schulhof_enabled` | boolean | `false` | — |
| `checkout.wc_enabled` | boolean | `false` | — |

### Value Resolution

The settings service resolves values in **two tiers**:

```
1. Tenant DB override  (config.setting_values row exists)
2. Registry default    (Definition.Default)
```

The service does **not** check environment variables. `Resolve*()` returns the registry default when no tenant override exists. Consumers that need env var backward compatibility must implement a three-step pattern manually: `HasTenantOverride()` → `Resolve*()` → `os.Getenv()`. See [Step 3](#step-3-add-consuming-code) for the correct pattern.

### SettingsService Interface

```go
Resolve(ctx, key) → any                              // Tenant override or registry default
ResolveString(ctx, key) → string                      // Type-safe string
ResolveBool(ctx, key) → bool                          // Type-safe bool
ResolveInt(ctx, key) → int                            // Type-safe int (rejects fractions, checks overflow)
ResolveStringForTenant(ctx, tenantID, key) → string   // Explicit tenant + wraps in own TenantTx
HasTenantOverride(ctx, key) → bool                    // Check if DB override exists
SetValue(ctx, key, value, changedBy, permissions)     // Upsert + audit + permission check
ResetValue(ctx, key, changedBy, permissions)           // Delete override + audit
GetSchema(ctx, permissions) → SettingsSchema           // Filtered by read permissions
```

### Tabs and Permissions

| Tab | WritePermission | Who can edit |
|-----|-----------------|-------------|
| `operations` | `config:update` | Admin |
| `gdpr` | `config:manage` | Admin |
| `security` | `config:manage` | Admin |
| `general` | (varies) | Admin |

Route-level auth accepts `config:update` OR `config:manage`. Service-level `checkWritePermission` uses wildcard-aware matching (`admin:*` works).

### Frontend

The settings page is fully auto-generated from the backend schema. Field components render based on `type`, with auto-save on blur (debounced for text/number/time), immediate save for boolean/select, green/red border feedback, conditional visibility via `depends_on`, and password fields masked as `••••••`.

### When to Use Settings vs Environment Variables

| Use a **setting** when... | Use an **env var** when... |
|---------------------------|---------------------------|
| Value differs per school/tenant | Value is the same across all tenants |
| Admins should be able to change it at runtime | Only infrastructure operators should change it |
| It's a business rule (times, thresholds, toggles) | It's infrastructure config (DB DSN, JWT secret, SMTP host) |
| It affects end-user behavior | It affects how the server connects to external services |

**RULE: New per-tenant runtime configuration MUST use the settings system, not environment variables.** Env vars are for infrastructure. If a school admin should be able to configure it, it's a setting. Existing env vars are kept for backward compatibility only.

## Adding a New Setting

### Step 1: Add Key Constant

```go
// models/config/keys.go
const (
    KeyMyNewSetting = "category.my_new_setting"
)
```

**Naming**: `Key` + PascalCase name. Value: `"category.setting_name"` (lowercase, dot-separated).

### Step 2: Register Definition

Create or edit `services/config/defaults/{category}.go`:

```go
config.Register(config.Definition{
    Key:             config.KeyMyNewSetting,
    Label:           "German UI Label",
    Description:     "German description of what this setting does",
    Type:            config.FieldNumber,       // boolean, number, time, text, password, select
    Default:         30,                        // Must match the Type
    ReadPermission:  "config:read",
    WritePermission: "config:update",           // Use "config:manage" for GDPR/security settings
    Tab:             "operations",              // operations, gdpr, security, general
    Category:        "sessions",                // Groups settings visually
    SortOrder:       4,                         // Display order within category
    Validation:      &config.ValidationRules{Min: &minVal, Max: &maxVal},  // Optional
    DependsOn:       &config.Dependency{Key: config.KeyParent, Condition: "eq", Value: true},  // Optional
})
```

### Step 3: Add Consuming Code

**MANDATORY PATTERN** — Use `HasTenantOverride` to preserve env var fallback:

```go
value := defaultValue

if settingsService != nil {
    if has, err := settingsService.HasTenantOverride(ctx, configModel.KeyMyNewSetting); err != nil {
        slog.Warn("settings override check failed, falling back",
            slog.String("key", configModel.KeyMyNewSetting),
            slog.String("error", err.Error()),
        )
    } else if has {
        if val, err := settingsService.ResolveString(ctx, configModel.KeyMyNewSetting); err == nil && val != "" {
            value = val
        }
    }
}

// Fall back to env var
if value == defaultValue {
    if envVal := os.Getenv("MY_ENV_VAR"); envVal != "" {
        value = envVal
    }
}
```

**WHY**: `Resolve*()` returns the registry default when no tenant override exists, bypassing env vars. `HasTenantOverride` distinguishes "no override" (fall through to env var) from "override exists" (use DB value).

### Step 4: Update Tests

- Add to `services/config/defaults/defaults_test.go` — update relevant test functions (currently 9 tests covering registration, types, dependencies, validation, and defaults)
- Add consuming code tests with mock that returns the setting value

### Step 5: Verify

```bash
cd backend && go build ./... && go test ./services/config/... -v
```

## Editing a Setting

| Change | What to update |
|--------|---------------|
| Default value | Registry definition in `defaults/{category}.go` |
| Validation rules | `ValidationRules` in the definition (min/max/pattern) |
| Permissions | `ReadPermission` / `WritePermission` in the definition |
| Label/description | Registry definition (frontend auto-updates) |
| Renaming a key | `keys.go` + all consumers + data migration for existing `config.setting_values` rows |

## Deleting a Setting

- [ ] Remove from `models/config/keys.go`
- [ ] Remove `config.Register()` call from `defaults/{category}.go`
- [ ] Remove all consuming code references
- [ ] Remove from `defaults_test.go` (all four test functions)
- [ ] Add data migration to delete orphaned rows: `DELETE FROM config.setting_values WHERE setting_key = 'old.key'`
- [ ] Add data migration to delete audit rows: `DELETE FROM config.setting_audit WHERE setting_key = 'old.key'`

## Consuming Code Patterns

### Scheduler (`services/scheduler/scheduler.go`)

The scheduler has helper methods that wrap the HasTenantOverride pattern for per-tenant iteration:

- `resolveStringSetting(ctx, key, envVarName, fallback)` — HasTenantOverride → ResolveString → os.Getenv → fallback
- `resolveBoolSetting(ctx, key, envVarName, fallback)` — same chain for booleans
- `resolveIntSetting(ctx, key, envVarName, fallback)` — same chain for integers

The scheduler uses `forEachTenantSettings()` to iterate all active schools and execute per-tenant logic with settings context.

### IoT Checkin (`api/iot/checkin/helpers.go`)

`getStudentDailyCheckoutTime()` follows the same pattern:
1. `HasTenantOverride(ctx, KeyStudentDailyCheckoutTime)`
2. If override: `ResolveString` → parse HH:MM
3. If no override: fall back to env var `STUDENT_DAILY_CHECKOUT_TIME`
4. If no env var: hardcoded default `"15:00"`

### Device PIN Authentication (`api/iot/api.go`)

Creates a PIN resolver from the settings service to validate the OGS device PIN (`security.ogs_device_pin`).

## Key Rules

- **NEVER add new env vars for per-tenant runtime config** — use the settings system instead. Env vars are for infrastructure (DB DSN, SMTP host, JWT secret). If a school admin should be able to change it, it's a setting. Existing env vars are kept for backward compatibility only.
- **NEVER call `Resolve*()` alone when env var fallback is needed** — the service returns the registry default, not the env var. Use `HasTenantOverride()` first to distinguish "no override" from "override exists"
- **NEVER hardcode key strings** — use constants from `models/config/keys.go`
- **ALWAYS use `config:manage`** for sensitive settings (GDPR, security)
- **ALWAYS use `config:update`** for operational settings
- **Number settings reject fractional values** — only whole integers are accepted
- **Password fields are automatically redacted** in the audit trail (`[REDACTED]`)
- **Labels and descriptions must be in German** — the settings UI is German-only

## When to Use ResolveString vs ResolveStringForTenant

| Method | When to use |
|--------|-------------|
| `ResolveString(ctx, key)` | Inside tenant middleware (ctx has tenant from JWT) |
| `ResolveStringForTenant(ctx, tenantID, key)` | Outside tenant middleware (e.g., device auth, scheduler per-tenant loops) |

`ResolveStringForTenant` wraps the query in its own `tenant.WithTenantTx` — use it when the caller doesn't already have a tenant transaction in context.

## Available Field Types

| Type | Go default | Frontend component | Notes |
|------|-----------|-------------------|-------|
| `boolean` | `true`/`false` | Toggle switch | Immediate save on change |
| `number` | `int` | Number input with min/max | Must be whole integer |
| `time` | `"HH:MM"` | Time picker | Validated via `time.Parse("15:04")`, resolved via `ResolveString` |
| `text` | `string` | Text input | Debounced auto-save |
| `password` | `""` | Masked input with edit button | Value masked in schema + audit |
| `select` | option value | Dropdown | Requires `Options.Static` |

## Conditional Visibility (DependsOn)

```go
DependsOn: &config.Dependency{
    Key:       config.KeyParentSetting,
    Condition: "eq",      // eq, neq, not_empty
    Value:     true,
}
```

The child setting is hidden in the UI when the condition is not met. The backend still accepts writes regardless of visibility — DependsOn is UI-only.

## Error Types

| Error | HTTP Status | When |
|-------|-------------|------|
| `DefinitionNotFoundError` | 404 | Key not in registry |
| `InvalidValueError` | 400 | Value fails type/validation checks |
| `PermissionDeniedError` | 403 | User lacks WritePermission for the setting |

## Frontend Behavior

### Proxy Routes

- `GET /api/settings/schema` → proxies to backend `/api/settings/schema`
- `PUT /api/settings/values/[key]` → proxies to backend `/api/settings/values/{key}`
- `DELETE /api/settings/values/[key]` → proxies to backend `/api/settings/values/{key}`
- Key validation: `/^[a-z0-9_.]{1,255}$/`

### Auto-Generated Settings Page

The settings page (`frontend/src/app/[tenant]/(protected)/settings/page.tsx`) is fully auto-generated from the backend schema. No frontend changes needed when adding new settings.

- **Auto-save**: Debounced 3s for text/number/time, immediate for boolean/select, explicit save button for password
- **Save feedback**: Green border on success, red on error (4s fade)
- **Reset button**: Visible only when setting has tenant override and user is writable
- **Permissions**: `writable: false` → input disabled, shows "Nur Lesen" badge
- **Conditional visibility**: Client re-evaluates `depends_on` conditions after each save
- **Optimistic updates**: UI updates locally, background re-fetch at 6s
- **Mobile-responsive**: Tab layout adapts below 768px (tabs become cards)
- **Error messages**: Translated to German in `settings-api.ts`

### `useSettingsTabs()` Hook

Exported from `settings-page.tsx` for reusable tab integration. Returns `null` if user has no settings access (graceful degradation). Returns `{ tabs, renderTab(tabId) }` for embedding in layouts.

## Database Tables

### `config.setting_values` (Migration 1.15.25)

Tenant-scoped overrides. UNIQUE(tenant_id, setting_key). RLS policy for tenant isolation.

| Column | Type | Notes |
|--------|------|-------|
| id | bigint PK | |
| tenant_id | bigint FK | → platform.schools |
| setting_key | text | Must match a registered key |
| value | jsonb | Marshaled setting value |
| updated_by | bigint FK | → auth.accounts |
| created_at, updated_at | timestamp | |

### `config.setting_audit` (Migration 1.15.25)

Append-only change log. RLS policy for tenant isolation.

| Column | Type | Notes |
|--------|------|-------|
| id | bigint PK | |
| tenant_id | bigint FK | |
| setting_key | text | |
| old_value, new_value | jsonb | Passwords redacted to `[REDACTED]` |
| action | text | `set`, `reset`, or `delete` |
| changed_by | bigint FK | → auth.accounts |
| changed_at | timestamp | |
