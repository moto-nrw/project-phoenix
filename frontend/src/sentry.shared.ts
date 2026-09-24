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

// Query strings carry search terms and names, fragments carry demo access
// tokens (#token=…). Neither may reach Sentry; the path with its IDs stays.
function stripQueryAndFragment(url: string): string {
  return url.replace(/[?#].*$/s, "");
}

// The same for URLs inside free text such as a breadcrumb message. Only
// URL-shaped tokens (absolute, or a path starting with "/") lose their query,
// so a question mark ending a sentence stays.
const urlInTextPattern = /((?:https?:\/\/|\/)[^\s"'?#]*)[?#][^\s"']+/g;

function scrubUrl(url: string): string {
  return redactFeedToken(stripQueryAndFragment(url));
}

function scrubText(text: string): string {
  return redactFeedToken(text.replace(urlInTextPattern, "$1"));
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

  // The internal account ID is the only user field Sentry may keep: no name,
  // e-mail, username or IP address.
  if (event.user) {
    event.user = event.user.id === undefined ? {} : { id: event.user.id };
  }

  scrubUrls(event);

  return event;
}

/**
 * Removes query strings and fragments from every URL Sentry captured and
 * redacts feed tokens in them: request, referrer, transaction name, the
 * Next.js request path, and breadcrumbs (navigation, fetch, xhr, log).
 */
function scrubUrls(event: ErrorEvent): void {
  if (event.request) {
    if (event.request.url) {
      event.request.url = scrubUrl(event.request.url);
    }
    delete event.request.query_string;
    const headers = event.request.headers;
    for (const name of Object.keys(headers ?? {})) {
      const value = headers?.[name];
      if (headers && value && name.toLowerCase() === "referer") {
        headers[name] = scrubUrl(value);
      }
    }
  }
  if (typeof event.transaction === "string") {
    event.transaction = scrubUrl(event.transaction);
  }
  const requestPath = getNestedString(event.contexts, "nextjs", "request_path");
  if (requestPath && isRecord(event.contexts?.nextjs)) {
    event.contexts.nextjs.request_path = scrubUrl(requestPath);
  }
  for (const breadcrumb of event.breadcrumbs ?? []) {
    if (typeof breadcrumb.message === "string") {
      breadcrumb.message = scrubText(breadcrumb.message);
    }
    const data = breadcrumb.data;
    if (!data) continue;
    for (const [key, value] of Object.entries(data)) {
      if (typeof value === "string") {
        data[key] = scrubText(value);
      }
    }
  }
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
