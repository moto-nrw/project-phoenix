import type { BrowserOptions, ErrorEvent, Event } from "@sentry/nextjs";

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

  scrubRequestAndUser(event);

  return event;
}

function scrubRequestAndUser(event: Event): void {
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
}

type SpanJSON = Parameters<NonNullable<BrowserOptions["beforeSendSpan"]>>[0];

// Span attributes that hold nothing but a query string or a fragment.
const querySpanAttributeKeys = [
  "http.query",
  "http.fragment",
  "url.query",
  "url.fragment",
];

// Personal fields the SDK copies onto spans from the scope user and the
// request. As on error events, only the account ID (`user.id`) stays.
const personalSpanAttributeKeys = [
  "user.email",
  "user.name",
  "user.username",
  "user.ip_address",
  "client.address",
  "http.client_ip",
];

// Request headers that carry credentials, as scrubEvent strips them.
const credentialHeaderAttributes = new Set([
  "http.request.header.authorization",
  "http.request.header.cookie",
]);

/**
 * beforeSendSpan: removes query strings, fragments and feed tokens from a
 * span's name and attributes (url.full, the referrer list of a page view,
 * browser.web_vital.lcp.url, …) and every personal field but the account ID,
 * the same data boundary as for error events. Paths with IDs stay.
 */
export function scrubSpan(span: SpanJSON): SpanJSON {
  span.name = scrubText(span.name);
  const attributes = span.attributes;
  for (const key of [...querySpanAttributeKeys, ...personalSpanAttributeKeys]) {
    delete attributes[key];
  }
  for (const key of Object.keys(attributes)) {
    if (credentialHeaderAttributes.has(key.toLowerCase())) {
      delete attributes[key];
    }
  }
  for (const [key, value] of Object.entries(attributes)) {
    attributes[key] = scrubAttributeValue(value);
  }
  return span;
}

// An attribute is a string, a list, or an object carrying the value and its
// unit.
function scrubAttributeValue(value: unknown): unknown {
  if (typeof value === "string") {
    return scrubText(value);
  }
  if (Array.isArray(value)) {
    return value.map((item: unknown) =>
      typeof item === "string" ? scrubText(item) : item,
    );
  }
  if (isRecord(value) && typeof value.value === "string") {
    return { ...value, value: scrubText(value.value) };
  }
  return value;
}

/**
 * What the SDK may collect on its own, the same in the browser, the BFF and
 * the edge. v11 collects user data (with the IP), cookies, all headers, bodies
 * and query strings unless told otherwise. The request URL is not covered;
 * scrubEvent and scrubSpan remove its query string.
 */
export const sentryDataCollection = {
  userInfo: false,
  cookies: false,
  httpHeaders: {
    request: { allow: ["user-agent", "referer"] },
    response: false,
  },
  httpBodies: [],
  urlQueryParams: false,
  graphQL: { document: false, variables: false },
  genAI: { inputs: false, outputs: false },
  databaseQueryData: false,
  queues: false,
  stackFrameVariables: false,
} satisfies BrowserOptions["dataCollection"];

/** Share of page loads and navigations the browser measures. */
export const pageViewTraceSampleRate = 0.05;

type TracesSamplingContext = Parameters<
  NonNullable<BrowserOptions["tracesSampler"]>
>[0];

// SEMANTIC_ATTRIBUTE_SENTRY_OP, spelled out so this module stays type-only.
const spanOpAttribute = "sentry.op";

// Ops of the Web Vital spans the SDK sends on their own, after the page load
// ended: ui.webvital.lcp, ui.webvital.cls and ui.interaction.<click|keyboard|
// pointer|drag> for INP.
const webVitalOpPrefixes = ["ui.webvital.", "ui.interaction."];

/**
 * Browser tracesSampler: 5 % of page loads and navigations, each decided in
 * the browser because the server never samples. With span streaming, LCP, CLS
 * and INP are child spans of their page view and inherit its decision without
 * asking the sampler. The Web Vital rule covers the case where the SDK starts
 * one as a root span: it is kept only inside a page view that was already
 * sampled. Every other root span stays unmeasured.
 */
export function sampleBrowserTrace(context: TracesSamplingContext): number {
  const op = context.attributes?.[spanOpAttribute];
  if (op === "pageload" || op === "navigation") {
    return pageViewTraceSampleRate;
  }
  const isWebVitalSpan =
    typeof op === "string" &&
    webVitalOpPrefixes.some((prefix) => op.startsWith(prefix));
  return isWebVitalSpan && context.parentSampled === true ? 1 : 0;
}

/**
 * Server and edge tracesSampler: never record a span. A plain
 * tracesSampleRate of 0 would still follow a sampled browser trace and send
 * BFF spans. Tracing stays enabled, so trace headers still reach the backend
 * and error events keep the trace ID.
 */
export function sampleNoTrace(): number {
  return 0;
}

/**
 * Removes query strings and fragments from every URL Sentry captured and
 * redacts feed tokens in them: request, referrer, transaction name, the
 * Next.js request path, and breadcrumbs (navigation, fetch, xhr, log).
 */
function scrubUrls(event: Event): void {
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
