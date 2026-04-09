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
| `frontend/src/lib/settings-api.ts` | Frontend API client |
| `frontend/src/components/settings/` | Settings page, field components, auto-save |

### Value Resolution

The settings service resolves values in **two tiers**:

```
1. Tenant DB override  (config.setting_values row exists)
2. Registry default    (Definition.Default)
```

The service does **not** check environment variables. `Resolve*()` returns the registry default when no tenant override exists. Consumers that need env var backward compatibility must implement a three-step pattern manually: `HasTenantOverride()` → `Resolve*()` → `os.Getenv()`. See [Step 3](#step-3-add-consuming-code) for the correct pattern.

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
