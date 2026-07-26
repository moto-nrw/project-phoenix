# Future Settings Backlog

**Date:** 2026-04-09
**Status:** Draft — comprehensive audit of hardcoded values that should become tenant-scoped settings.

This document catalogs every hardcoded business-logic decision in Project Phoenix that should eventually be configurable per school via the settings system. It covers both the main app (project-phoenix) and the device kiosk (PyrePortal).

---

## Currently Implemented (21 settings)

For reference, these settings already exist in the registry (`services/config/defaults/`):

| Key | Type | Default | Tab | File |
|-----|------|---------|-----|------|
| `security.ogs_device_pin` | password | `"1234"` | security | `defaults/security.go` |
| `operations.session_end_enabled` | boolean | `true` | operations | `defaults/operations.go` |
| `operations.session_end_time` | time | `"18:00"` | operations | `defaults/operations.go` |
| `operations.session_end_timeout_minutes` | number | `10` | operations | `defaults/operations.go` |
| `operations.student_daily_checkout_time` | time | `""` (optional — empty = always available) | operations | `defaults/operations.go` |
| `operations.session_cleanup_enabled` | boolean | `false` | operations | `defaults/operations.go` |
| `operations.session_cleanup_interval_minutes` | number | `15` | operations | `defaults/operations.go` |
| `operations.session_abandoned_threshold_minutes` | number | `60` | operations | `defaults/operations.go` |
| `gdpr.data_cleanup_enabled` | boolean | `true` | gdpr | `defaults/gdpr.go` |
| `gdpr.data_cleanup_time` | time | `"02:00"` | gdpr | `defaults/gdpr.go` |
| `gdpr.data_cleanup_timeout_minutes` | number | `30` | gdpr | `defaults/gdpr.go` |
| `gdpr.attendance_log_enabled` | boolean | `false` | gdpr | `defaults/gdpr.go` |
| `gdpr.attendance_visible_days` | number | `30` | gdpr | `defaults/gdpr.go` |
| `gdpr.room_detail_visible_days` | number | `7` | gdpr | `defaults/gdpr.go` |
| `gdpr.attendance_log_scope` | select | `"group_supervisors_only"` | gdpr | `defaults/gdpr.go` |
| `gdpr.student_data_scope` | select | `"group_supervisors_only"` | gdpr | `defaults/gdpr.go` |
| `feedback.enabled` | boolean | `false` | gdpr | `defaults/feedback.go` |
| `feedback.data_retention_days` | number | `90` | gdpr | `defaults/feedback.go` |
| `checkout.raumwechsel_enabled` | boolean | `true` | devices | `defaults/devices.go` |
| `checkout.schulhof_enabled` | boolean | `false` | devices | `defaults/devices.go` |
| `checkout.wc_enabled` | boolean | `false` | devices | `defaults/devices.go` |

---

## Priority 1 — Checkout & Device Screen Configuration

**Status:** Ready to implement
**Decisions:** Finalized 2026-04-09

These settings define what students see on the RFID kiosk (PyrePortal). Currently every school gets the exact same checkout experience with no way to customize it.

### Scope (V1)

5 settings total: 3 new checkout button toggles + 2 existing settings exposed to devices via a new config endpoint. Also requires a behavioral change to make `student_daily_checkout_time` optional.

### New Settings (3)

All go in the new **devices** tab, category **checkout**. Permission: `config:update`.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `checkout.raumwechsel_enabled` | boolean | `true` | Show "Raumwechsel" button on the device checkout screen |
| `checkout.schulhof_enabled` | boolean | `true` | Show "Schulhof" button (still requires the Schulhof room to exist) |
| `checkout.wc_enabled` | boolean | `true` | Show "Toilette" button (still requires the WC room to exist) |

### Existing Settings Exposed to Devices (2)

These already exist and work in the admin settings UI. They just need to be included in the new IoT config endpoint so PyrePortal can read them.

| Key | Current Tab | Behavioral Change |
|-----|-------------|-------------------|
| `feedback.enabled` | operations | None — already controls feedback modal. Just expose via config endpoint. |
| `operations.student_daily_checkout_time` | operations | **Make optional.** Currently defaults to `"15:00"` if unset. New behavior: if no value is set (empty/null), the "nach Hause" button is always available (no time restriction). If a time IS set, the button only appears after that time. |

### New Endpoint: `GET /api/iot/config`

Delivers device-relevant settings to PyrePortal. Called once on page mount, cached by the device.

**Auth:** Device API key only (no staff PIN required) — same auth level as `GET /api/iot/school-name`. Settings are non-sensitive school configuration, not student data.

**Response shape:**

```json
{
  "checkout": {
    "raumwechsel_enabled": true,
    "schulhof_enabled": true,
    "wc_enabled": true,
    "daily_checkout_time": "15:00"
  },
  "feedback": {
    "enabled": true
  }
}
```

- `daily_checkout_time`: string `"HH:MM"` or `null`. When `null`, the "nach Hause" button is always available (no time gate). When set, the button only appears after that time.
- `feedback.enabled`: boolean. When `false`, the feedback modal is skipped entirely after "nach Hause".

### Backend Implementation Plan

#### 1. Register 3 new settings (`services/config/defaults/devices.go` — new file)

```go
// New file: services/config/defaults/devices.go
func init() {
    config.Register(config.Definition{
        Key:             config.KeyCheckoutRaumwechselEnabled,
        Label:           "Raumwechsel-Button anzeigen",
        Description:     "Zeigt den Raumwechsel-Button auf dem Geräte-Checkout-Bildschirm",
        Type:            config.FieldBoolean,
        Default:         true,
        ReadPermission:  "config:read",
        WritePermission: "config:update",
        Tab:             "devices",
        Category:        "checkout",
        SortOrder:       1,
    })
    // ... same pattern for schulhof_enabled (SortOrder: 2) and wc_enabled (SortOrder: 3)
}
```

#### 2. Add key constants (`models/config/keys.go`)

```go
// Checkout button settings (devices tab).
const (
    KeyCheckoutRaumwechselEnabled = "checkout.raumwechsel_enabled"
    KeyCheckoutSchulhofEnabled    = "checkout.schulhof_enabled"
    KeyCheckoutWCEnabled          = "checkout.wc_enabled"
)
```

#### 3. Make `student_daily_checkout_time` optional

**Current behavior** (`api/iot/checkin/helpers.go:17-64`):
- Fallback chain: tenant DB override → `STUDENT_DAILY_CHECKOUT_TIME` env var → `"15:00"`
- Always has a value — "nach Hause" is always time-gated

**New behavior:**
- Fallback chain: tenant DB override → env var → **no default (nil)**
- If resolved value is empty/nil → `shouldShowDailyCheckoutWithGroup()` skips the time check → "nach Hause" always available
- If resolved value is set → current time-gate logic applies

**Change in `defaults/operations.go`:** Change `Default` from `"15:00"` to `""` (empty string).

**Change in `helpers.go:getStudentDailyCheckoutTime()`:** Return `(nil, nil)` when no time is configured. Callers treat nil as "always available".

**Change in `helpers.go:shouldShowDailyCheckoutWithGroup()`:** If checkout time is nil, skip the time check entirely.

#### 4. New IoT config endpoint (`api/iot/config.go` — new file)

```go
// GET /api/iot/config — device API key only (no PIN)
func (rs *Resource) getDeviceConfig(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Resolve all device-relevant settings
    raumwechsel, _ := rs.SettingsService.ResolveBool(ctx, configModel.KeyCheckoutRaumwechselEnabled)
    schulhof, _ := rs.SettingsService.ResolveBool(ctx, configModel.KeyCheckoutSchulhofEnabled)
    wc, _ := rs.SettingsService.ResolveBool(ctx, configModel.KeyCheckoutWCEnabled)
    feedbackEnabled, _ := rs.SettingsService.ResolveBool(ctx, configModel.KeyFeedbackEnabled)
    dailyCheckoutTime, _ := rs.SettingsService.ResolveString(ctx, configModel.KeyStudentDailyCheckoutTime)

    response := map[string]interface{}{
        "checkout": map[string]interface{}{
            "raumwechsel_enabled": raumwechsel,
            "schulhof_enabled":   schulhof,
            "wc_enabled":         wc,
            "daily_checkout_time": nilIfEmpty(dailyCheckoutTime),
        },
        "feedback": map[string]interface{}{
            "enabled": feedbackEnabled,
        },
    }

    render.JSON(w, r, response)
}
```

#### 5. Register route in `api/iot/api.go`

Add under the device-only auth group (API key, no PIN):

```go
r.Get("/config", rs.getDeviceConfig)
```

### PyrePortal Changes (Document Only — Separate PR)

Changes needed in `PyrePortal/src/pages/ActivityScanningPage.tsx`:

#### 1. Fetch config on mount

Add alongside the existing Schulhof/WC room fetch (`ActivityScanningPage.tsx:442-478`):

```typescript
// Fetch device config (checkout button visibility, feedback settings)
const config = await api.getDeviceConfig(authenticatedUser.pin);
setDeviceConfig(config); // new state variable
```

New API method in `PyrePortal/src/services/api.ts`:

```typescript
async getDeviceConfig(pin: string): Promise<DeviceConfig> {
  const response = await this.fetchWithAuth('/api/iot/config', { pin });
  return response.json();
}
```

#### 2. Filter checkout buttons based on config

Currently (`ActivityScanningPage.tsx:927-1008`): buttons are always included or conditional on room existence.

Change to:

```typescript
const destinations = [
  // Raumwechsel: was always shown, now conditional on config
  ...(deviceConfig?.checkout.raumwechsel_enabled !== false ? [raumwechselButton] : []),

  // Schulhof: was conditional on room existing, now also needs config
  ...(schulhofRoomId && deviceConfig?.checkout.schulhof_enabled !== false ? [schulhofButton] : []),

  // WC: same pattern as Schulhof
  ...(wcRoomId && deviceConfig?.checkout.wc_enabled !== false ? [wcButton] : []),

  // nach Hause: was conditional on daily_checkout_available, stays the same
  ...(checkoutDestinationState.dailyCheckoutAvailable ? [nachHauseButton] : []),
];
```

#### 3. Daily checkout time handling

The backend already controls `daily_checkout_available` in the checkin response (`workflow.go:844`). Making the time optional only changes backend logic — PyrePortal already reads the boolean flag. **No PyrePortal change needed for this.**

#### 4. Feedback visibility

Currently (`ActivityScanningPage.tsx:612-640`): `handleNachHause` always shows feedback prompt.

Change to: check `deviceConfig?.feedback.enabled` before showing feedback. If `false`, skip straight to farewell screen.

```typescript
const handleNachHause = async () => {
  // ... existing daily checkout API call ...

  if (deviceConfig?.feedback.enabled !== false) {
    setShowFeedbackPrompt(true);
  } else {
    // Skip feedback, go straight to farewell
    setCheckoutDestinationState(prev => (prev ? { ...prev, showingFarewell: true } : null));
  }
};
```

**Note:** `feedback_enabled` is already returned in the `AttendanceToggleResponse` (`attendance/types.go:77`), so PyrePortal could also read it from there. However, reading from the config endpoint is simpler — one source of truth, fetched once on mount.

### Files Changed (Backend)

| File | Change |
|------|--------|
| `models/config/keys.go` | Add 3 new key constants |
| `services/config/defaults/devices.go` | **New file** — register 3 checkout settings |
| `services/config/defaults/operations.go` | Change `student_daily_checkout_time` default from `"15:00"` to `""` |
| `api/iot/checkin/helpers.go` | Make `getStudentDailyCheckoutTime()` return nil when unset; update `shouldShowDailyCheckoutWithGroup()` to skip time check when nil |
| `api/iot/config.go` | **New file** — `GET /api/iot/config` handler |
| `api/iot/api.go` | Register `/config` route under device-only auth |

### Files Changed (PyrePortal — Separate PR)

| File | Change |
|------|--------|
| `services/api.ts` | Add `getDeviceConfig()` method + `DeviceConfig` type |
| `pages/ActivityScanningPage.tsx` | Fetch config on mount; filter buttons by config; check feedback enabled |

### Deferred (Future P1)

These were originally in P1 but need more design work:

| What | Why Deferred |
|------|-------------|
| `checkout.screen_timeout_seconds` | Low value — 7s works for most schools. Can add later. |
| `checkout.farewell_timeout_seconds` | Same — 1.5s is fine. |
| `feedback.question_text` | Needs placeholder convention (`{name}` interpolation). |
| `feedback.options_count` | Needs backend validation changes + new feedback value constants. |
| `feedback.mensa_enabled` | New screen in PyrePortal, second API call. |
| `devices.rfid_enabled` | No frontend UI adapts to this yet. |
| `devices.checkin_method` | Needs broader design (what does "web-only" mean for device endpoints?). |
| `devices.student_self_checkin` | Needs permission model changes. |
| Custom destinations beyond 4 defaults | Needs a new data model, not just a setting. |

---

## Priority 2 — Module Toggles

These settings control whether entire feature areas are visible in the sidebar/navigation. Currently every school sees the same modules — some are always on, some are hardcoded as "coming soon".

**Problem:** The sidebar (`frontend/src/components/dashboard/sidebar.tsx`) uses `alwaysShow: true`, `comingSoon: true`, `requiresAdmin: true`, and `hideForAdmin: true` flags, but none of these are per-school configurable.

| Proposed Key | Type | Default | Currently | Location |
|---|---|---|---|---|
| `modules.time_tracking_enabled` | boolean | `true` | Always ON | `sidebar.tsx:78` — `alwaysShow: true` |
| `modules.suggestions_enabled` | boolean | `true` | Always ON | `sidebar.tsx:124` — `alwaysShow: true` |
| `modules.mensa_enabled` | boolean | `false` | Always OFF | `sidebar.tsx:92` — `comingSoon: true` |
| `modules.substitutions_enabled` | boolean | `true` | Admin-only, always visible | `sidebar.tsx` — `requiresAdmin` |
| `modules.reminders_enabled` | boolean | `true` | Caregiver-only, always visible | `sidebar.tsx:105` — `hideForAdmin` |
| `modules.schulhof_banner_enabled` | boolean | `false` | Always OFF | `unclaimed-rooms.tsx:18` — `const = false` |

**Implementation approach:** The frontend would fetch module toggles from the settings schema and use them alongside the existing role-based conditions. Backend would expose these as read-only to non-admins.

**Sidebar history links** (student detail page, `HistoryLinks.tsx`):

| Proposed Key | Type | Default | Currently |
|---|---|---|---|
| `modules.feedback_history_enabled` | boolean | `false` | Disabled button (`HistoryLinks.tsx:47`) |
| `modules.mensa_history_enabled` | boolean | `false` | Disabled button (`HistoryLinks.tsx:54`) |

---

## Priority 3 — Operations & Dashboard Tuning

### Dashboard Display

| Proposed Key | Type | Default | Hardcoded In | Description |
|---|---|---|---|---|
| `operations.pickup_urgency_minutes` | number | `30` | `frontend: ogs-group-helpers.ts:22` | Minutes before pickup time to show "soon" warning |
| `operations.dashboard_recent_minutes` | number | `30` | `dashboard_helpers.go:379` | How long activities appear as "recently started" |
| `operations.activity_full_percent` | number | `80` | `dashboard_helpers.go:458` | Percentage at which activity shows "ending soon" status |
| `operations.dashboard_refresh_seconds` | number | `300` | `frontend: dashboard/page.tsx:264` | Dashboard auto-refresh interval |

**Note:** Break compliance thresholds (6h/9h, 30min/45min) are German labor law (ArbZG). These could be org-level overrides for Träger with stricter internal policies, but changing them has legal implications.

---

## Priority 4 — GDPR & Privacy

### Retention Defaults

**Problem:** Per-student data retention is validated as 1-31 days (`student-form-validation.ts:20-23`, `student_import_config.go:233`), but the GDPR settings allow `attendance_visible_days` up to 365. These should be aligned or the per-student max should be a setting.

| Proposed Key | Type | Default | Hardcoded In | Description |
|---|---|---|---|---|
| `gdpr.default_student_retention_days` | number | `30` | `cleanup_service.go:112` | Default retention when no explicit consent exists |
| `gdpr.max_student_retention_days` | number | `31` | `student_import_config.go:233`, `student-form-validation.ts:20` | Maximum retention allowed per student — currently capped at 31, should potentially match `attendance_visible_days` range |

---

## Priority 5 — Security & Auth

These are currently env-var-only or hardcoded. They could be per-tenant or per-organization settings.

| Proposed Key | Type | Default | Hardcoded In | Description |
|---|---|---|---|---|
| `security.invitation_expiry_hours` | number | `48` | `services/factory.go:120` | How long invitation links are valid (1-168h) |
| `security.password_reset_expiry_minutes` | number | `30` | `services/factory.go:128` | How long password reset links are valid (1-1440min) |
| `security.max_sessions_per_account` | number | `5` | `services/auth/auth_login.go:147` | Max concurrent sessions per user account |

---

## Priority 6 — Provisioning & Infrastructure Defaults

These are hardcoded values that define what a school gets when it's first set up.

### System Room Capacities

| What | Hardcoded Value | Location | Description |
|---|---|---|---|
| Schulhof room capacity | `300` | `constants/activities.go:26` | Max students on playground |
| Schulhof max participants | `300` | `constants/activities.go:29` | Max participants in Schulhof activity |
| WC room capacity | `20` | `constants/activities.go:50` | Max students in bathroom |
| WC max participants | `20` | `constants/activities.go:53` | Max participants in WC activity |

These could become settings like `infrastructure.schulhof_capacity` and `infrastructure.wc_capacity`, but they're rarely changed after initial setup. An alternative is making them editable via the room management UI (they're currently auto-created and not editable).

### Default Activity Categories

When a school is provisioned, 9 activity categories are auto-seeded (`operator_provisioning_service.go:1327-1356`):

1. Sport
2. Kunst & Basteln
3. Musik
4. Spiele
5. Lesen
6. Hausaufgabenhilfe
7. Natur & Forschen
8. Computer
9. Gruppenraum

These are reasonable defaults but can't be customized per-organization. A Träger creating multiple schools might want a consistent set of categories. This is more of an **org-level template** than a per-tenant setting.

---

## Priority 7 — Organization-Level Settings (Future Architecture)

The settings system currently only supports tenant (school) scope. Some settings naturally belong at the organization (Träger) level. This requires extending the settings architecture to support org-scoped resolution (org override → tenant override → registry default).

| Proposed Key | Type | Default | Scope | Description |
|---|---|---|---|---|
| Break thresholds | numbers | `360/540/30/45` | org | ArbZG defaults, org may have stricter policy |
| JWT access lifetime | duration | `1h` | org | Security posture per organization |
| JWT refresh lifetime | duration | `168h` | org | Session duration policy |
| Upload size limits | numbers | `10MB/5MB` | org | IT policy for file imports and avatars |
| Mandatory 2FA | boolean | `false` | org | Require all accounts to use 2FA |

**Blocked by:** No org-scoped settings layer exists yet. The `SettingsService` would need a new resolution chain: org override → tenant override → registry default.

---

## Not Settings (Data Model Changes)

Some items from the knowledge base require new data models, not just settings:

| Feature | Why Not a Setting |
|---|---|
| Custom checkout destinations (beyond 4 defaults) | Needs a `checkout_destinations` table, room associations, device API changes |
| Custom feedback questions | Needs a `feedback_questions` table, versioning, multi-language support |
| Attendance mode (room vs. day tracking) | Fundamental data model difference — `active.visits` are per-room |
| Custom form fields per school | Needs a `custom_fields` table with field definitions and value storage |
| AG enrollment/priority system | Needs enrollment, waitlist, and priority scoring models |
| Parent portal | Entirely new frontend + auth flow |
| Billing/Abrechnung | Entire new domain with financial models |

---

## Cross-Repo Impact (PyrePortal)

Many checkout/feedback settings require changes in PyrePortal (`../PyrePortal/`). The current device kiosk hardcodes:

| What | File | Lines | Impact |
|---|---|---|---|
| Checkout button labels, colors, icons | `ActivityScanningPage.tsx` | 927-1008 | Must fetch config from backend API |
| Feedback button count and labels | `ActivityScanningPage.tsx` | 164-168 | Must fetch feedback config from API |
| Screen timeouts (7s checkout, 1.5s farewell) | `ActivityScanningPage.tsx` | 34, 40 | Could read from API or use defaults |
| "Wie war dein Tag?" question text | `ActivityScanningPage.tsx` | 1395 | Must come from API |
| Destination values ("zuhause", "unterwegs") | `api.ts` | 1320-1350 | Backend validation must also change |

**Coordination needed:** Any setting that affects the device screen requires:
1. New setting key + registration in backend
2. New API field in the checkin/checkout response
3. PyrePortal reading config from the API instead of using hardcoded values
4. PyrePortal release + device update via balena

---

## Summary

| Priority | Category | New Settings | Effort | Status |
|---|---|---|---|---|
| **P1** | Checkout & Device Screen (V1) | 3 new + 2 existing exposed + 1 behavioral change | Medium (backend + PyrePortal separate PR) | **Ready to implement** |
| P1-deferred | Checkout & Device Screen (V2) | ~8 | High (validation changes, new screens) | Needs design |
| P2 | Module Toggles | ~8 | Medium (frontend + settings registry) | Backlog |
| P3 | Operations & Dashboard | ~5 | Low (frontend + settings registry) | Backlog |
| P4 | GDPR & Privacy | ~2 | Low (align existing validation) | Backlog |
| P5 | Security & Auth | ~3 | Medium (wire env vars into settings) | Backlog |
| P6 | Provisioning Defaults | ~4 | Low (make auto-created rooms editable) | Backlog |
| P7 | Org-Level Settings | ~5 | High (new settings scope layer) | Blocked |
| — | Not Settings (data models) | ~7 | Varies (separate feature work) | — |
| **Total** | | **~35+ new settings** | | |
