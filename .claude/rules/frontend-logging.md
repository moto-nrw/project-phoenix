# Frontend Logging: Use Structured Logger Only (MANDATORY)

**ABSOLUTE RULE: All frontend TypeScript/React code MUST use `createLogger` from `~/lib/logger` for logging. NEVER use bare `console.log`, `console.error`, `console.warn`, or `console.info`.**

This project uses a structured logging system that mirrors the backend `slog` architecture, enabling Grafana/Loki observability across the full stack. Use the `frontend-structured-logging` skill for detailed usage instructions.

## Rules

### ALWAYS: Import and create a scoped logger
```typescript
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MyComponentName" });
```

### ALWAYS: Use snake_case event names as the first argument
```typescript
// CORRECT
logger.error("profile_save_failed", { error: err.message });
logger.warn("no permission to view group", { group_id: roomId });
logger.info("students loaded", { count: 25, group_id: "123" });
logger.debug("SWR fetch complete", { duration_ms: 42 });

// WRONG - human-readable sentences
logger.error("Failed to save profile", { error: err.message });
console.error("Failed to save profile:", err);
```

### ALWAYS: Extract error messages, never pass raw Error objects
```typescript
// CORRECT
} catch (error) {
  logger.error("fetch_failed", {
    error: error instanceof Error ? error.message : String(error),
  });
}

// WRONG - passing raw Error object
} catch (error) {
  logger.error("fetch_failed", { error });
  console.error("fetch_failed:", error);
}
```

### NEVER: Use bare console.* in production code
```typescript
// FORBIDDEN
console.log("something happened");
console.error("Failed to fetch:", error);
console.warn("Missing data");
console.info("User logged in");
```

### NEVER: Create standalone logger instances with custom config
```typescript
// FORBIDDEN - always use createLogger from ~/lib/logger
const logger = { error: console.error, warn: console.warn };
```

## Component Naming Convention

| File Type | Component Name Pattern | Example |
|-----------|----------------------|---------|
| Page component | `{PageName}Page` | `createLogger({ component: "SettingsPage" })` |
| API route handler | `{Domain}{Action}Route` | `createLogger({ component: "AuthLoginRoute" })` |
| React hook | `use{HookName}` | `createLogger({ component: "useOperatorSuggestionsUnread" })` |
| Context provider | `{Name}Context` | `createLogger({ component: "OperatorAuthContext" })` |
| UI component | `{ComponentName}` | `createLogger({ component: "AnnouncementModal" })` |

## Log Level Guidelines

| Level | When to use | Example |
|-------|------------|---------|
| `debug` | Verbose development info, performance timing | `logger.debug("SWR fetch complete", { duration_ms: 42 })` |
| `info` | Normal operations worth tracking | `logger.info("students loaded", { count: 25 })` |
| `warn` | Recoverable issues, degraded behavior | `logger.warn("no permission to view group", { group_id })` |
| `error` | Failures in catch blocks | `logger.error("fetch_failed", { error: err.message })` |

## Testing

The logger is globally mocked in `frontend/src/test/setup.ts`. The mock passes through to `console.*`, so tests can spy on `console.error` to assert logging:

```typescript
const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});

// ... trigger error ...

expect(consoleError).toHaveBeenCalledWith("event_name", {
  error: "Expected error message",
});
consoleError.mockRestore();
```

## Exceptions

The ONLY files allowed to use raw `console.*`:
- `src/lib/logger.ts` — The logger implementation itself
- `src/test/setup.ts` — Global test mock pass-through
- `src/app/api/logs/route.ts` — Log shipping endpoint (writes JSON to stdout)
