---
name: backend-structured-logging
description: Use when writing or modifying backend Go code that needs logging. Covers slog injection via factory pattern, scoped loggers, key-value conventions, GDPR constraints, nil-safe patterns, and sloglint enforcement. Triggers on log statements, error handling, new service/handler creation, or any logging-related backend work.
---

# Backend Structured Logging (slog)

This project uses Go's stdlib `log/slog` exclusively. The logger is bootstrapped once at startup (`applog.New()`) and injected through the factory pattern. All log output flows to Grafana/Loki as structured JSON.

**MANDATORY**: Never use `logrus`, `log.Printf`, or any other logging library. Only `log/slog` via the injected logger.

## Architecture

```
cmd/serve.go
     │
     ▼
 applog.New(Config{Level, Format, Env})
     │
     ├─ Creates *slog.Logger (JSON in prod, Text in dev)
     ├─ Sets slog.SetDefault() (captures stray log.Printf as WARN)
     ├─ Adds global "env" attribute
     │
     ▼
 services.NewFactory(repos, db, logger)
     │
     ├─ logger.With("service", "active")      → Active Service
     ├─ logger.With("service", "auth")         → Auth Service
     ├─ logger.With("service", "platform")     → Platform Services
     ├─ logger.With("service", "facilities")   → Facilities Service
     ├─ logger.With("service", "usercontext")  → UserContext Service
     ├─ logger.With("service", "database")     → Database Service
     ├─ logger.With("component", "email")      → Email Dispatcher
     └─ logger.With("component", "sse-hub")    → Realtime Hub
           │
           ▼
     API Layer receives logger too:
     ├─ logger.With("handler", "active")       → Active Handler
     ├─ logger.With("handler", "iot")          → IoT Handler
     └─ logger.With("handler", "sse")          → SSE Handler
```

## Quick Start

### 1. In a service — accept logger via constructor

```go
import "log/slog"

type myService struct {
    repo   SomeRepository
    logger *slog.Logger
}

func NewMyService(repo SomeRepository, logger *slog.Logger) *myService {
    return &myService{
        repo:   repo,
        logger: logger,
    }
}
```

### 2. Use key-value pairs in log calls

```go
// CORRECT — key-value pairs, snake_case keys
s.logger.Info("visit recorded",
    "student_id", studentID,
    "group_id", groupID,
)

s.logger.Error("checkin failed",
    "error", err,
    "student_id", studentID,
)

s.logger.Warn("room conflict detected",
    "room_id", roomID,
    "active_groups", count,
)

s.logger.Debug("query executed",
    "duration_ms", elapsed.Milliseconds(),
    "rows", rowCount,
)
```

### 3. WRONG patterns (sloglint will reject these)

```go
// WRONG — positional args without keys
s.logger.Info("visit recorded", studentID, groupID)

// WRONG — camelCase keys
s.logger.Info("visit recorded", "studentId", sid, "groupId", gid)

// WRONG — mixed args (some keyed, some not)
s.logger.Info("visit recorded", "student_id", sid, groupID)

// WRONG — bare log.Printf
log.Printf("visit recorded: %d", studentID)

// WRONG — logrus
logrus.Info("visit recorded")

// WRONG — standalone logger creation in a service
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
```

## Scoped Logger Convention

The factory creates scoped loggers with a `"service"` or `"component"` key. This appears on every log line for Grafana filtering.

| Layer | Key | Pattern | Example |
|-------|-----|---------|---------|
| Service | `"service"` | Domain name | `logger.With("service", "active")` |
| Handler | `"handler"` | Domain name | `logger.With("handler", "iot")` |
| Infrastructure | `"component"` | Component name | `logger.With("component", "sse-hub")` |

When adding a new service to the factory, create a scoped logger:

```go
// In services/factory.go NewFactory()
myLogger := logger.With("service", "mydomain")

myService := mydomain.NewService(repos.MyRepo, db, myLogger)
```

## Nil-Safe Logger Pattern

When a struct accepts `logger *slog.Logger`, tests creating the struct via literal `&Struct{}` will have a nil logger. Use the `getLogger()` pattern:

```go
type myService struct {
    repo   SomeRepository
    db     *bun.DB
    logger *slog.Logger
}

func (s *myService) getLogger() *slog.Logger {
    if s.logger != nil {
        return s.logger
    }
    return slog.Default()
}

// Then use s.getLogger() everywhere instead of s.logger
func (s *myService) DoWork(ctx context.Context) error {
    s.getLogger().Info("processing started", "item_id", id)
    return nil
}
```

This pattern is already used in: `schulhof_service.go`, `invitation_service.go`, `hub.go`, `scheduler.go`.

## GDPR: Student Names in Logs

**Student names MUST NOT appear at Info level or above.** This is a legal requirement.

```go
// CORRECT — IDs at Info level
s.logger.Info("student checked in", "student_id", studentID)

// CORRECT — names only at Debug level (filtered in production)
s.logger.Debug("student details",
    "student_id", studentID,
    "name", name,
)

// FORBIDDEN — name at Info level
s.logger.Info("student checked in", "student_name", name)
```

## Log Level Guidelines

| Level | Purpose | Visible in production? |
|-------|---------|----------------------|
| `Debug` | Development verbosity, SQL timing, intermediate values | No (filtered by `LOG_LEVEL`) |
| `Info` | Normal operations worth tracking (checked in, session started, cleanup done) | Yes |
| `Warn` | Recoverable issues, missing optional data, degraded behavior, stray `log.Printf` | Yes |
| `Error` | Failures that need attention (DB errors, failed broadcasts, invalid state) | Yes |

Configure via `LOG_LEVEL` environment variable (default: `"info"`). Format via `LOG_FORMAT` (`"json"` in production, `"text"` in development).

## Error Handling Pattern (Copy-Paste Ready)

### In service methods

```go
func (s *myService) CreateRecord(ctx context.Context, input CreateInput) (*Record, error) {
    record, err := s.repo.Create(ctx, input)
    if err != nil {
        s.getLogger().Error("record creation failed",
            "error", err,
            "input_id", input.ID,
        )
        return nil, fmt.Errorf("create record: %w", err)
    }

    s.getLogger().Info("record created",
        "record_id", record.ID,
    )
    return record, nil
}
```

### Fire-and-forget operations (SSE, email)

```go
// SSE broadcast — log but never block
if s.broadcaster != nil {
    event := realtime.NewEvent(realtime.EventStudentCheckIn, activeGroupID, data)
    if err := s.broadcaster.BroadcastToGroup(activeGroupID, event); err != nil {
        s.getLogger().Warn("SSE broadcast failed",
            "error", err,
            "active_group_id", activeGroupID,
            "event_type", string(event.Type),
        )
    }
}
```

### In handler methods

```go
func (rs *Resource) handleCreate(w http.ResponseWriter, r *http.Request) {
    // Handlers typically delegate to services.
    // Only log at the handler level for HTTP-specific concerns:
    rs.logger.Debug("request received",
        "method", r.Method,
        "path", r.URL.Path,
    )
}
```

## sloglint Enforcement

The `sloglint` linter is configured in `.golangci.yml` and runs in CI:

```yaml
linters:
  enable:
    - sloglint
  settings:
    sloglint:
      no-mixed-args: true       # No mixing keyed and positional args
      key-naming-case: snake    # Keys must be snake_case
      args-on-sep-lines: true   # Each key-value pair on its own line
```

This means `golangci-lint run` will reject:
- Mixed positional/keyed arguments
- camelCase or PascalCase log keys
- Multiple key-value pairs on one line

## Configuration Reference

| Env Variable | Values | Default | Purpose |
|-------------|--------|---------|---------|
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` | Minimum log level |
| `LOG_FORMAT` | `json`, `text` | `json` (prod), `text` (dev) | Output format |
| `APP_ENV` | `development`, `test`, `production` | — | Adds `"env"` field, enables source location in prod |

## Known Exceptions

These files intentionally use `log.Printf` and must NOT be converted:

| File | Reason |
|------|--------|
| `auth/jwt/tokenauth.go` | Startup config logging (process exits on failure) |
| `cmd/` package | CLI bootstrapping (before logger exists) |
| `seed/` package | One-off data seeding |
| `simulator/` package | Standalone simulation tool |

All `log.Printf` calls from these files route through slog as WARN level via `slog.SetLogLoggerLevel(slog.LevelWarn)`.

## File Reference

| File | Purpose |
|------|---------|
| `applog/applog.go` | Logger bootstrap (`New()`, `parseLevel()`, `SetDefault`) |
| `applog/applog_test.go` | Logger unit tests (JSON output, level filtering, stdlib routing) |
| `services/factory.go` | Scoped logger creation and injection into all services |
| `cmd/serve.go` | Entry point — calls `applog.New()` with viper config |
| `.golangci.yml` | sloglint configuration (`no-mixed-args`, `snake` keys, `args-on-sep-lines`) |
