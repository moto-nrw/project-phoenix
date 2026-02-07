---
name: frontend-structured-logging
description: Use when writing or modifying frontend TypeScript/React code that needs logging. Covers the structured logger (createLogger), log levels, error handling patterns, and test assertions. Triggers on catch blocks, error handling, console.* usage, or any logging-related frontend work.
---

# Frontend Structured Logging

This project uses a centralized structured logging system (`~/lib/logger`) that mirrors the backend Go `slog` architecture. All frontend logging flows through this system for Grafana/Loki observability.

**MANDATORY**: Never use `console.log`, `console.error`, `console.warn`, or `console.info` in production code. Always use the structured logger.

## Architecture

```
Client Component                     Server Component / API Route
     │                                        │
     ▼                                        ▼
 ClientLogger                            ServerLogger
     │                                        │
     ├─ Batches logs (10 entries / 5s)        ├─ Writes JSON to stdout
     ├─ Ships via POST /api/logs              │  (Promtail/Alloy captures)
     └─ Console output in dev mode only       │
                                              ▼
                                         Loki / Grafana
```

## Quick Start

### 1. Import and create a scoped logger (module-level)

```typescript
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MyComponentName" });
```

### 2. Use in catch blocks and important operations

```typescript
// Error handling (most common pattern)
} catch (error) {
  logger.error("profile_save_failed", {
    error: error instanceof Error ? error.message : String(error),
  });
}

// Informational logging
logger.info("students_loaded", { count: students.length, group_id: groupId });

// Warnings for degraded behavior
logger.warn("no_permission_to_view_group", { group_id: roomId });

// Debug for development verbosity
logger.debug("swr_fetch_complete", { duration_ms: Math.round(elapsed) });
```

## Component Naming Convention

The `component` field appears in every log entry and is used for Grafana filtering.

| File Type | Pattern | Example |
|-----------|---------|---------|
| Page | `{PageName}Page` | `ActiveSupervisionsPage` |
| API route | `{Domain}{Action}Route` | `AuthLoginRoute`, `OperatorLoginRoute` |
| Hook | `use{Name}` | `useOperatorSuggestionsUnread` |
| Context | `{Name}Context` | `OperatorAuthContext` |
| Modal/Component | `{Name}` | `AnnouncementModal`, `StatusDropdown` |

## Migration Cheat Sheet

When replacing `console.*` calls:

| Before (FORBIDDEN) | After (CORRECT) |
|---|---|
| `console.error("Failed to save:", error)` | `logger.error("save_failed", { error: error instanceof Error ? error.message : String(error) })` |
| `console.warn("No data for group")` | `logger.warn("no_data_for_group")` |
| `console.log("Fetched", items.length, "items")` | `logger.info("items_fetched", { count: items.length })` |
| `console.log("Debug:", someVar)` | `logger.debug("debug_info", { value: someVar })` |

### Key differences from console.*:
1. **First arg is an event name** (snake_case), NOT a human sentence
2. **Second arg is a context object** (key-value pairs), NOT positional args
3. **Never pass raw Error objects** — extract `.message` first
4. **Logger is module-scoped**, created once at the top of the file

## Log Level Guidelines

| Level | Purpose | Visible in production? |
|-------|---------|----------------------|
| `debug` | Development verbosity, performance timing, intermediate values | No (filtered by `NEXT_PUBLIC_LOG_LEVEL`) |
| `info` | Normal operations worth tracking (data loaded, actions completed) | Yes |
| `warn` | Recoverable issues, missing optional data, degraded behavior | Yes |
| `error` | Failures in catch blocks, unexpected states | Yes |

Configure via `NEXT_PUBLIC_LOG_LEVEL` environment variable (default: `debug` in dev, `info` in production).

## Error Handling Pattern (Copy-Paste Ready)

### In React components / hooks

```typescript
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MyPage" });

// In a handler or effect:
try {
  await someApiCall();
} catch (error) {
  logger.error("api_call_failed", {
    error: error instanceof Error ? error.message : String(error),
  });
  // Show user-facing error (toast, alert, etc.)
}
```

### In API route handlers (server-side)

```typescript
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MyApiRoute" });

export async function POST(request: NextRequest) {
  try {
    // ... handle request
  } catch (error) {
    logger.error("request_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json({ error: "Internal error" }, { status: 500 });
  }
}
```

### In fire-and-forget promises

```typescript
someAsyncAction().catch((error) => {
  logger.error("background_action_failed", {
    error: error instanceof Error ? error.message : String(error),
  });
});
```

## Child Loggers

For long-lived contexts, create child loggers with additional fields:

```typescript
const logger = createLogger({ component: "ActiveSupervisionsPage" });
const groupLogger = logger.child({ group_id: activeGroupId });

groupLogger.info("supervision_started"); // Inherits component + group_id
groupLogger.error("checkout_failed", { student_id: sid });
```

## Testing

The logger is **globally mocked** in `frontend/src/test/setup.ts`. The mock passes calls through to `console.*`, so existing spy patterns work:

```typescript
it("handles errors gracefully", async () => {
  const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});

  mockApiCall.mockRejectedValue(new Error("Network error"));

  // ... trigger the error in the component ...

  await waitFor(() => {
    expect(consoleError).toHaveBeenCalledWith("api_call_failed", {
      error: "Network error",
    });
  });

  consoleError.mockRestore();
});
```

### Key points for tests:
- Spy on `console.error` (not the logger directly) — the mock routes logger calls to console
- Assert with exact event name + context object (not `expect.any(Error)`)
- The error string comes from `error.message` extraction, so match the mock's error message
- Always call `.mockRestore()` after assertions

## File Reference

| File | Purpose |
|------|---------|
| `src/lib/logger.ts` | Logger implementation (ServerLogger + ClientLogger) |
| `src/app/api/logs/route.ts` | Log shipping endpoint (client batches POST here) |
| `src/test/setup.ts` | Global test mock (passes through to console.*) |
| `src/env.js` | `NEXT_PUBLIC_LOG_LEVEL` Zod validation |
