---
name: settings
description: Use when adding, editing, deleting, or debugging tenant-scoped settings. Triggers on mentions of per-school config, settings registry, setting keys, HasTenantOverride, or config.setting_values.
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# Tenant-Scoped Settings

Workflow for any settings system work. The full reference is in `.Codex/rules/settings-system.md` — this skill guides you through the right files to read first.

## Before Any Settings Work

Read these files to understand current state:

```
backend/models/config/keys.go              # All setting key constants
backend/models/config/registry.go          # Definition struct, field types, Register()
backend/services/config/defaults/          # Registered definitions by category (ls first)
backend/services/config/settings_service.go # Resolve, SetValue, ResetValue, HasTenantOverride
```

Then read the full rule: `.Codex/rules/settings-system.md`

## Critical: How Resolution Works

The settings service resolves as **DB override -> registry default only**. It does NOT check env vars.

```
Resolve*(ctx, key)
  1. Tenant DB override exists? -> return it
  2. No override? -> return Definition.Default (NOT env var)
```

Consumers needing env var backward compatibility MUST use this pattern:

```go
value := defaultValue
if settingsService != nil {
    if has, err := settingsService.HasTenantOverride(ctx, key); err != nil {
        slog.Warn("settings check failed", "key", key, "error", err.Error())
    } else if has {
        if val, err := settingsService.ResolveString(ctx, key); err == nil && val != "" {
            value = val
        }
    }
}
if value == defaultValue {
    if envVal := os.Getenv("MY_ENV_VAR"); envVal != "" {
        value = envVal
    }
}
```

## Task: Add a Setting

1. Add key constant to `backend/models/config/keys.go`
2. Register definition in `backend/services/config/defaults/{category}.go`
3. Add consuming code with HasTenantOverride pattern (see above)
4. Update tests in `backend/services/config/defaults/defaults_test.go`
5. Verify: `cd backend && go build ./... && go test ./services/config/... -v`

## Task: Edit a Setting

Read the current definition in `defaults/{category}.go`, then consult the editing table in `.Codex/rules/settings-system.md`. Key changes that need data migrations: renaming a key.

## Task: Delete a Setting

Follow the deletion checklist in `.Codex/rules/settings-system.md` — includes removing key, definition, consumers, tests, AND data migration for orphaned DB rows.

## Task: Debug a Setting

1. Check the key exists: `grep -r "KeyMySettingName" backend/models/config/keys.go`
2. Check it's registered: `grep -r "KeyMySettingName" backend/services/config/defaults/`
3. Check consuming code uses HasTenantOverride (not bare Resolve)
4. Check DB state: `SELECT * FROM config.setting_values WHERE setting_key = 'category.key'`
5. Check audit trail: `SELECT * FROM config.setting_audit WHERE setting_key = 'category.key' ORDER BY changed_at DESC`

## Resolve Method Reference

| Method | Use when |
|--------|----------|
| `ResolveString(ctx, key)` | Inside tenant middleware (ctx has tenant) |
| `ResolveBool(ctx, key)` | Boolean settings |
| `ResolveInt(ctx, key)` | Number settings (whole integers only) |
| `ResolveStringForTenant(ctx, tenantID, key)` | Outside tenant middleware (device auth, scheduler) |
| `HasTenantOverride(ctx, key)` | Check before Resolve when env var fallback needed |

## Common Mistakes

| Mistake | Consequence |
|---------|-------------|
| Call `Resolve*()` without `HasTenantOverride` | Registry default returned instead of env var fallback |
| Hardcode key string instead of constant | Breaks if key renamed; no compile-time safety |
| Use `config:update` for GDPR/security settings | Wrong permission level; should be `config:manage` |
| Forget data migration when deleting a setting | Orphaned rows in config.setting_values |
| Use fractional number as default | Validation rejects non-integers |
