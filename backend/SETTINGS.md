# Settings System Documentation

This document explains how to add settings and actions to Project Phoenix.

## Architecture Overview

The settings system consists of:

1. **Backend**: Go-based definition registry, service layer, and API
2. **Frontend**: React components for rendering settings and actions
3. **Database**: PostgreSQL tables for tabs, definitions, and values

```
Backend                          Frontend
┌─────────────────────────┐      ┌─────────────────────────┐
│ settings/definitions/   │      │ components/settings/    │
│   ├── tabs.go           │      │   ├── setting-control   │
│   ├── display.go        │      │   ├── action-control    │
│   ├── email.go          │      │   └── settings-category │
│   ├── rooms.go          │      └─────────────────────────┘
│   ├── system.go         │
│   └── actions.go        │
└─────────────────────────┘
          ↓
┌─────────────────────────┐
│ services/config/        │
│   ├── hierarchical_     │
│   │   settings_service  │
│   ├── action_handlers   │
│   └── builtin_action_   │
│       handlers          │
└─────────────────────────┘
          ↓
┌─────────────────────────┐
│ api/settings/           │
│   ├── handlers.go       │
│   └── actions.go        │
└─────────────────────────┘
```

## Adding a New Tab

Tabs organize settings in the UI. Add to `settings/definitions/tabs.go`:

```go
import "github.com/moto-nrw/project-phoenix/settings"

func init() {
    settings.MustRegisterTab(settings.TabDefinition{
        Key:                "mytab",           // Unique identifier
        Name:               "My Tab",          // Display name
        Icon:               "cog",             // Lucide icon name
        DisplayOrder:       50,                // Sort order (lower = first)
        RequiredPermission: "config:manage",   // Empty = all authenticated users
    })
}
```

## Adding a Setting

Settings are registered in definition files. Add to appropriate file in `settings/definitions/`:

```go
import (
    "github.com/moto-nrw/project-phoenix/models/config"
    "github.com/moto-nrw/project-phoenix/settings"
)

func init() {
    settings.MustRegister(settings.Definition{
        // Required fields
        Key:         "category.setting_name",        // Unique key (format: category.name)
        Type:        config.ValueTypeString,         // See Value Types below
        Default:     "default_value",                // Default value as string
        Tab:         "mytab",                        // Tab key from tabs.go
        Category:    "subcategory",                  // Groups settings within tab
        Label:       "Setting Label",                // UI label
        Description: "What this setting does",       // Help text

        // Scopes (where the setting can be overridden)
        Scopes: []config.Scope{
            config.ScopeSystem,  // System-wide default
            config.ScopeUser,    // Per-user override
            config.ScopeDevice,  // Per-device override
        },

        // Permissions
        ViewPerm: "config:read",   // Permission to see (empty = all)
        EditPerm: "config:manage", // Permission to edit

        // Optional fields
        DisplayOrder:    10,                         // Sort within category
        Validation:      settings.IntRange(1, 100), // Validation rules
        RequiresRestart: true,                       // Show restart warning
        IsSensitive:     true,                       // Mask value in UI
    })
}
```

### Value Types

| Type | Go Constant | Description | Example Default |
|------|-------------|-------------|-----------------|
| String | `config.ValueTypeString` | Text input | `"hello"` |
| Int | `config.ValueTypeInt` | Integer input | `"42"` |
| Float | `config.ValueTypeFloat` | Decimal input | `"3.14"` |
| Bool | `config.ValueTypeBool` | Toggle switch | `"true"` or `"false"` |
| Enum | `config.ValueTypeEnum` | Dropdown select | `"option1"` |
| Time | `config.ValueTypeTime` | Time picker | `"14:30"` |
| Duration | `config.ValueTypeDuration` | Duration input | `"1h30m"` |
| ObjectRef | `config.ValueTypeObjectRef` | Database object reference | `""` (empty = none) |
| JSON | `config.ValueTypeJson` | JSON editor | `"{}"` |
| Action | `config.ValueTypeAction` | Executable action | N/A |

### Enum Settings

For enum types, provide options:

```go
settings.MustRegister(settings.Definition{
    Key:     "display.theme",
    Type:    config.ValueTypeEnum,
    Default: "system",
    // ... other fields ...
    EnumOptions: []settings.EnumOption{
        {Value: "light", Label: "Light Mode"},
        {Value: "dark", Label: "Dark Mode"},
        {Value: "system", Label: "System Default"},
    },
})
```

### Object Reference Settings

For referencing database objects:

```go
settings.MustRegister(settings.Definition{
    Key:           "checkin.default_room",
    Type:          config.ValueTypeObjectRef,
    Default:       "",
    // ... other fields ...
    ObjectRefType: "room",                     // Object type
    ObjectRefFilter: map[string]interface{}{   // Optional filter
        "is_active": true,
    },
})
```

### Validation Rules

```go
// Integer range
Validation: settings.IntRange(1, 100)

// Float range
Validation: settings.FloatRange(0.0, 1.0)

// String length
Validation: settings.StringLength(1, 100)    // min, max
Validation: settings.StringMaxLength(255)    // max only

// Custom validation (implement ValidateFunc)
Validation: settings.Validation{
    Min: ptr(0),
    Max: ptr(100),
}
```

## Adding an Action

Actions are executable operations (e.g., "Clear Cache", "Test Email").

### Step 1: Register the Action Definition

In `settings/definitions/actions.go`:

```go
import (
    "github.com/moto-nrw/project-phoenix/models/config"
    "github.com/moto-nrw/project-phoenix/settings"
)

func init() {
    settings.MustRegister(settings.Definition{
        Key:          "system.clear_cache",
        Type:         config.ValueTypeAction,  // This makes it an action
        Tab:          "system",
        Category:     "maintenance",
        DisplayOrder: 100,
        Label:        "Clear Cache",
        Description:  "Clears the application cache",
        Scopes:       []config.Scope{config.ScopeSystem},
        ViewPerm:     "config:read",
        EditPerm:     "config:manage",
        Icon:         "trash",                 // Lucide icon

        // Action-specific fields
        ActionEndpoint:             "/api/settings/actions/system.clear_cache/execute",
        ActionMethod:               "POST",
        ActionRequiresConfirmation: true,
        ActionConfirmationTitle:    "Clear Cache?",
        ActionConfirmationMessage:  "All cached data will be deleted.",
        ActionConfirmationButton:   "Clear Cache",
        ActionSuccessMessage:       "Cache cleared successfully",
        ActionErrorMessage:         "Failed to clear cache",
        ActionIsDangerous:          false,     // true = red styling
    })
}
```

### Step 2: Register the Action Handler

In `services/config/builtin_action_handlers.go`:

```go
import (
    "context"
    "github.com/moto-nrw/project-phoenix/models/config"
)

func RegisterBuiltinActionHandlers(settingsService HierarchicalSettingsService) {
    RegisterActionHandler("system.clear_cache", func(ctx context.Context, audit *config.ActionAuditContext) (*ActionResult, error) {
        // Perform the action
        settingsService.ClearCache()

        // Return result
        return &ActionResult{
            Success: true,
            Message: "Cache cleared successfully",
            Data: map[string]interface{}{
                "entriesCleared": 42,
            },
        }, nil
    })
}
```

### ActionResult Structure

```go
type ActionResult struct {
    Success bool                   // true = success, false = failure
    Message string                 // User-facing message
    Data    interface{}            // Optional additional data
}
```

### ActionAuditContext

Every action receives audit context:

```go
type ActionAuditContext struct {
    AccountID   int64   // User's account ID
    AccountName string  // User's display name
    IPAddress   string  // Client IP address
    UserAgent   string  // Client user agent
}
```

All action executions are logged to `config.action_audit_log` for compliance.

## Scopes and Inheritance

Settings can have values at different scopes:

```
System (default) → User override → Device override
```

The most specific value wins. For example:
- System sets `display.theme = "light"`
- User overrides with `display.theme = "dark"`
- User sees dark theme

Allowed scopes depend on the setting definition.

## Frontend Integration

The frontend automatically renders settings based on their type:

| Type | Rendered As |
|------|-------------|
| `string` | Text input (password input if `isSensitive`) |
| `int` | Number input with min/max |
| `float` | Number input with decimals |
| `bool` | Toggle switch |
| `enum` | Select dropdown |
| `time` | Time picker |
| `duration` | Text input with format hint |
| `objectRef` | Searchable select |
| `json` | Textarea with monospace font |
| `action` | Button with confirmation modal |

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/settings/tabs` | GET | List available tabs |
| `/api/settings/tabs/{key}` | GET | Get tab settings with resolved values |
| `/api/settings/values` | PUT | Update setting value |
| `/api/settings/actions/{key}/execute` | POST | Execute an action |
| `/api/settings/actions/{key}/history` | GET | Get action execution history |

## Best Practices

1. **Use descriptive keys**: Format as `category.setting_name` (e.g., `email.smtp_host`)
2. **Provide good defaults**: Settings should work out-of-the-box
3. **Write clear descriptions**: Help users understand what each setting does
4. **Group related settings**: Use consistent categories within tabs
5. **Consider permissions**: Not all users should edit all settings
6. **Mark sensitive data**: Use `IsSensitive: true` for passwords/secrets
7. **Validate input**: Use validation rules to prevent invalid values
8. **Actions need confirmation**: Destructive actions should require confirmation

## Database Tables

```
config.setting_tabs          - Tab definitions
config.setting_definitions   - Setting definitions (synced from code)
config.setting_values        - Stored setting values
config.action_audit_log      - Action execution history
```

Definitions are synced from code to database on server startup.
