import type { ErrorEvent } from "@sentry/nextjs";

const routerStateParseMessage =
  "The router state header was sent but could not be parsed.";

// Calendar- and request-feed capability tokens ride in URL paths. Redact them
// from every recorded URL/path so a Sentry event cannot leak a replayable
// secret. This mirrors the backend request-log redactor.
const feedTokenPatterns = [
  /(\/api\/calendar-feed\/)[^/?#\s"]+/g,
  /(\/public\/calendar\/)[^/?#\s"]+/g,
  /(\/api\/request-feed\/)[^/?#\s"]+/g,
  /(\/public\/request-feed\/)[^/?#\s"]+/g,
];

function redactFeedToken(value: string): string {
  return feedTokenPatterns.reduce(
    (acc, pattern) => acc.replace(pattern, "$1[REDACTED]"),
    value,
  );
}

/**
 * Shared Sentry beforeSend handler for GDPR-compliant event scrubbing.
 * Used by client, server, and edge configs to keep scrubbing rules in sync.
 */
export function scrubEvent(event: ErrorEvent): ErrorEvent | null {
  if (isKnownRscRouterStateParseError(event)) {
    return null;
  }

  // Strip auth headers and cookies
  if (event.request?.headers) {
    delete event.request.headers["Authorization"];
    delete event.request.headers["authorization"];
    delete event.request.headers["Cookie"];
    delete event.request.headers["cookie"];
  }
  if (event.request?.cookies) {
    event.request.cookies = {};
  }

  // Strip PII from auto-collected user context
  if (event.user) {
    delete event.user.ip_address;
    delete event.user.email;
    delete event.user.username;
  }

  // Redact feed tokens from any URL/path Sentry captured.
  if (event.request?.url) {
    event.request.url = redactFeedToken(event.request.url);
  }
  if (typeof event.transaction === "string") {
    event.transaction = redactFeedToken(event.transaction);
  }
  const requestPath = getNestedString(event.contexts, "nextjs", "request_path");
  if (requestPath && isRecord(event.contexts?.nextjs)) {
    event.contexts.nextjs.request_path = redactFeedToken(requestPath);
  }
  if (event.breadcrumbs) {
    for (const breadcrumb of event.breadcrumbs) {
      if (typeof breadcrumb.message === "string") {
        breadcrumb.message = redactFeedToken(breadcrumb.message);
      }
      const url = breadcrumb.data?.url;
      if (typeof url === "string") {
        breadcrumb.data!.url = redactFeedToken(url);
      }
    }
  }

  return event;
}

function isKnownRscRouterStateParseError(event: ErrorEvent): boolean {
  if (!eventHasMessage(event, routerStateParseMessage)) {
    return false;
  }

  const requestPath = getNestedString(event.contexts, "nextjs", "request_path");
  if (requestPath?.includes("_rsc=")) {
    return true;
  }

  return event.request?.url?.includes("_rsc=") ?? false;
}

function eventHasMessage(event: ErrorEvent, message: string): boolean {
  if (event.message === message) {
    return true;
  }

  return (
    event.exception?.values?.some((value) => value.value === message) ?? false
  );
}

function getNestedString(
  source: unknown,
  firstKey: string,
  secondKey: string,
): string | undefined {
  if (!isRecord(source)) {
    return undefined;
  }
  const nested = source[firstKey];
  if (!isRecord(nested)) {
    return undefined;
  }
  const value = nested[secondKey];
  return typeof value === "string" ? value : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
