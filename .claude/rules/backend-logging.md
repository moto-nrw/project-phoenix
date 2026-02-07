# Backend Logging: Use slog Only

**ABSOLUTE RULE: All backend Go code MUST use `log/slog` for logging. Never use `logrus`, `log.Printf`, or any other logging library.**

The project completed a full migration from logrus to Go's stdlib `log/slog`. The `sloglint` linter enforces conventions at build time. Use the `backend-structured-logging` skill for detailed usage instructions.

## How Logging Works

1. `applog.New()` bootstraps a `*slog.Logger` at startup (`cmd/serve.go`)
2. The logger is injected through the factory pattern: `services.NewFactory(repos, db, logger)`
3. Services receive scoped loggers: `logger.With("service", "active")`
4. Handlers receive loggers via their resource constructors

## Rules

### DO: Use injected logger
```go
// In a service constructor
func NewService(repo SomeRepo, logger *slog.Logger) *Service {
    return &Service{repo: repo, logger: logger}
}

// In service methods
func (s *Service) DoWork(ctx context.Context) error {
    s.logger.Info("processing request", "item_id", id)
    return nil
}
```

### DO: Use key-value pairs (not positional strings)
```go
// CORRECT
slog.Info("user authenticated", "account_id", accountID, "method", "jwt")

// WRONG - positional string args without keys
slog.Info("user authenticated", accountID, "jwt")
```

### DO: Use snake_case for log keys
```go
// CORRECT
slog.Info("visit recorded", "student_id", sid, "group_id", gid)

// WRONG
slog.Info("visit recorded", "studentId", sid, "groupId", gid)
```

### NEVER: Import logrus or use bare log.Printf
```go
// FORBIDDEN
import "github.com/sirupsen/logrus"
import "log"

logrus.Info("something")  // NO
log.Printf("something")   // NO
```

### NEVER: Create standalone loggers in services
```go
// FORBIDDEN - always use the injected logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
```

## GDPR: Student Names in Logs

**Student names MUST NOT appear at Info level or above.** This is a GDPR requirement.

```go
// CORRECT - use IDs at Info level
s.logger.Info("student checked in", "student_id", studentID)

// CORRECT - names only at Debug level
s.logger.Debug("student details", "student_id", studentID, "name", name)

// FORBIDDEN - name at Info level
s.logger.Info("student checked in", "student_name", name)
```

## Known Exceptions

These files intentionally use `log.Printf` and must NOT be converted:

- `auth/jwt/tokenauth.go` — startup configuration logging (process exits on failure)
- `cmd/`, `seed/`, `simulator/` — routed through slog default at WARN level via `slog.SetLogLoggerLevel(slog.LevelWarn)`

## Nil-Safe Logger Pattern

When adding `logger *slog.Logger` to a struct, tests that create the struct via literal `&Struct{}` will have a nil logger. Use the `getLogger()` pattern:

```go
func (s *MyStruct) getLogger() *slog.Logger {
    if s.logger != nil {
        return s.logger
    }
    return slog.Default()
}
```

Then use `s.getLogger()` instead of `s.logger` throughout the struct methods.

## Enforcement

- `sloglint` is configured in `.golangci.yml` and runs in CI
- Rules enforced: `no-mixed-arguments`, `key-naming-case: snake`, `args-on-sep-lines`
