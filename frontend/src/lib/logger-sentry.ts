/**
 * Forwards structured log entries to Sentry.
 *
 * Structured logs become breadcrumbs for independently reported exceptions.
 * A log level alone does not decide whether a failure is a defect, so logging
 * never creates a Sentry event.
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

  if (entry.level !== "debug") {
    Sentry.addBreadcrumb({
      category: `log.${component}`,
      message: entry.msg,
      level: BREADCRUMB_LEVEL[entry.level],
      data: entryDetails(entry),
    });
  }
}
