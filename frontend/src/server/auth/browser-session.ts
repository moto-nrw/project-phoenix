import "server-only";
import type { Session } from "next-auth";
import type { ServerSession } from "./shared";

/**
 * Every user field the browser may see (#5005). The type demands one entry
 * per field of Session["user"], and Session carries no token: a new session
 * field reaches the browser only once it is listed here.
 */
const BROWSER_USER_FIELDS = {
  id: true,
  name: true,
  email: true,
  image: true,
  roles: true,
  roleIsSystem: true,
  permissions: true,
  firstName: true,
  isAdmin: true,
  tenantId: true,
  orgId: true,
  scope: true,
  isPreview: true,
  previewTargetName: true,
  previewTargetAccountId: true,
} as const satisfies Record<keyof Session["user"], true>;

const BROWSER_USER_FIELD_NAMES = Object.keys(
  BROWSER_USER_FIELDS,
) as (keyof Session["user"])[];

/**
 * Maps the server's session to the one the browser receives. Access token,
 * refresh token and every other field outside the allowlist stay on the
 * server: BFF routes, the SSE proxy and server components read them from the
 * session cookie themselves.
 */
export function toBrowserSession(session: ServerSession): Session {
  const user: Record<string, unknown> = {};
  for (const field of BROWSER_USER_FIELD_NAMES) {
    if (session.user[field] !== undefined) user[field] = session.user[field];
  }
  return {
    expires: session.expires,
    user: user as unknown as Session["user"],
    ...(session.error === undefined ? {} : { error: session.error }),
  };
}

function isServerSession(value: unknown): value is ServerSession {
  if (typeof value !== "object" || value === null) return false;
  const user = (value as { user?: unknown }).user;
  return typeof user === "object" && user !== null;
}

/**
 * Wraps a portal's Auth.js route handler so the session action (GET and the
 * POST of update()) answers with the browser session. Status and headers,
 * including the rotated session cookie, pass through unchanged; every other
 * action is left alone.
 */
export function withBrowserSession<A extends [Request, ...unknown[]]>(
  handler: (...args: A) => Promise<Response>,
): (...args: A) => Promise<Response> {
  return async (...args) => {
    const response = await handler(...args);
    if (!new URL(args[0].url).pathname.endsWith("/session")) return response;

    const body: unknown = await response
      .clone()
      .json()
      .catch(() => null);
    if (!isServerSession(body)) return response;

    const headers = new Headers(response.headers);
    headers.delete("Content-Length");
    return new Response(JSON.stringify(toBrowserSession(body)), {
      status: response.status,
      statusText: response.statusText,
      headers,
    });
  };
}
