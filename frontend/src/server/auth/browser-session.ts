import "server-only";
import type { Session } from "next-auth";
import {
  familyStateOf,
  gateSessionCookieWrites,
  type FamilyStates,
} from "./session-cookie-writes";
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
 * POST of update()) answers with the browser session. Every other action is
 * left alone.
 *
 * The session it shows is confirmed with Identity & Access on each call
 * (#5399): a session whose refresh-token family is revoked reads as ended
 * (`RefreshTokenError`), even when a late answer wrote its cookie back after
 * the logout deleted it. Its cookie writes pass gateSessionCookieWrites: only
 * a changed session of a live family is written. A family that cannot be read
 * leaves the session as it is and writes nothing.
 */
export function withBrowserSession<A extends [Request, ...unknown[]]>(
  handler: (...args: A) => Promise<Response>,
): (...args: A) => Promise<Response> {
  return async (...args) => {
    const [request] = args;
    if (!new URL(request.url).pathname.endsWith("/session"))
      return handler(...args);
    const answer = await handler(...args);
    const body: unknown = await answer
      .clone()
      .json()
      .catch(() => null);
    const states: FamilyStates = new Map();
    const response = await gateSessionCookieWrites(request, answer, states);
    if (!isServerSession(body)) return response;

    const shown =
      body.error === undefined && body.user.token
        ? await familyStateOf(states, {
            token: body.user.token,
            scope: body.user.scope,
          })
        : null;
    const session: ServerSession =
      shown === "ended" ? { ...body, error: "RefreshTokenError" } : body;

    const headers = new Headers(response.headers);
    headers.delete("Content-Length");
    return new Response(JSON.stringify(toBrowserSession(session)), {
      status: response.status,
      statusText: response.statusText,
      headers,
    });
  };
}
