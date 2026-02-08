/**
 * Centralized logging utility with structured JSON output
 *
 * Server-side: Writes JSON to stdout (captured by Promtail/Docker logs)
 * Client-side: Batches logs and ships via POST to /api/logs
 *
 * @example
 * ```typescript
 * const logger = createLogger({ component: "StudentList" });
 * logger.info("students loaded", { count: 25, group_id: "123" });
 * logger.error("fetch failed", { error: err.message });
 * ```
 */

/**
 * Log severity levels (matches backend slog)
 */
export type LogLevel = "debug" | "info" | "warn" | "error";

/**
 * Numeric levels for filtering (matches slog.Level)
 */
export const LogLevelValue: Record<LogLevel, number> = {
  debug: -4,
  info: 0,
  warn: 4,
  error: 8,
} as const;

/**
 * Structured log entry (Loki-compatible JSON)
 */
export interface LogEntry {
  timestamp: string; // ISO 8601
  level: LogLevel;
  msg: string;
  component?: string; // Component/module name
  route?: string; // Current path
  user_id?: string; // Session user ID (GDPR-safe)
  environment: "development" | "production" | "test";
  context: "server" | "client";
  [key: string]: unknown; // Additional key-value pairs
}

/**
 * Logger configuration
 */
export interface LoggerConfig {
  /** Minimum level to log (filters below this) */
  level?: LogLevel;
  /** Component/module name for all logs */
  component?: string;
  /** Additional context attached to all logs */
  defaultContext?: Record<string, unknown>;
  /** Enable/disable logging entirely */
  enabled?: boolean;
}

/**
 * Logger interface (abstract, swappable implementations)
 */
export interface Logger {
  /**
   * Log at DEBUG level (verbose, development only)
   */
  debug(msg: string, context?: Record<string, unknown>): void;

  /**
   * Log at INFO level (normal operations)
   */
  info(msg: string, context?: Record<string, unknown>): void;

  /**
   * Log at WARN level (recoverable issues)
   */
  warn(msg: string, context?: Record<string, unknown>): void;

  /**
   * Log at ERROR level (failures requiring attention)
   */
  error(msg: string, context?: Record<string, unknown>): void;

  /**
   * Create child logger with merged context
   */
  child(context: Record<string, unknown>): Logger;
}

/**
 * Client-side log batch for API shipping
 */
interface LogBatch {
  entries: LogEntry[];
  timestamp: string;
}

/**
 * Detect if code is running in browser or server
 */
function isBrowserContext(): boolean {
  return typeof window !== "undefined";
}

/**
 * Get environment from process.env
 */
function getEnvironment(): "development" | "production" | "test" {
  if (
    typeof process !== "undefined" &&
    process.env.NODE_ENV &&
    (process.env.NODE_ENV === "development" ||
      process.env.NODE_ENV === "production" ||
      process.env.NODE_ENV === "test")
  ) {
    return process.env.NODE_ENV;
  }
  return "development";
}

/**
 * Get log level from environment variable
 */
function getLogLevelFromEnv(): LogLevel {
  if (typeof process !== "undefined" && process.env.NEXT_PUBLIC_LOG_LEVEL) {
    const envLevel = process.env.NEXT_PUBLIC_LOG_LEVEL.toLowerCase();
    if (
      envLevel === "debug" ||
      envLevel === "info" ||
      envLevel === "warn" ||
      envLevel === "error"
    ) {
      return envLevel;
    }
  }
  return getEnvironment() === "production" ? "info" : "debug";
}

/**
 * Base logger configuration (can be overridden)
 */
const DEFAULT_CONFIG: Required<LoggerConfig> = {
  level: getLogLevelFromEnv(),
  component: "unknown",
  defaultContext: {},
  enabled: true,
};

/**
 * Server-side logger: writes JSON to stdout
 * (Promtail reads from Docker logs)
 */
class ServerLogger implements Logger {
  private config: Required<LoggerConfig>;

  constructor(config: Required<LoggerConfig>) {
    this.config = config;
  }

  debug(msg: string, context?: Record<string, unknown>): void {
    this.log("debug", msg, context);
  }

  info(msg: string, context?: Record<string, unknown>): void {
    this.log("info", msg, context);
  }

  warn(msg: string, context?: Record<string, unknown>): void {
    this.log("warn", msg, context);
  }

  error(msg: string, context?: Record<string, unknown>): void {
    this.log("error", msg, context);
  }

  child(context: Record<string, unknown>): Logger {
    return new ServerLogger({
      ...this.config,
      defaultContext: { ...this.config.defaultContext, ...context },
    });
  }

  /**
   * Core logging method
   */
  private log(
    level: LogLevel,
    msg: string,
    context?: Record<string, unknown>,
  ): void {
    if (!this.config.enabled) return;
    if (LogLevelValue[level] < LogLevelValue[this.config.level]) return;

    const entry: LogEntry = {
      timestamp: new Date().toISOString(),
      level,
      msg,
      component: this.config.component,
      environment: getEnvironment(),
      context: "server",
      ...this.config.defaultContext,
      ...context,
    };

    // Write JSON to stdout (Promtail captures this)
    console.log(JSON.stringify(entry));
  }
}

/**
 * Client-side logger: batches logs and ships to API
 */
class ClientLogger implements Logger {
  private config: Required<LoggerConfig>;
  private batch: LogEntry[] = [];
  private flushTimer: ReturnType<typeof setTimeout> | null = null;
  private readonly BATCH_SIZE = 10;
  private readonly BATCH_INTERVAL_MS = 5000; // 5 seconds

  constructor(config: Required<LoggerConfig>) {
    this.config = config;
    this.startBatchTimer();
  }

  debug(msg: string, context?: Record<string, unknown>): void {
    this.log("debug", msg, context);
  }

  info(msg: string, context?: Record<string, unknown>): void {
    this.log("info", msg, context);
  }

  warn(msg: string, context?: Record<string, unknown>): void {
    this.log("warn", msg, context);
  }

  error(msg: string, context?: Record<string, unknown>): void {
    this.log("error", msg, context);
  }

  child(context: Record<string, unknown>): Logger {
    return new ClientLogger({
      ...this.config,
      defaultContext: { ...this.config.defaultContext, ...context },
    });
  }

  /**
   * Core logging method
   */
  private log(
    level: LogLevel,
    msg: string,
    context?: Record<string, unknown>,
  ): void {
    if (!this.config.enabled) return;
    if (LogLevelValue[level] < LogLevelValue[this.config.level]) return;

    const entry: LogEntry = {
      timestamp: new Date().toISOString(),
      level,
      msg,
      component: this.config.component,
      environment: getEnvironment(),
      context: "client",
      ...this.config.defaultContext,
      ...context,
    };

    // Enrich with client-side context
    this.enrichClientContext(entry);

    // Add to batch
    this.batch.push(entry);

    // Also write to console in development
    if (getEnvironment() === "development") {
      this.consoleOutput(level, msg, entry);
    }

    // Flush if batch is full
    if (this.batch.length >= this.BATCH_SIZE) {
      void this.flush();
    }
  }

  /**
   * Enrich log entry with client-side context
   */
  private enrichClientContext(entry: LogEntry): void {
    // Add current route
    if (typeof window !== "undefined") {
      entry.route = window.location.pathname;
    }

    // Note: User context (user_id) would require async session access
    // For now, we skip it to keep the logger synchronous
    // If needed, this can be added via logger.child({ user_id: ... })
  }

  /**
   * Start batch flush timer
   */
  private startBatchTimer(): void {
    this.flushTimer = setInterval(() => {
      void this.flush();
    }, this.BATCH_INTERVAL_MS);
  }

  /**
   * Flush batch to API endpoint
   */
  private async flush(): Promise<void> {
    if (this.batch.length === 0) return;

    const payload: LogBatch = {
      entries: this.batch,
      timestamp: new Date().toISOString(),
    };

    // Clear batch immediately (avoid duplicates)
    this.batch = [];

    try {
      // POST to Next.js API route (proxies to logging pipeline)
      const response = await fetch("/api/logs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        // Fallback to console if API fails
        console.error("[Logger] Failed to ship logs:", response.statusText);
      }
    } catch (error) {
      // Network error - logs are lost (acceptable tradeoff)
      console.error("[Logger] Network error shipping logs:", error);
    }
  }

  /**
   * Output to console in development
   */
  private consoleOutput(level: LogLevel, msg: string, entry: LogEntry): void {
    const prefix = `[${entry.component}]`;
    const contextStr = JSON.stringify(
      { ...entry, msg: undefined, component: undefined },
      null,
      2,
    );

    switch (level) {
      case "debug":
        console.debug(prefix, msg, contextStr);
        break;
      case "info":
        console.info(prefix, msg, contextStr);
        break;
      case "warn":
        console.warn(prefix, msg, contextStr);
        break;
      case "error":
        console.error(prefix, msg, contextStr);
        break;
    }
  }

  /**
   * Cleanup on unmount (for React components)
   */
  destroy(): void {
    if (this.flushTimer) {
      clearInterval(this.flushTimer);
      this.flushTimer = null;
    }
    void this.flush(); // Final flush
  }
}

/**
 * Create a logger instance (auto-detects environment)
 *
 * @example
 * ```typescript
 * const logger = createLogger({ component: "StudentList" });
 * logger.info("students loaded", { count: 25, group_id: "123" });
 * ```
 */
export function createLogger(config?: LoggerConfig): Logger {
  const mergedConfig = { ...DEFAULT_CONFIG, ...config };

  if (isBrowserContext()) {
    return new ClientLogger(mergedConfig);
  } else {
    return new ServerLogger(mergedConfig);
  }
}

/**
 * Global default logger (lazy-initialized)
 */
let _defaultLogger: Logger | null = null;

/**
 * Get the default logger instance (singleton)
 */
export function getLogger(): Logger {
  _defaultLogger ??= createLogger();
  return _defaultLogger;
}
