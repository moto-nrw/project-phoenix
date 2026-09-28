/**
 * The browser's PostHog session travels with every request to the own host
 * (`tracing_headers` in analytics-policy.ts). Route handlers pass it on to
 * the backend, which links its core-action events (#3602) to the session of
 * the page views. Only a session UUID passes; anything else is dropped.
 */
const ANALYTICS_SESSION_HEADER = "X-POSTHOG-SESSION-ID";

const SESSION_ID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

type HeaderReader = Pick<Headers, "get">;

/** The session header to forward, read from the incoming request headers. */
export function analyticsSessionHeaders(
  incoming: HeaderReader | null | undefined,
): Record<string, string> {
  const sessionId = incoming?.get(ANALYTICS_SESSION_HEADER)?.trim();
  return sessionId && SESSION_ID.test(sessionId)
    ? { [ANALYTICS_SESSION_HEADER]: sessionId }
    : {};
}

/**
 * The session header of the request being handled, for helpers that do not
 * receive the request. Outside a request scope there is none.
 */
export async function incomingAnalyticsSessionHeaders(): Promise<
  Record<string, string>
> {
  try {
    const { headers } = await import("next/headers");
    return analyticsSessionHeaders(await headers());
  } catch {
    return {};
  }
}
