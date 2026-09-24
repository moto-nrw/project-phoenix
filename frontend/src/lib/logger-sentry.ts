/**
 * Forwards structured log entries to Sentry.
 *
 * Handled errors (a failed save that shows an inline error) never reach
 * Sentry's automatic capture, and the logs alone cannot show what a user did
 * before a crash. So every error-level entry becomes a Sentry event, and on
 * the client every info-or-higher entry becomes a breadcrumb that later
 * events carry along.
 *
 * Entries arrive already redacted by the logger; Sentry's beforeSend scrubber
 * (sentry.shared.ts) runs on top.
 */

import * as Sentry from "@sentry/nextjs";

interface SentryLogEntry {
  level: "debug" | "info" | "warn" | "error";
  msg: string;
  component?: string;
  context: "server" | "client";
  [key: string]: unknown;
}

/**
 * Error-level messages that are expected noise, not defects: the SSE stream
 * reconnects on its own (about 700 entries a day in production), and a wrong
 * parent password is user input. They stay in the logs.
 */
const NOT_SENT_TO_SENTRY = new Set([
  "sse connection error",
  "parent login failed",
]);

const BREADCRUMB_LEVEL: Record<SentryLogEntry["level"], Sentry.SeverityLevel> =
  {
    debug: "debug",
    info: "info",
    warn: "warning",
    error: "error",
  };

// Envelope fields repeated on every entry; the event carries them elsewhere.
const ENVELOPE_KEYS = new Set([
  "timestamp",
  "level",
  "msg",
  "component",
  "environment",
  "context",
]);

function entryDetails(entry: SentryLogEntry): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(entry).filter(([key]) => !ENVELOPE_KEYS.has(key)),
  );
}

export function reportLogToSentry(entry: SentryLogEntry): void {
  const component = entry.component ?? "unknown";

  if (entry.context === "client" && entry.level !== "debug") {
    Sentry.addBreadcrumb({
      category: `log.${component}`,
      message: entry.msg,
      level: BREADCRUMB_LEVEL[entry.level],
      data: entryDetails(entry),
    });
  }

  if (entry.level !== "error" || NOT_SENT_TO_SENTRY.has(entry.msg)) return;

  Sentry.captureMessage(entry.msg, {
    level: "error",
    tags: { component, log_source: "logger" },
    extra: entryDetails(entry),
    fingerprint: ["logger", component, entry.msg],
  });
}
