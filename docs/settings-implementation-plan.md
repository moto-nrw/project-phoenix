# Settings System: Implementation Plan

**Author:** yungweng
**Status:** Draft — trimmed from Chris's RFC to MVP scope
**Date:** 2026-03-27T15:00:00+02:00

---

## Principle

Same core idea as the RFC (registry + schema-driven UI), scoped to what we need today: 11 tenant-level settings, 3 field types, no cascade, no actions.

---

## 1. Backend

### 1.1 Registry

Minimal definition struct in `models/config/registry.go`:

```go
type FieldType string

const (
    FieldBoolean FieldType = "boolean"
    FieldNumber  FieldType = "number"
    FieldTime    FieldType = "time"
    FieldText    FieldType = "text"
)

type Definition struct {
    Key             string
    Label           string        // German display text, i18n later
    Description     string
    Type            FieldType
    Default         interface{}
    Validation      *ValidationRules
    ReadPermission  string
    WritePermission string
    Tab             string
    Category        string
    SortOrder       int
}

type ValidationRules struct {
    Required bool
    Min      *float64
    Max      *float64
}
```

Registry singleton with `Register()`, `GetDefinition()`, `AllDefinitions()`. Panics on duplicate keys at startup.

### 1.2 Setting Registration

One file per domain, called via `init()`:

```go
// services/config/defaults/operations.go

func init() {
    config.Register(config.Definition{
        Key:             "operations.session_end_time",
        Label:           "Sitzungsende",
        Description:     "Uhrzeit, zu der alle aktiven Sitzungen automatisch beendet werden",
        Type:            config.FieldTime,
        Default:         "18:00",
        ReadPermission:  "config:read",
        WritePermission: "config:update",
        Tab:             "operations",
        Category:        "sessions",
        SortOrder:       1,
    })
    // ... remaining settings
}
```

**Settings to register:**

| Key | Type | Default | Tab | Category |
|-----|------|---------|-----|----------|
| `security.ogs_device_pin` | text | `""` | security | auth |
| `operations.session_end_enabled` | boolean | `true` | operations | sessions |
| `operations.session_end_time` | time | `18:00` | operations | sessions |
| `operations.session_end_timeout_minutes` | number | `10` | operations | sessions |
| `operations.student_daily_checkout_time` | time | `15:00` | operations | checkout |
| `operations.session_cleanup_enabled` | boolean | `true` | operations | cleanup |
| `operations.session_cleanup_interval_minutes` | number | `15` | operations | cleanup |
| `operations.session_abandoned_threshold_minutes` | number | `60` | operations | cleanup |
| `gdpr.data_cleanup_enabled` | boolean | `true` | gdpr | cleanup |
| `gdpr.data_cleanup_time` | time | `02:00` | gdpr | cleanup |
| `gdpr.data_cleanup_timeout_minutes` | number | `30` | gdpr | cleanup |

### 1.3 Database

New table `config.setting_values`:

```sql
CREATE TABLE config.setting_values (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id),
    setting_key VARCHAR(255) NOT NULL,
    value       JSONB NOT NULL,
    updated_by  BIGINT REFERENCES auth.accounts(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_tenant_setting UNIQUE (tenant_id, setting_key)
);

CREATE INDEX idx_setting_values_tenant ON config.setting_values (tenant_id);

ALTER TABLE config.setting_values ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON config.setting_values
    USING (tenant_id = current_setting('app.current_tenant_id')::BIGINT);
```

No `scope_type`/`scope_id` columns — every row is tenant-scoped. No audit table for now (git-blame the migration if you need history, add audit later if required).

Data migration: copy relevant rows from existing `config.settings` to `config.setting_values`.

### 1.4 Resolution

One function, not a service with chain walking:

```go
func (s *SettingsService) Resolve(ctx context.Context, key string) (interface{}, error) {
    def := config.GetDefinition(key)
    if def == nil {
        return nil, fmt.Errorf("unknown setting: %s", key)
    }

    tenantID := tenant.FromContext(ctx)

    // Check tenant override
    val, err := s.repo.FindByTenantAndKey(ctx, tenantID, key)
    if err == nil {
        return val.Value, nil
    }

    // Fallback to registry default
    return def.Default, nil
}
```

### 1.5 API Endpoints

Three endpoints mounted at `/api/settings`:

```
GET    /api/settings/schema         → full schema + resolved values for current tenant
PUT    /api/settings/values/{key}   → set tenant override
DELETE /api/settings/values/{key}   → reset to registry default
```

**Schema response shape:**

```json
{
  "tabs": [
    {
      "key": "operations",
      "label": "Betrieb",
      "categories": [
        {
          "key": "sessions",
          "label": "Sitzungen",
          "items": [
            {
              "key": "operations.session_end_time",
              "type": "time",
              "label": "Sitzungsende",
              "description": "...",
              "value": "18:00",
              "default": "18:00",
              "is_default": true,
              "writable": true,
              "validation": null
            }
          ]
        }
      ]
    }
  ]
}
```

### 1.6 Compat Wrapper

Wrap existing `ConfigService.GetStringValue()` etc. to delegate to the new resolver. Existing code keeps working without changes.

---

## 2. Scheduler Refactor

**This is the hardest part.** The scheduler currently reads env vars once at boot. It needs to become tenant-aware.

### Current

```go
// One global value, read at startup
endTime := os.Getenv("SESSION_END_TIME") // "18:00"
// One cron job fires at 18:00 for ALL tenants
```

### Target

```go
// Scheduler runs a periodic check (e.g. every minute)
// For each active tenant:
//   1. Resolve "operations.session_end_time" for that tenant
//   2. If current time matches, run session-end for that tenant
```

### Approach

- Add `SettingsService` as a dependency to the scheduler
- Replace each `os.Getenv()` call with `settingsService.Resolve(ctx, key)`
- The scheduler iterates tenants and applies per-tenant config
- Env vars become the registry defaults (fallback if no tenant override exists)
- Remove the env vars from `.env.example` / `docker-compose.example.yml` once migrated

---

## 3. Frontend

### 3.1 File Structure

```
frontend/src/
├── lib/
│   └── settings-api.ts              # fetch schema, put/delete values
└── components/settings/
    ├── settings-page.tsx             # fetches schema, renders tabs
    ├── settings-category.tsx         # group of fields with heading
    ├── settings-field.tsx            # single row: label + component + reset button
    └── fields/
        ├── boolean-field.tsx         # toggle/switch
        ├── number-field.tsx          # number input with validation
        ├── time-field.tsx            # time picker (HH:MM)
        └── text-field.tsx            # text input
```

5-6 files total.

### 3.2 Settings Page

- Fetches `GET /api/settings/schema` on mount
- Renders tabs from response (existing `SettingsLayout` pattern with `extraTabs`)
- Each tab contains categories, each category contains fields
- Field component chosen by `type` → component map
- Save on change (debounced PUT), reset button calls DELETE
- "Default" badge shown when `is_default: true`

### 3.3 Integration

The existing settings page at `/[tenant]/(protected)/settings/` has Profile and Security tabs. The new tabs (Operations, GDPR, Security) are injected via `extraTabs` on `SettingsLayout`.

---

## What We're NOT Building (Yet)

Parked from the RFC for later, if ever needed:

| Feature | When to revisit |
|---------|----------------|
| Scope cascade (user, device, group, room, org) | When a setting genuinely needs per-room or per-device values |
| Conditional dependencies (`DependsOn`) | When the settings page grows beyond one screen |
| Action buttons in settings | Keep these on their respective domain pages (facilities, system admin) |
| Dynamic select options | When a setting needs a dropdown populated from an API |
| Custom renderers | When a setting needs exotic UI |
| Audit trail table + UI | When compliance requires change history |
| Bulk operations | When managing settings across many tenants |
| Import/export | When migrating between environments |
| Settings search | When there are enough settings to get lost in |
| i18n keys | When we need multi-language support |

---

## Phases

| Phase | Scope | Estimate |
|-------|-------|----------|
| **1. Backend** | Registry, table, resolver, 3 endpoints, register 11 settings, compat wrapper | — |
| **2. Scheduler** | Make scheduler tenant-aware, replace `os.Getenv()` calls with resolver | — |
| **3. Frontend** | Settings page with schema-driven rendering, 4 field components | — |

---

## Issue 1856 activation rollout

Ten previously registered settings now have runtime consumers. Before deploying this activation, inventory existing overrides without modifying them:

```bash
docker compose run --rm server go run . settings overrides
docker compose run --rm server go run . settings overrides --json
```

Review non-default values with the responsible operator or school. Preserve `config.setting_values` and `config.setting_audit`; no cleanup migration removes either. The activated keys are `attendance.web_enabled`, `operations.group_mode`, `timetable.show_expected_children_count`, `enrollment.collect_grade_level`, `enrollment.care_offerings_enabled`, `enrollment.notify_per_decision`, `enrollment.duplicate_handling`, `enrollment.rejected_retention_days`, `enrollment.waitlist_enabled`, and `enrollment.auto_invite_guardian_on_approval`.

Resolution failures on permission, mutation, and deletion paths fail the request or retain restrictive behavior. Disabling care offerings preserves the catalog and stored selections. Disabling waitlists makes a phase overflow mode of `waitlist` effectively `allow`, so no forbidden waitlist status is created. Rejected enrollment cleanup runs under the existing nightly GDPR schedule and deletes only requests whose children are all rejected and older than the configured retention period. Decision emails use tenant-scoped outbox idempotency keys.

### `operations.group_mode` access inventory

`open_care` broadens group scope only after the route permission check succeeds. It does not change student-data access, photo access, planned or actual pickup-time access, or any other GDPR setting.

| Path | Fixed groups | Open care |
|---|---|---|
| Active-room dashboard read and filter | Staff member's supervised rooms | All active rooms |
| Active-room roster read | Requires supervision of the active room | Any staff member with `groups:read`; student fields keep their normal GDPR filter |
| Web check-in | Requires the configured group or supervision match | Any staff member with the route's check-in permission |
| Transit assignment and room moves | Source and target must fall within supervised groups | Any staff member with `visits:update` may use any active source and target |
| Timetable `planned-now`, roster, start, complete, and attendance writes | Requires instance assignment or admin overview | Any staff member with `schedules:read` may operate the instance |

### Rejected-enrollment cascade policy

The cleanup selects a request only when it has children, every child is rejected, and every rejection date is older than the tenant's retention cutoff. A request with any approved, waitlisted, withdrawn, submitted, or under-review child stays intact. The cleanup first deletes email-outbox rows linked to the request, then deletes the request. Database cascades remove request children, child offerings, extra guardians, change requests and messages, and offering-adjustment audit rows. It does not delete students, guardian profiles, accounts, phases, form schemas, or care-offering catalogs. A savepoint rolls back the whole request cleanup if any dependent delete fails.
