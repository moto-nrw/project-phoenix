# No Fallback Values - Fail Fast Policy

## Purpose

This project enforces a **fail-fast policy**. Hardcoded fallback values mask configuration errors, delay failure detection, and create subtle bugs that are difficult to trace. Missing configuration should cause immediate, loud failure at startup—not silent degradation.

## Rule: NEVER Use Fallback Values for Configuration

When you read code containing fallback patterns, you MUST:
1. **Flag it immediately** to the user
2. **Recommend removal** of the fallback
3. **Suggest fail-fast alternative** (validation + error)

## Banned Patterns

### TypeScript/JavaScript
```typescript
// BANNED - Silent fallback
const API_URL = process.env.API_URL ?? "http://localhost:8080";
const API_URL = process.env.API_URL || "http://localhost:8080";
const API_URL = process.env.API_URL ? process.env.API_URL : "http://localhost:8080";

// REQUIRED - Fail fast
const API_URL = process.env.API_URL;
if (!API_URL) {
  throw new Error("API_URL environment variable is required");
}
```

### Go
```go
// BANNED - Silent fallback
apiURL := os.Getenv("API_URL")
if apiURL == "" {
    apiURL = "http://localhost:8080"  // NEVER DO THIS
}

// Also banned via viper
viper.SetDefault("api_url", "http://localhost:8080")  // BANNED

// REQUIRED - Fail fast
apiURL := os.Getenv("API_URL")
if apiURL == "" {
    log.Fatal("API_URL environment variable is required")
}
```

### Python
```python
# BANNED - Silent fallback
api_url = os.getenv("API_URL", "http://localhost:8080")
api_url = os.environ.get("API_URL") or "http://localhost:8080"

# REQUIRED - Fail fast
api_url = os.environ["API_URL"]  # Raises KeyError if missing
# OR
api_url = os.getenv("API_URL")
if not api_url:
    raise ValueError("API_URL environment variable is required")
```

## What This Rule Covers

| Category | Examples | Action |
|----------|----------|--------|
| Environment variables | `process.env.X ?? "default"` | Remove fallback, add validation |
| Configuration files | Default values in config structs | Make fields required |
| Database connections | Fallback connection strings | Fail if not configured |
| API endpoints | Hardcoded URLs as fallbacks | Require explicit configuration |
| Feature flags | Default to enabled/disabled | Require explicit setting |
| Credentials/Secrets | Any default value | CRITICAL: Never have defaults |

## Exceptions (Very Limited)

Fallbacks are ONLY acceptable for:
1. **Computed defaults** that don't hide configuration errors (e.g., `port := configuredPort; if port == 0 { port = 8080 }` where 0 is explicitly "use default")
2. **Feature toggles in tests** where the default is clearly documented test behavior
3. **Backwards compatibility migrations** with explicit deprecation warnings and removal timeline

Even for exceptions, prefer explicit configuration over implicit defaults.

## When You Encounter Fallbacks

When reading code with fallback patterns, respond like this:

> **Fallback Value Detected**
>
> Found fallback pattern at `file.ts:42`:
> ```typescript
> const API_URL = process.env.API_URL ?? "http://localhost:8080";
> ```
>
> **This violates our fail-fast policy.** Fallbacks mask configuration errors and should be removed.
>
> **Recommended fix:**
> ```typescript
> const API_URL = process.env.API_URL;
> if (!API_URL) {
>   throw new Error("API_URL environment variable is required");
> }
> ```
>
> Should I fix this now?

## Why Fail Fast?

1. **Immediate feedback** - Errors surface at startup, not in production at 3 AM
2. **Clear root cause** - "Missing API_URL" vs "Connection refused to localhost:8080"
3. **No silent degradation** - System either works correctly or doesn't start
4. **Explicit configuration** - All required settings are documented by their validation
5. **Security** - No accidental exposure of development endpoints in production

## Enforcement

This rule applies to:
- All new code
- All code modifications
- Code review suggestions

When in doubt, fail fast. A clear startup error is always better than mysterious runtime behavior.
