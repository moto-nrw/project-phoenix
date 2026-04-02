# RFC: Settings System for Project Phoenix

**Status:** Draft — awaiting team approval
**Author:** Chris (with Claude)
**Date:** 2026-03-26
**Branch:** TBD (will be `feature/settings-system`)

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Requirements](#2-requirements)
3. [Architecture Overview](#3-architecture-overview)
4. [Backend Design](#4-backend-design)
5. [Frontend Design](#5-frontend-design)
6. [Scope System (Deep Dive)](#6-scope-system-deep-dive)
7. [Field Types & Renderers](#7-field-types--renderers)
8. [Permission Model](#8-permission-model)
9. [Dependencies & Conditional Visibility](#9-dependencies--conditional-visibility)
10. [Action Buttons](#10-action-buttons)
11. [Dynamic Select Options](#11-dynamic-select-options)
12. [Categorization (Tabs & Categories)](#12-categorization-tabs--categories)
13. [Database Schema](#13-database-schema)
14. [API Endpoints](#14-api-endpoints)
15. [Adding a New Setting (Developer Guide)](#15-adding-a-new-setting-developer-guide)
16. [Implementation Phases](#16-implementation-phases)
17. [Open Questions](#17-open-questions)

---

## 1. Problem Statement

Project Phoenix needs a flexible, extensible settings system that supports:

- Multiple scoping targets (tenant, user, device, group, room, system-wide, etc.)
- Multiple field types (text, number, boolean, select, date, color, etc.)
- Permission-gated visibility and editing
- Conditional visibility (parent/child dependencies)
- Action buttons that trigger backend/frontend operations
- Dynamic select options populated from API queries
- Default values with cascading inheritance
- Easy registration of new settings without touching UI code
- Tab and category-based organization

The current `config.settings` table is a flat key-value store scoped only to tenants. It lacks scope cascading, type metadata, conditional logic, and permission control.

---

## 2. Requirements

### Must Have

| # | Requirement | Description |
|---|-------------|-------------|
| R1 | Multi-scope | Settings can be scoped to: system, tenant, organization, user, device, group, room |
| R2 | Scope cascading | Values resolve through a hierarchy: most specific scope wins, falling back to parent scopes |
| R3 | Typed fields | Support at minimum: text, number, boolean, select, date, time, color |
| R4 | Dynamic selects | Select options populated from API endpoints (e.g., list of rooms, groups) |
| R5 | Default values | Every setting has a registry default; overrides are optional |
| R6 | Permissions | Per-setting read and write permissions; unauthorized settings are hidden |
| R7 | Conditional visibility | Settings can depend on a parent setting's value (e.g., show time picker only if auto-checkout is enabled) |
| R8 | Action buttons | Buttons in the settings UI that trigger backend/frontend operations (e.g., clear cache, export data) |
| R9 | Categorization | Settings organized into tabs and categories |
| R10 | Easy registration | Adding a new setting = adding a registry entry + no UI code changes |

### Should Have

| # | Requirement | Description |
|---|-------------|-------------|
| R11 | Audit trail | Track who changed which setting, when, and what the previous value was |
| R12 | Inherited-from indicator | UI shows where a value was inherited from (e.g., "inherited from: Tenant") |
| R13 | Reset to default | Allow removing an override to fall back to the parent scope |
| R14 | Bulk operations | Set a value across all tenants or all rooms at once |
| R15 | i18n-ready | Labels and descriptions use translation keys |

### Nice to Have

| # | Requirement | Description |
|---|-------------|-------------|
| R16 | Custom renderers | Escape hatch for exotic settings that need a custom React component |
| R17 | Import/export | Export settings as JSON for backup or migration between environments |
| R18 | Settings search | Full-text search across setting labels and descriptions |

---

## 3. Architecture Overview

### Core Principle: Schema-Driven with Escape Hatches

Settings are defined as **metadata in a backend registry**. The backend is the single source of truth for:
- What settings exist, their types, defaults, and validation
- Which scopes each setting supports
- Permission requirements
- Dependency relationships

The frontend **renders dynamically** from this schema. Adding a new setting requires zero frontend code changes — the renderer maps types to components automatically.

### Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                        BACKEND                                   │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────────────┐  │
│  │   Registry    │    │   Service    │    │   Repository      │  │
│  │ (definitions) │───▶│ (resolution  │───▶│ (config.settings  │  │
│  │              │    │  + logic)    │    │   + audit table)  │  │
│  └──────────────┘    └──────────────┘    └───────────────────┘  │
│         │                    │                                    │
│         ▼                    ▼                                    │
│  GET /settings/schema  GET/PUT /settings/values                  │
└─────────────────────────────────────────────────────────────────┘
           │                    │
           ▼                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                        FRONTEND                                  │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────────────┐  │
│  │ Schema fetch  │───▶│  Renderer    │───▶│  Field Components │  │
│  │ (on mount)   │    │ (dynamic)    │    │  (type → React)   │  │
│  └──────────────┘    └──────────────┘    └───────────────────┘  │
│                                                                  │
│  Types: boolean→Toggle, text→Input, select→Select, etc.        │
│  Custom: settingsRenderers.register("key", CustomComponent)     │
└─────────────────────────────────────────────────────────────────┘
```

### Relationship to Existing Config Domain

The existing `config.settings` table and service handle simple key-value settings scoped to tenants. The new system **replaces** this with a more capable design:

- The existing `config.settings` table gets migrated to the new schema
- Existing settings become registry entries with `Scopes: [ScopeTenant]`
- The existing `ConfigService` methods (`GetStringValue`, `GetBoolValue`, etc.) get a compatibility wrapper that delegates to the new resolver
- API endpoints change (new schema endpoint, scope-aware value endpoints)

---

## 4. Backend Design

### 4.1 Settings Registry

The registry is an in-memory map of setting definitions, populated at application startup via `Register()` calls. It lives in `models/config/`.

```go
// models/config/registry.go

package config

type FieldType string

const (
    FieldText     FieldType = "text"
    FieldNumber   FieldType = "number"
    FieldBoolean  FieldType = "boolean"
    FieldSelect   FieldType = "select"
    FieldDate     FieldType = "date"
    FieldTime     FieldType = "time"
    FieldColor    FieldType = "color"
    FieldPassword FieldType = "password"   // masked input
    FieldTextarea FieldType = "textarea"   // multi-line text
    FieldJSON     FieldType = "json"       // raw JSON editor (escape hatch)
)

type Scope string

const (
    ScopeSystem       Scope = "system"
    ScopeTenant       Scope = "tenant"
    ScopeOrganization Scope = "organization"
    ScopeUser         Scope = "user"
    ScopeDevice       Scope = "device"
    ScopeGroup        Scope = "group"
    ScopeRoom         Scope = "room"
)

type Definition struct {
    // Identity
    Key         string    // unique key, e.g. "attendance.auto_checkout_enabled"
    Label       string    // display label (or i18n key), e.g. "Auto-Checkout"
    Description string    // help text explaining the setting

    // Type & Validation
    Type       FieldType        // determines which frontend component renders
    Default    interface{}      // default value (used when no override exists at any scope)
    Validation *ValidationRules // optional: min, max, pattern, required, etc.

    // Scoping
    Scopes         []Scope  // which scope levels this setting can exist at
    ResolutionChain []Scope  // optional: custom resolution order (defaults to global chain)

    // Permissions
    ReadPermission  string // permission required to see this setting (empty = visible to all authenticated users)
    WritePermission string // permission required to change this setting

    // Organization
    Tab      string // top-level tab grouping, e.g. "general", "attendance", "system"
    Category string // sub-grouping within a tab, e.g. "checkout", "notifications"
    SortOrder int   // display order within category (lower = higher)

    // Dependencies
    DependsOn *Dependency // optional: only show this setting when parent condition is met

    // Select Options (only for FieldSelect)
    Options *SelectOptions // static list or dynamic endpoint
}

type ValidationRules struct {
    Required  bool        // value cannot be empty/null
    Min       *float64    // minimum (for numbers) or min length (for strings)
    Max       *float64    // maximum (for numbers) or max length (for strings)
    Pattern   string      // regex pattern (for strings)
    AllowedValues []interface{} // whitelist of acceptable values
}

type Dependency struct {
    Key       string      // parent setting key
    Condition string      // "eq", "neq", "gt", "lt", "in", "not_empty"
    Value     interface{} // expected value (for eq/neq/gt/lt) or list (for in)
}

type SelectOptions struct {
    // Static options (use one or the other, not both)
    Static []SelectOption

    // Dynamic options (fetched from API)
    Endpoint   string // API path, e.g. "/api/facilities/rooms"
    LabelField string // field name for display label in response
    ValueField string // field name for value in response
}

type SelectOption struct {
    Label string      `json:"label"`
    Value interface{} `json:"value"`
}
```

**Registry singleton:**

```go
// models/config/registry.go (continued)

var registry = make(map[string]*Definition)

// Register adds a setting definition to the registry.
// Called at init() time from domain packages.
// Panics on duplicate keys (catches typos at startup).
func Register(def Definition) {
    if _, exists := registry[def.Key]; exists {
        panic(fmt.Sprintf("duplicate setting key: %s", def.Key))
    }
    if err := def.Validate(); err != nil {
        panic(fmt.Sprintf("invalid setting definition %s: %v", def.Key, err))
    }
    registry[def.Key] = &def
}

// GetDefinition returns a definition by key, or nil if not found.
func GetDefinition(key string) *Definition {
    return registry[key]
}

// AllDefinitions returns all registered definitions.
func AllDefinitions() map[string]*Definition {
    return registry
}
```

**Example registration (from a domain package):**

```go
// services/config/defaults/attendance.go

package defaults

import "github.com/moto-nrw/project-phoenix/models/config"

func init() {
    config.Register(config.Definition{
        Key:         "attendance.auto_checkout_enabled",
        Label:       "settings.attendance.auto_checkout_enabled",
        Description: "settings.attendance.auto_checkout_enabled.description",
        Type:        config.FieldBoolean,
        Default:     false,
        Scopes:      []config.Scope{config.ScopeSystem, config.ScopeTenant},
        ReadPermission:  "config:read",
        WritePermission: "config:update",
        Tab:      "attendance",
        Category: "checkout",
        SortOrder: 1,
    })

    config.Register(config.Definition{
        Key:         "attendance.auto_checkout_time",
        Label:       "settings.attendance.auto_checkout_time",
        Description: "settings.attendance.auto_checkout_time.description",
        Type:        config.FieldTime,
        Default:     "16:00",
        Scopes:      []config.Scope{config.ScopeTenant, config.ScopeRoom},
        ReadPermission:  "config:read",
        WritePermission: "config:update",
        Tab:      "attendance",
        Category: "checkout",
        SortOrder: 2,
        DependsOn: &config.Dependency{
            Key:       "attendance.auto_checkout_enabled",
            Condition: "eq",
            Value:     true,
        },
    })

    config.Register(config.Definition{
        Key:         "attendance.default_room",
        Label:       "settings.attendance.default_room",
        Description: "settings.attendance.default_room.description",
        Type:        config.FieldSelect,
        Default:     nil,
        Scopes:      []config.Scope{config.ScopeTenant},
        ReadPermission:  "config:read",
        WritePermission: "config:update",
        Tab:      "attendance",
        Category: "general",
        SortOrder: 1,
        Options: &config.SelectOptions{
            Endpoint:   "/api/facilities/rooms",
            LabelField: "name",
            ValueField: "id",
        },
    })
}
```

### 4.2 Scope Resolution

The resolver walks a configurable chain from most specific to least specific. First match wins.

```go
// Default resolution chains per context
// The resolver picks the appropriate chain based on what scope references are available in the request.

var DefaultChains = map[string][]Scope{
    "user":   {ScopeUser, ScopeGroup, ScopeTenant, ScopeOrganization, ScopeSystem},
    "device": {ScopeDevice, ScopeRoom, ScopeTenant, ScopeOrganization, ScopeSystem},
    "room":   {ScopeRoom, ScopeTenant, ScopeOrganization, ScopeSystem},
    "tenant": {ScopeTenant, ScopeOrganization, ScopeSystem},
}
```

**Resolution algorithm:**

```
Input:  key = "attendance.auto_checkout_time"
        context = { user_id: 42, group_ids: [7, 12], tenant_id: 5 }

Step 1: Get definition → Scopes: [ScopeTenant, ScopeRoom]
Step 2: Pick chain → "user" chain: [User, Group, Tenant, Organization, System]
Step 3: Filter chain by definition's allowed scopes → [Tenant]
        (User, Group, Organization, System not in [ScopeTenant, ScopeRoom])
Step 4: Query database for first match:
        - scope_type=tenant, scope_id=5 → found "15:30" → RETURN "15:30"
Step 5: No match → return definition.Default ("16:00")

Result: { value: "15:30", inherited_from: { scope: "tenant", scope_id: 5 } }
```

**Multi-group conflict resolution:** If a user belongs to multiple groups with different values, the system uses the **first match by group sort order** (alphabetical by group name, or a configurable priority). This is a policy decision — see [Open Questions](#17-open-questions).

### 4.3 Service Layer

The new `AdvancedSettingsService` wraps the existing `ConfigService` and adds scope resolution:

```go
// services/config/settings_service.go

type SettingsService interface {
    // Schema
    GetSchema(ctx context.Context, scopeType Scope, scopeID int64) (*SettingsSchema, error)

    // Resolution (read)
    ResolveValue(ctx context.Context, key string, refs []ScopeRef) (*ResolvedValue, error)
    ResolveAll(ctx context.Context, tab string, refs []ScopeRef) ([]*ResolvedSetting, error)

    // Mutations (write)
    SetValue(ctx context.Context, key string, scopeType Scope, scopeID int64, value interface{}) error
    ResetValue(ctx context.Context, key string, scopeType Scope, scopeID int64) error

    // Actions
    ExecuteAction(ctx context.Context, actionKey string, scopeType Scope, scopeID int64) (*ActionResult, error)

    // Audit
    GetHistory(ctx context.Context, key string, scopeType Scope, scopeID int64) ([]*SettingAuditEntry, error)
}

// ScopeRef identifies a specific scope target
type ScopeRef struct {
    Type Scope
    ID   int64 // 0 for system scope
}

// ResolvedValue is what the resolver returns
type ResolvedValue struct {
    Key           string      `json:"key"`
    Value         interface{} `json:"value"`
    InheritedFrom *ScopeRef   `json:"inherited_from"` // nil if using registry default
    IsDefault     bool        `json:"is_default"`     // true if no override exists anywhere
}

// ResolvedSetting combines definition + resolved value for the schema endpoint
type ResolvedSetting struct {
    Definition                         // embedded: key, label, type, etc.
    Value         interface{}          `json:"value"`
    InheritedFrom *ScopeRef           `json:"inherited_from,omitempty"`
    IsDefault     bool                `json:"is_default"`
    IsOverridden  bool                `json:"is_overridden"` // true if this specific scope has an override
    Writable      bool                `json:"writable"`      // based on user permissions
    Visible       bool                `json:"visible"`       // based on dependency + permissions
}
```

### 4.4 Action Definitions

Actions are separate from settings — they trigger operations, not store values:

```go
// models/config/action.go

type ActionDefinition struct {
    Key         string   // unique key, e.g. "system.clear_cache"
    Label       string   // display label
    Description string   // help text
    Tab         string   // which tab to display on
    Category    string   // which category within tab
    SortOrder   int      // display order

    // Execution
    Handler     string   // registered handler name (backend dispatches to the right function)

    // Permissions & Scoping
    Permission  string   // permission required to execute
    Scopes      []Scope  // which scope levels this action appears on

    // UI
    Variant     string   // button style: "default", "danger", "outline"
    ConfirmText string   // if non-empty, show confirmation dialog before executing
    Icon        string   // optional lucide icon name
}

var actionRegistry = make(map[string]*ActionDefinition)

func RegisterAction(def ActionDefinition) {
    if _, exists := actionRegistry[def.Key]; exists {
        panic(fmt.Sprintf("duplicate action key: %s", def.Key))
    }
    actionRegistry[def.Key] = &def
}
```

**Example action registrations:**

```go
// services/config/defaults/actions.go

func init() {
    config.RegisterAction(config.ActionDefinition{
        Key:         "system.clear_cache",
        Label:       "settings.actions.clear_cache",
        Description: "settings.actions.clear_cache.description",
        Tab:         "system",
        Category:    "maintenance",
        Handler:     "clear_cache",
        Permission:  "config:manage",
        Scopes:      []config.Scope{config.ScopeSystem},
        Variant:     "danger",
        ConfirmText: "settings.actions.clear_cache.confirm",
        SortOrder:   1,
    })

    config.RegisterAction(config.ActionDefinition{
        Key:         "user.export_data",
        Label:       "settings.actions.export_data",
        Description: "settings.actions.export_data.description",
        Tab:         "privacy",
        Category:    "gdpr",
        Handler:     "export_user_data",
        Permission:  "", // any authenticated user can export their own data
        Scopes:      []config.Scope{config.ScopeUser},
        Variant:     "default",
        ConfirmText: "settings.actions.export_data.confirm",
        SortOrder:   1,
    })
}
```

---

## 5. Frontend Design

### 5.1 Schema-Driven Renderer

The frontend fetches the schema once on page load, then renders all settings dynamically:

```
GET /api/settings/schema?scope_type=tenant&scope_id=5&tab=attendance

Response: {
  tabs: [
    {
      key: "attendance",
      label: "settings.tabs.attendance",
      categories: [
        {
          key: "checkout",
          label: "settings.categories.checkout",
          items: [
            {
              key: "attendance.auto_checkout_enabled",
              type: "boolean",
              label: "settings.attendance.auto_checkout_enabled",
              description: "...",
              value: true,
              default: false,
              is_default: false,
              inherited_from: { scope: "system", scope_id: 0 },
              is_overridden: true,
              writable: true,
              visible: true,
              depends_on: null,
              validation: null,
              options: null,
              sort_order: 1
            },
            {
              key: "attendance.auto_checkout_time",
              type: "time",
              label: "settings.attendance.auto_checkout_time",
              value: "16:00",
              default: "16:00",
              is_default: true,
              inherited_from: null,
              is_overridden: false,
              writable: true,
              visible: true,
              depends_on: { key: "attendance.auto_checkout_enabled", condition: "eq", value: true },
              sort_order: 2
            }
          ],
          actions: [
            {
              key: "attendance.recalculate",
              label: "settings.actions.recalculate",
              description: "...",
              variant: "outline",
              confirm_text: "...",
              icon: "refresh-cw"
            }
          ]
        }
      ]
    }
  ]
}
```

### 5.2 Component Architecture

```
frontend/src/
├── components/
│   └── settings/
│       ├── settings-page.tsx          # Main settings page (fetches schema, renders tabs)
│       ├── settings-tab.tsx           # Renders categories within a tab
│       ├── settings-category.tsx      # Renders items within a category
│       ├── settings-item.tsx          # Single setting row (label + field + inherited badge)
│       ├── settings-action.tsx        # Action button component
│       ├── settings-scope-picker.tsx  # Scope selector (system/tenant/room/etc.)
│       └── fields/
│           ├── field-registry.ts      # Type → Component mapping
│           ├── boolean-field.tsx       # Toggle/switch
│           ├── text-field.tsx          # Text input
│           ├── number-field.tsx        # Number input with validation
│           ├── select-field.tsx        # Select (static + dynamic options)
│           ├── date-field.tsx          # Date picker
│           ├── time-field.tsx          # Time picker
│           ├── color-field.tsx         # Color picker
│           ├── password-field.tsx      # Masked input
│           ├── textarea-field.tsx      # Multi-line text
│           └── json-field.tsx          # Raw JSON editor
├── lib/
│   ├── settings-api.ts               # API client for settings endpoints
│   ├── settings-helpers.ts           # Type mappings, response transformers
│   └── settings-context.tsx          # Settings state provider (schema + values + mutations)
```

### 5.3 Field Registry (Type → Component Mapping)

```typescript
// components/settings/fields/field-registry.ts

import type { ComponentType } from "react";
import { BooleanField } from "./boolean-field";
import { TextField } from "./text-field";
import { NumberField } from "./number-field";
import { SelectField } from "./select-field";
import { DateField } from "./date-field";
import { TimeField } from "./time-field";
import { ColorField } from "./color-field";

export interface FieldProps {
  settingKey: string;
  value: unknown;
  defaultValue: unknown;
  isDefault: boolean;
  isOverridden: boolean;
  inheritedFrom: ScopeRef | null;
  writable: boolean;
  validation: ValidationRules | null;
  options: SelectOptions | null;
  onChange: (key: string, value: unknown) => void;
  onReset: (key: string) => void;
}

// Default type → component map
const fieldMap = new Map<string, ComponentType<FieldProps>>([
  ["boolean", BooleanField],
  ["text", TextField],
  ["number", NumberField],
  ["select", SelectField],
  ["date", DateField],
  ["time", TimeField],
  ["color", ColorField],
  ["password", PasswordField],
  ["textarea", TextareaField],
  ["json", JsonField],
]);

// Custom renderer overrides (escape hatch)
const customMap = new Map<string, ComponentType<FieldProps>>();

// Register a custom renderer for a specific setting key
export function registerCustomField(settingKey: string, component: ComponentType<FieldProps>) {
  customMap.set(settingKey, component);
}

// Get the component for a setting
export function getFieldComponent(settingKey: string, fieldType: string): ComponentType<FieldProps> {
  // Custom renderer takes priority
  const custom = customMap.get(settingKey);
  if (custom) return custom;

  // Fall back to type-based renderer
  const component = fieldMap.get(fieldType);
  if (!component) {
    // Unknown type → fall back to text input (safe default)
    return TextField;
  }
  return component;
}
```

### 5.4 Settings Item Component (Per Setting Row)

Each setting row shows:

```
┌─────────────────────────────────────────────────────────────┐
│ Auto-Checkout Time                    [15:30]  ↻ Reset      │
│ Automatically check out at this time   ── inherited from:   │
│                                           Tenant "OGS Köln" │
└─────────────────────────────────────────────────────────────┘
```

- **Label + description** on the left
- **Field component** on the right (determined by type)
- **Inherited-from badge** showing where the value comes from
- **Reset button** (only visible when this scope has an override)
- **Lock icon** when the user doesn't have write permission
- **Hidden entirely** when the user doesn't have read permission or dependency not met

### 5.5 Dependency Handling (Frontend)

When the schema arrives, the frontend builds a dependency graph:

```typescript
// In settings-context.tsx
function evaluateDependency(dep: Dependency, currentValues: Map<string, unknown>): boolean {
  const parentValue = currentValues.get(dep.key);
  switch (dep.condition) {
    case "eq":       return parentValue === dep.value;
    case "neq":      return parentValue !== dep.value;
    case "not_empty": return parentValue != null && parentValue !== "";
    // ... etc.
  }
}
```

When a parent value changes, all dependent settings re-evaluate visibility in real-time (no API call needed — it's pure frontend state).

### 5.6 Integration with Existing Settings Page

The current settings page (`/[tenant]/(protected)/settings/page.tsx`) uses `SettingsLayout` with Profile and Security tabs. The new system integrates as additional tabs:

```
Settings Page
├── Profile tab          (existing — user profile form, stays as-is)
├── Security tab         (existing — password change, stays as-is)
├── Attendance tab       (NEW — from schema, rendered dynamically)
├── Display tab          (NEW — from schema, rendered dynamically)
├── System tab           (NEW — from schema, rendered dynamically, admin only)
└── ...                  (any future tabs appear automatically from registry)
```

The existing Profile and Security tabs remain hand-coded components. New settings tabs are injected via the `extraTabs` prop of `SettingsLayout`.

---

## 6. Scope System (Deep Dive)

### 6.1 What Is a Scope?

A scope is **a level at which a setting can have a value**. Each scope level maps to an existing database entity:

| Scope | Entity Table | Example |
|-------|-------------|---------|
| `system` | — (singleton, `scope_id = NULL`) | System-wide defaults |
| `organization` | `platform.organizations` | Träger-level policy |
| `tenant` | `platform.schools` (school_id = tenant_id) | Per-school configuration |
| `user` | `auth.accounts` | Per-user preferences |
| `device` | `iot.devices` | Per-device behavior |
| `group` | `education.groups` | Per-group rules |
| `room` | `facilities.rooms` | Per-room configuration |

### 6.2 Not Every Setting Exists at Every Scope

The `Scopes` field in the definition controls which levels are valid:

```go
// "Auto-checkout" is a policy decision → only system and tenant
Scopes: []Scope{ScopeSystem, ScopeTenant}

// "UI theme" is a user preference → user and tenant (for default)
Scopes: []Scope{ScopeUser, ScopeTenant, ScopeSystem}

// "Screen timeout" is device-specific → device, room, tenant
Scopes: []Scope{ScopeDevice, ScopeRoom, ScopeTenant, ScopeSystem}
```

### 6.3 Cascade Resolution

When resolving a value, the system walks from the most specific scope to the least specific:

```
Example: What is "display.screen_timeout" for Device 99 in Room 7, Tenant 5?

Definition allows: [Device, Room, Tenant, System]

Resolution:
  1. Check config.settings WHERE key="display.screen_timeout" AND scope_type="device" AND scope_id=99
     → not found
  2. Check config.settings WHERE key="display.screen_timeout" AND scope_type="room" AND scope_id=7
     → found: 600
     → RETURN { value: 600, inherited_from: { scope: "room", scope_id: 7 } }

If Room 7 also had no override:
  3. Check scope_type="tenant" AND scope_id=5 → found: 300 → return
  4. Check scope_type="system" AND scope_id=NULL → found: 120 → return
  5. Nothing in DB → return registry default
```

**Key insight:** Most settings rows will never be created. Only explicit overrides exist in the database. Everything else cascades from parents or uses the registry default.

### 6.4 Adding a New Scope Later

Adding a scope requires:

| Step | Work | Files Touched |
|------|------|---------------|
| 1. Add constant | `ScopeBuilding Scope = "building"` | `models/config/registry.go` |
| 2. Add to chain | Insert in DefaultChains where appropriate | `services/config/resolver.go` |
| 3. Add resolver | Function to extract building ID from context/request | `services/config/scope_resolvers.go` |
| **No migration** | `scope_type` is `varchar`, not `ENUM` | — |
| **No frontend changes** | Schema endpoint already returns allowed scopes | — |

Total: ~10 lines of Go code, zero migrations, zero frontend changes.

---

## 7. Field Types & Renderers

### 7.1 Built-in Types

| Type | Frontend Component | Go Value Type | Example Default |
|------|--------------------|---------------|-----------------|
| `boolean` | Toggle/Switch | `bool` | `false` |
| `text` | Input | `string` | `""` |
| `number` | Number Input (with stepper) | `float64` | `0` |
| `select` | Select dropdown | `string` or `int64` | `nil` |
| `date` | Date Picker (react-day-picker) | `string` (ISO 8601) | `nil` |
| `time` | Time Picker | `string` ("HH:MM") | `"00:00"` |
| `color` | Color Picker | `string` (hex) | `"#000000"` |
| `password` | Masked Input | `string` | `""` |
| `textarea` | Textarea | `string` | `""` |
| `json` | JSON editor (monospace) | `json.RawMessage` | `{}` |

### 7.2 Adding a New Field Type

**Backend:**
1. Add constant to `FieldType` in `models/config/registry.go`
2. Add serialization/validation logic if needed

**Frontend:**
1. Create `components/settings/fields/new-field.tsx` implementing `FieldProps`
2. Add entry to `fieldMap` in `field-registry.ts`

No existing code changes — purely additive.

### 7.3 Custom Renderers (Escape Hatch)

For settings that need completely custom UI (e.g., a visual room layout editor):

```typescript
// Somewhere in the app initialization
import { registerCustomField } from "~/components/settings/fields/field-registry";
import { RoomLayoutEditor } from "~/components/rooms/room-layout-editor";

registerCustomField("facilities.room_layout", RoomLayoutEditor);
```

The `RoomLayoutEditor` receives the same `FieldProps` interface and can render whatever it wants. The settings system just calls `onChange(key, value)` when it produces a new value.

---

## 8. Permission Model

### 8.1 Per-Setting Permissions

Each setting definition specifies:

```go
ReadPermission:  "config:read"     // who can SEE the setting
WritePermission: "config:update"   // who can CHANGE the setting
```

The schema endpoint checks the requesting user's permissions and:
- **Omits settings entirely** if the user lacks `ReadPermission`
- **Sets `writable: false`** if the user lacks `WritePermission` (setting is visible but read-only)

### 8.2 Permission Granularity

Settings can use existing permissions or define new ones:

```go
// Reuse existing config permissions
ReadPermission: permissions.ConfigRead   // "config:read"
WritePermission: permissions.ConfigUpdate // "config:update"

// Or use domain-specific permissions
ReadPermission: permissions.AttendanceRead    // "attendance:read"
WritePermission: permissions.AttendanceUpdate // "attendance:update"

// Or no permission required (visible to all authenticated users)
ReadPermission: ""
WritePermission: ""
```

### 8.3 Scope-Based Permission Filtering

The schema endpoint also filters based on scope:
- **System scope** → only platform operators see system-level settings
- **Tenant scope** → only users with tenant admin permissions
- **User scope** → users see their own settings, admins see any user's settings

This is handled in the service layer, combining JWT claims (`scope`, `tenant_id`) with the setting's `ReadPermission`.

---

## 9. Dependencies & Conditional Visibility

### 9.1 Dependency Definition

```go
DependsOn: &Dependency{
    Key:       "attendance.auto_checkout_enabled",  // parent setting key
    Condition: "eq",                                 // comparison operator
    Value:     true,                                 // expected value
}
```

### 9.2 Supported Conditions

| Condition | Meaning | Example |
|-----------|---------|---------|
| `eq` | Parent value equals | `{condition: "eq", value: true}` |
| `neq` | Parent value does not equal | `{condition: "neq", value: "disabled"}` |
| `gt` | Parent value greater than | `{condition: "gt", value: 0}` |
| `lt` | Parent value less than | `{condition: "lt", value: 100}` |
| `in` | Parent value in list | `{condition: "in", value: ["a", "b"]}` |
| `not_empty` | Parent has a non-null/non-empty value | `{condition: "not_empty", value: nil}` |

### 9.3 Evaluation

- **Backend (schema endpoint):** Dependencies are included in the response as metadata. The backend does NOT hide dependent settings — it sends them with `visible: true/false` based on current parent values. This allows the frontend to toggle visibility instantly when a parent changes.
- **Frontend (real-time):** When a parent value changes, all dependent settings re-evaluate their `visible` state client-side. No API round-trip needed.
- **Nested dependencies** are supported: setting C depends on B, which depends on A. The frontend evaluates the full chain.

---

## 10. Action Buttons

### 10.1 Backend Handlers

Actions are registered in the same way as settings, but instead of storing values, they execute functions:

```go
// services/config/action_handlers.go

type ActionHandler func(ctx context.Context, scopeType Scope, scopeID int64) (*ActionResult, error)

type ActionResult struct {
    Success bool        `json:"success"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"` // optional payload (e.g., download URL)
}

var actionHandlers = make(map[string]ActionHandler)

func RegisterActionHandler(key string, handler ActionHandler) {
    actionHandlers[key] = handler
}

// Example handlers
func init() {
    RegisterActionHandler("clear_cache", func(ctx context.Context, _ Scope, _ int64) (*ActionResult, error) {
        // Clear application cache
        cache.FlushAll()
        return &ActionResult{Success: true, Message: "Cache cleared successfully"}, nil
    })

    RegisterActionHandler("export_user_data", func(ctx context.Context, _ Scope, scopeID int64) (*ActionResult, error) {
        // Generate GDPR data export for user
        url, err := gdpr.GenerateExport(ctx, scopeID)
        if err != nil {
            return nil, err
        }
        return &ActionResult{
            Success: true,
            Message: "Export ready for download",
            Data:    map[string]string{"download_url": url},
        }, nil
    })
}
```

### 10.2 Frontend Rendering

Actions appear alongside settings in their category. The schema response includes them:

```json
{
  "categories": [{
    "key": "maintenance",
    "items": [],
    "actions": [
      {
        "key": "system.clear_cache",
        "label": "Clear Cache",
        "variant": "danger",
        "confirm_text": "Are you sure?",
        "icon": "trash-2"
      }
    ]
  }]
}
```

The frontend renders a button that:
1. Shows a confirmation dialog if `confirm_text` is set
2. Calls `POST /api/settings/actions/{key}` with scope context
3. Shows success/error toast from the response

### 10.3 Frontend-Only Actions

Some actions are purely client-side (e.g., clear browser cache, reset local storage). These are registered in the frontend field registry:

```typescript
// Frontend-only actions don't have a backend handler
// They're registered in the settings context
const clientActions: Record<string, () => Promise<ActionResult>> = {
  "client.clear_local_storage": async () => {
    localStorage.clear();
    return { success: true, message: "Local storage cleared" };
  },
};
```

These are defined with `Handler: "client"` in the backend registry, signaling the frontend to handle them locally.

---

## 11. Dynamic Select Options

### 11.1 Static Options

Defined directly in the registry:

```go
Options: &config.SelectOptions{
    Static: []config.SelectOption{
        {Label: "Deutsch", Value: "de"},
        {Label: "English", Value: "en"},
        {Label: "Türkçe", Value: "tr"},
    },
}
```

Frontend receives these in the schema and renders a normal `<select>`.

### 11.2 Dynamic Options (API-Backed)

Defined with an endpoint reference:

```go
Options: &config.SelectOptions{
    Endpoint:   "/api/facilities/rooms",
    LabelField: "name",
    ValueField: "id",
}
```

Frontend behavior:
1. Schema arrives with `options.endpoint` set
2. `SelectField` component fetches the endpoint on mount (with tenant context)
3. Maps response using `label_field` and `value_field`
4. Renders as a searchable select dropdown
5. Caches the result for the session (SWR/stale-while-revalidate)

**Why not embed the options in the schema response?** Because:
- Options can be large (hundreds of rooms/groups)
- Options are tenant-scoped and change independently of settings
- The frontend can cache and refresh them separately
- Existing API endpoints already handle pagination, search, and filtering

### 11.3 Filtered Dynamic Options

Some selects need filtered data. The endpoint supports query params:

```go
Options: &config.SelectOptions{
    Endpoint:   "/api/education/groups?type=afternoon",
    LabelField: "name",
    ValueField: "id",
}
```

The frontend appends these params when fetching.

---

## 12. Categorization (Tabs & Categories)

### 12.1 Structure

```
Settings Page
├── Tab: "Allgemein" (General)
│   ├── Category: "Darstellung" (Display)
│   │   ├── Setting: ui.theme
│   │   ├── Setting: ui.language
│   │   └── Setting: ui.items_per_page
│   └── Category: "Benachrichtigungen" (Notifications)
│       ├── Setting: notifications.email_enabled
│       └── Setting: notifications.push_enabled
├── Tab: "Anwesenheit" (Attendance)
│   ├── Category: "Check-out"
│   │   ├── Setting: attendance.auto_checkout_enabled
│   │   └── Setting: attendance.auto_checkout_time (depends on above)
│   └── Category: "Allgemein"
│       └── Setting: attendance.default_room
├── Tab: "System"
│   └── Category: "Wartung" (Maintenance)
│       ├── Setting: system.session_timeout
│       └── Action: system.clear_cache
└── Tab: "Datenschutz" (Privacy / GDPR)
    └── Category: "Datenexport"
        └── Action: user.export_data
```

### 12.2 Tab & Category Ordering

Tabs and categories are ordered by convention:

```go
// Tab ordering (defined once, centrally)
var TabOrder = []string{
    "general",
    "attendance",
    "education",
    "facilities",
    "iot",
    "system",
    "privacy",
}
```

Settings within a category are ordered by `SortOrder`. Categories within a tab are ordered by the lowest `SortOrder` of their contained settings.

### 12.3 Permission-Based Tab Visibility

Tabs that contain zero visible settings (after permission filtering) are hidden entirely. The backend handles this — the schema response only includes tabs/categories that the user can actually see.

---

## 13. Database Schema

### 13.1 New Table: `config.setting_values`

The existing `config.settings` table stores flat key-value pairs. The new table adds scope support:

```sql
CREATE TABLE config.setting_values (
    id            BIGSERIAL    PRIMARY KEY,
    tenant_id     BIGINT       REFERENCES platform.schools(id),  -- NULL for system scope
    setting_key   VARCHAR(255) NOT NULL,                         -- references registry key
    scope_type    VARCHAR(50)  NOT NULL,                         -- "system", "tenant", "user", etc.
    scope_id      BIGINT,                                        -- NULL for system scope
    value         JSONB        NOT NULL,                         -- the actual value, JSON-encoded
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by    BIGINT       REFERENCES auth.accounts(id),    -- who last changed this

    -- Each setting can only have one value per scope target
    CONSTRAINT uq_setting_scope UNIQUE (tenant_id, setting_key, scope_type, scope_id)
);

-- Indexes for resolution queries
CREATE INDEX idx_setting_values_key_scope
    ON config.setting_values (setting_key, scope_type, scope_id);

CREATE INDEX idx_setting_values_tenant
    ON config.setting_values (tenant_id);

-- RLS policy (tenant isolation)
ALTER TABLE config.setting_values ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON config.setting_values
    USING (tenant_id = current_setting('app.current_tenant_id')::BIGINT OR tenant_id IS NULL);

-- Auto-update timestamp trigger
CREATE TRIGGER setting_values_updated_at
    BEFORE UPDATE ON config.setting_values
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### 13.2 Audit Table: `config.setting_audit`

```sql
CREATE TABLE config.setting_audit (
    id            BIGSERIAL    PRIMARY KEY,
    tenant_id     BIGINT       REFERENCES platform.schools(id),
    setting_key   VARCHAR(255) NOT NULL,
    scope_type    VARCHAR(50)  NOT NULL,
    scope_id      BIGINT,
    old_value     JSONB,                                         -- NULL for first set
    new_value     JSONB,                                         -- NULL for reset/delete
    action        VARCHAR(20)  NOT NULL,                         -- "set", "reset", "delete"
    changed_by    BIGINT       REFERENCES auth.accounts(id),
    changed_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- No unique constraint — this is append-only
    CONSTRAINT chk_action CHECK (action IN ('set', 'reset', 'delete'))
);

CREATE INDEX idx_setting_audit_key ON config.setting_audit (setting_key, scope_type, scope_id);
CREATE INDEX idx_setting_audit_tenant ON config.setting_audit (tenant_id);

ALTER TABLE config.setting_audit ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON config.setting_audit
    USING (tenant_id = current_setting('app.current_tenant_id')::BIGINT OR tenant_id IS NULL);
```

### 13.3 Migration Strategy

The existing `config.settings` table (migration `001006001`) stays as-is during the transition. The new migration:

1. Creates `config.setting_values` and `config.setting_audit`
2. Migrates existing `config.settings` rows to `config.setting_values` with `scope_type = 'tenant'`
3. Does NOT drop `config.settings` yet (backward compatibility during rollout)
4. A later cleanup migration drops `config.settings` once all code uses the new service

---

## 14. API Endpoints

### 14.1 Schema Endpoint

```
GET /api/settings/schema
    ?scope_type=tenant           (required: which scope level to show settings for)
    &scope_id=5                  (required for non-system scopes)
    &tab=attendance              (optional: filter to specific tab)

Response: SettingsSchema (tabs → categories → items + actions)
Permissions: authenticated + per-setting ReadPermission filtering
```

### 14.2 Value Endpoints

```
GET /api/settings/resolve/{key}
    ?scope_type=user&scope_id=42    (the target to resolve for)
    &context_tenant_id=5            (implicit from JWT usually)

Response: { key, value, inherited_from, is_default }
Permissions: ReadPermission of the setting

---

PUT /api/settings/values/{key}
    Body: { scope_type: "room", scope_id: 7, value: "15:30" }

Response: { key, value, scope_type, scope_id }
Permissions: WritePermission of the setting
Side effect: creates audit entry

---

DELETE /api/settings/values/{key}
    ?scope_type=room&scope_id=7

Response: 204 No Content (override removed, falls back to parent)
Permissions: WritePermission of the setting
Side effect: creates audit entry with action="reset"
```

### 14.3 Bulk Endpoints

```
GET /api/settings/values
    ?scope_type=tenant&scope_id=5
    &keys=attendance.auto_checkout_enabled,attendance.auto_checkout_time

Response: [ { key, value, inherited_from, is_default }, ... ]

---

PUT /api/settings/values/bulk
    Body: { scope_type: "tenant", scope_id: 5, values: { "key1": "val1", "key2": "val2" } }

Response: [ { key, value }, ... ]
```

### 14.4 Action Endpoint

```
POST /api/settings/actions/{key}
    Body: { scope_type: "system", scope_id: 0 }

Response: { success: true, message: "Cache cleared", data: null }
Permissions: ActionDefinition.Permission
```

### 14.5 Audit Endpoint

```
GET /api/settings/audit/{key}
    ?scope_type=tenant&scope_id=5
    &limit=50&offset=0

Response: [ { setting_key, old_value, new_value, action, changed_by, changed_at }, ... ]
Permissions: config:manage
```

---

## 15. Adding a New Setting (Developer Guide)

This is the main developer experience goal: adding a setting should be a single file change.

### Step 1: Create or Edit a Registration File

```go
// services/config/defaults/my_domain.go

package defaults

import "github.com/moto-nrw/project-phoenix/models/config"

func init() {
    config.Register(config.Definition{
        Key:             "my_domain.my_setting",
        Label:           "settings.my_domain.my_setting",
        Description:     "settings.my_domain.my_setting.description",
        Type:            config.FieldBoolean,
        Default:         true,
        Scopes:          []config.Scope{config.ScopeTenant},
        ReadPermission:  "config:read",
        WritePermission: "config:update",
        Tab:             "my_domain",
        Category:        "general",
        SortOrder:       1,
    })
}
```

### Step 2: Import the Registration File

```go
// services/config/defaults/defaults.go

package defaults

// Blank imports trigger init() functions
import (
    _ "github.com/moto-nrw/project-phoenix/services/config/defaults/attendance"
    _ "github.com/moto-nrw/project-phoenix/services/config/defaults/my_domain"  // ← add this
)
```

### Step 3: There Is No Step 3

- No migration needed (the value is only stored when someone overrides the default)
- No frontend code needed (the schema endpoint returns the new setting, the renderer handles it)
- No API code needed (generic endpoints handle all settings)
- No test needed for the setting itself (the registry validates at startup; the resolver is tested generically)

### When You DO Need Extra Work

| Scenario | Extra Work |
|----------|-----------|
| New field type (e.g., "slider") | Add Go constant + one React component + one `fieldMap` entry |
| New scope level (e.g., "building") | Add Go constant + chain entry + scope resolver (~10 lines) |
| Custom UI for one specific setting | `registerCustomField("key", Component)` in frontend |
| New action | Register action definition + implement handler function |
| New tab | Just use a new `Tab` value in the definition — tabs appear automatically |

---

## 16. Implementation Phases

### Phase 1: Foundation (Backend Core)

**Goal:** Registry, database, resolver, basic CRUD API

| Task | Description | Files |
|------|-------------|-------|
| 1.1 | Define `FieldType`, `Scope`, `Definition`, `Dependency`, `SelectOptions` structs | `models/config/registry.go` |
| 1.2 | Implement registry singleton (`Register`, `RegisterAction`, `GetDefinition`, `AllDefinitions`) | `models/config/registry.go` |
| 1.3 | Define `ActionDefinition`, `ActionResult`, action registry | `models/config/action.go` |
| 1.4 | Create migration for `config.setting_values` + `config.setting_audit` | `database/migrations/001006002_config_setting_values.go` |
| 1.5 | Implement `SettingValueRepository` (CRUD on `config.setting_values`) | `database/repositories/config/setting_value.go` |
| 1.6 | Implement `SettingAuditRepository` (append-only writes, list reads) | `database/repositories/config/setting_audit.go` |
| 1.7 | Implement scope resolution service (`ResolveValue`, `ResolveAll`) | `services/config/settings_service.go` |
| 1.8 | Implement schema builder (permissions filtering, dependency evaluation) | `services/config/schema_builder.go` |
| 1.9 | Implement action dispatcher | `services/config/action_handlers.go` |
| 1.10 | Wire into repository factory + service factory | `database/repositories/factory.go`, `services/factory.go` |
| 1.11 | Add new permission constants if needed | `auth/authorize/permissions/constants.go` |
| 1.12 | Write tests for registry, resolver, schema builder | `*_test.go` files |

**Deliverable:** Backend can register settings, resolve values through scope chain, and serve schema.

### Phase 2: API Layer (Backend Endpoints)

**Goal:** REST endpoints for schema, values, actions, audit

| Task | Description | Files |
|------|-------------|-------|
| 2.1 | Implement schema endpoint (`GET /settings/schema`) | `api/config/settings_api.go` |
| 2.2 | Implement value endpoints (`GET/PUT/DELETE /settings/values/{key}`) | `api/config/settings_api.go` |
| 2.3 | Implement bulk value endpoints | `api/config/settings_api.go` |
| 2.4 | Implement action endpoint (`POST /settings/actions/{key}`) | `api/config/settings_api.go` |
| 2.5 | Implement audit endpoint (`GET /settings/audit/{key}`) | `api/config/settings_api.go` |
| 2.6 | Mount routes in API router | `api/base.go` |
| 2.7 | Write API integration tests | `api/config/settings_test.go` |

**Deliverable:** Full REST API, testable via curl/Postman.

### Phase 3: Initial Settings Registration

**Goal:** Register real settings for existing features

| Task | Description | Files |
|------|-------------|-------|
| 3.1 | Register attendance settings (auto-checkout, timeout, etc.) | `services/config/defaults/attendance.go` |
| 3.2 | Register display/UI settings (theme, language, items per page) | `services/config/defaults/display.go` |
| 3.3 | Register system settings (session timeout, cleanup intervals) | `services/config/defaults/system.go` |
| 3.4 | Register IoT settings (scan interval, device timeout) | `services/config/defaults/iot.go` |
| 3.5 | Register common actions (clear cache, export data) | `services/config/defaults/actions.go` |
| 3.6 | Create blank import file to trigger all registrations | `services/config/defaults/defaults.go` |
| 3.7 | Data migration: copy existing `config.settings` to `config.setting_values` | `database/migrations/001006003_migrate_settings_data.go` |

**Deliverable:** Existing settings available through new system, backward compatible.

### Phase 4: Frontend Core

**Goal:** Schema-driven settings renderer

| Task | Description | Files |
|------|-------------|-------|
| 4.1 | Create settings API client | `lib/settings-api.ts` |
| 4.2 | Create settings helpers (type mapping, response transformation) | `lib/settings-helpers.ts` |
| 4.3 | Create settings context provider | `lib/settings-context.tsx` |
| 4.4 | Create field registry + type→component map | `components/settings/fields/field-registry.ts` |
| 4.5 | Create field components: boolean, text, number, select | `components/settings/fields/*.tsx` |
| 4.6 | Create field components: date, time, color | `components/settings/fields/*.tsx` |
| 4.7 | Create `settings-item.tsx` (single setting row with label, field, inherited badge) | `components/settings/settings-item.tsx` |
| 4.8 | Create `settings-category.tsx` (group of items + actions) | `components/settings/settings-category.tsx` |
| 4.9 | Create `settings-tab.tsx` (group of categories) | `components/settings/settings-tab.tsx` |
| 4.10 | Create `settings-action.tsx` (action button with confirmation) | `components/settings/settings-action.tsx` |
| 4.11 | Create `settings-page.tsx` (main page, fetches schema, renders tabs) | `components/settings/settings-page.tsx` |

**Deliverable:** Functional settings UI that renders from backend schema.

### Phase 5: Frontend Integration

**Goal:** Wire into existing app, polish UX

| Task | Description | Files |
|------|-------------|-------|
| 5.1 | Integrate with existing settings page via `extraTabs` on `SettingsLayout` | `app/[tenant]/(protected)/settings/page.tsx` |
| 5.2 | Add scope picker component (for admin: "viewing settings for: Tenant / Room X / etc.") | `components/settings/settings-scope-picker.tsx` |
| 5.3 | Implement "inherited from" badges and "reset to default" button | `components/settings/settings-item.tsx` |
| 5.4 | Implement dependency-based conditional visibility (real-time) | `lib/settings-context.tsx` |
| 5.5 | Implement dynamic select options (fetch from API endpoint) | `components/settings/fields/select-field.tsx` |
| 5.6 | Add loading states, error handling, optimistic updates | Various |
| 5.7 | Responsive design (mobile tab navigation matches existing SettingsLayout pattern) | Various |
| 5.8 | Run `pnpm run check` — zero warnings | — |

**Deliverable:** Production-ready settings UI integrated into the app.

### Phase 6: Hardening & Migration

**Goal:** Audit trail, existing code migration, cleanup

| Task | Description | Files |
|------|-------------|-------|
| 6.1 | Implement audit trail UI (setting history viewer) | `components/settings/settings-audit.tsx` |
| 6.2 | Migrate existing code that reads `config.settings` directly to use the new resolver | Various service files |
| 6.3 | Add compatibility wrapper for `ConfigService.GetStringValue()` etc. | `services/config/compat.go` |
| 6.4 | Add settings search functionality | Frontend + schema endpoint |
| 6.5 | Cleanup migration: drop old `config.settings` table (after full rollout) | `database/migrations/001006004_drop_legacy_settings.go` |
| 6.6 | End-to-end integration tests | `api/config/settings_integration_test.go` |

**Deliverable:** Clean, fully migrated system with audit trail.

---

## 17. Open Questions

These need team input before implementation begins:

| # | Question | Options | Recommendation |
|---|----------|---------|----------------|
| Q1 | **Multi-group conflict**: If a user is in groups A and B with different values for the same setting, which wins? | (a) Alphabetical by group name (b) Explicit priority field on groups (c) Most recently updated value (d) Don't allow group scope — use tenant only | **(b)** — add a `priority` field to groups, highest priority wins. If equal, alphabetical. |
| Q2 | **Scope hierarchy**: Should organization always sit between tenant and system? | (a) Yes, fixed: User → Group → Room → Tenant → Org → System (b) Configurable per setting | **(a)** — fixed hierarchy is simpler and sufficient. Per-setting override via `ResolutionChain` as escape hatch. |
| Q3 | **Audit retention**: How long to keep audit entries? | (a) Forever (b) 1 year (c) Configurable per tenant (d) Same as data retention settings | **(c)** — make it a setting itself ("audit.retention_days"), which dogfoods the system. |
| Q4 | **Existing settings migration**: Migrate existing `config.settings` data in-place or start fresh? | (a) Migrate existing rows to new table (b) Start fresh, re-seed defaults (c) Keep old table, use new table only for new settings | **(a)** — migrate data to preserve any tenant-specific overrides already set. |
| Q5 | **System scope + RLS**: System-scoped settings have `tenant_id = NULL`. How does RLS handle this? | (a) Separate RLS policy for NULL tenant_id (b) Use tenant_id = 0 as sentinel for system scope (c) System settings in a separate table | **(a)** — add `OR tenant_id IS NULL` to the RLS policy. System settings are readable by all tenants. |
| Q6 | **i18n strategy**: How to handle setting labels and descriptions? | (a) Plain German strings in registry (b) i18n keys that map to translation files (c) Both: German as default, keys for future translation | **(c)** — use i18n keys like `"settings.attendance.auto_checkout"` with German fallback. Ready for future translation without blocking v1. |
| Q7 | **Who sees system-scope settings?** | (a) Only platform operators (b) Tenant admins can see (read-only), operators can edit (c) Configurable per setting | **(b)** — tenant admins should understand system defaults affecting their school, but only operators can change them. |

---

## Appendix A: File Tree (All New Files)

```
backend/
├── models/config/
│   ├── registry.go              (NEW — Definition, FieldType, Scope, Register())
│   ├── action.go                (NEW — ActionDefinition, RegisterAction())
│   ├── setting_value.go         (NEW — SettingValue model for DB)
│   ├── setting_audit.go         (NEW — SettingAuditEntry model for DB)
│   └── setting_value_repository.go (NEW — repository interface)
├── database/
│   ├── repositories/config/
│   │   ├── setting_value.go     (NEW — SettingValueRepository impl)
│   │   └── setting_audit.go     (NEW — SettingAuditRepository impl)
│   └── migrations/
│       ├── 001006002_config_setting_values.go  (NEW — create tables)
│       └── 001006003_migrate_settings_data.go  (NEW — data migration)
├── services/config/
│   ├── settings_service.go      (NEW — SettingsService with resolver)
│   ├── schema_builder.go        (NEW — builds schema response)
│   ├── action_handlers.go       (NEW — action dispatch + handlers)
│   ├── compat.go                (NEW — backward compat wrapper)
│   └── defaults/
│       ├── defaults.go          (NEW — blank imports for init())
│       ├── attendance.go        (NEW — attendance settings)
│       ├── display.go           (NEW — display/UI settings)
│       ├── system.go            (NEW — system settings)
│       ├── iot.go               (NEW — IoT settings)
│       └── actions.go           (NEW — action registrations)
└── api/config/
    └── settings_api.go          (NEW — schema, values, actions, audit endpoints)

frontend/src/
├── lib/
│   ├── settings-api.ts          (NEW — API client)
│   ├── settings-helpers.ts      (NEW — type mapping)
│   └── settings-context.tsx     (NEW — state provider)
└── components/settings/
    ├── settings-page.tsx         (NEW — main orchestrator)
    ├── settings-tab.tsx          (NEW — tab content)
    ├── settings-category.tsx     (NEW — category group)
    ├── settings-item.tsx         (NEW — single setting row)
    ├── settings-action.tsx       (NEW — action button)
    ├── settings-scope-picker.tsx (NEW — scope selector)
    ├── settings-audit.tsx        (NEW — audit history viewer)
    └── fields/
        ├── field-registry.ts     (NEW — type→component map)
        ├── boolean-field.tsx     (NEW)
        ├── text-field.tsx        (NEW)
        ├── number-field.tsx      (NEW)
        ├── select-field.tsx      (NEW)
        ├── date-field.tsx        (NEW)
        ├── time-field.tsx        (NEW)
        ├── color-field.tsx       (NEW)
        ├── password-field.tsx    (NEW)
        ├── textarea-field.tsx    (NEW)
        └── json-field.tsx        (NEW)
```

## Appendix B: Modified Existing Files

| File | Change |
|------|--------|
| `backend/database/repositories/factory.go` | Add `SettingValue` and `SettingAudit` repository fields |
| `backend/services/factory.go` | Add `Settings` service field (the new `SettingsService`) |
| `backend/api/base.go` | Add `Settings` resource field, mount `/settings` routes |
| `backend/auth/authorize/permissions/constants.go` | Add new permission constants if needed |
| `frontend/src/app/[tenant]/(protected)/settings/page.tsx` | Add dynamic tabs via `extraTabs` |
| `frontend/src/components/shared/settings-layout.tsx` | Minor: ensure `extraTabs` supports async loading |
