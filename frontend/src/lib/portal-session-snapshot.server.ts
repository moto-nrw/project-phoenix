import type { Session } from "next-auth";
import type { JWT } from "next-auth/jwt";
import { getToken } from "next-auth/jwt";
import { headers } from "next/headers";
import { env } from "~/env";
import { tenantSessionCookieName } from "~/lib/session-cookie-exit.server";
import { toBrowserSession } from "~/server/auth/browser-session";
import {
  PORTALS,
  type PortalDescriptor,
  type PortalKey,
} from "~/server/auth/portals";
import { serverSessionFromJwt } from "~/server/auth/session-callbacks";
import { ACCESS_TOKEN_REFRESH_BUFFER_MS } from "~/server/auth/session-lifetime";
import {
  familyBacksSession,
  sessionFamilyState,
} from "~/server/auth/token-validation";

export type { PortalKey };

/**
 * A portal layout's read of its session cookie. `session` is serialized into
 * the HTML and RSC payload and carries no token (#5005); `accessToken` stays
 * on the server, for the tenant shell's preload.
 */
export interface PortalSessionSnapshot {
  session: Session;
  accessToken: string;
  /** The school has not booked the portal's module; the session stays. */
  notBooked: boolean;
}

/** Auth.js' own name when the portal configures none (tenant on localhost). */
function sessionCookieName(key: PortalKey): string {
  const configured = PORTALS[key].cookieNames?.session;
  if (configured) return configured;
  const secure = new URL(env.NEXTAUTH_URL).protocol === "https:";
  return `${secure ? "__Secure-" : ""}${tenantSessionCookieName(undefined)}`;
}

/**
 * Decodes one portal's Auth.js session cookie without running its refresh
 * callback (#4990). A Server Component cannot persist a rotated cookie, so
 * calling the portal's auth() in a layout could consume the refresh token and
 * strand the browser on the old cookie (#1938).
 *
 * Only the portal's own cookie is read, decrypted with its own salt: another
 * portal's cookie on the same request (the domain-wide tenant cookie, for
 * example) neither decrypts nor counts. The token must carry one of the
 * portal's scopes and its snapshot claims, be outside the refresh window and
 * belong to a family Identity & Access confirms live (#5399), also when it
 * refuses the portal's module as not booked; otherwise the
 * layout renders without a snapshot and SessionProvider fetches the session
 * through the portal's response-aware route.
 */
async function readFreshPortalToken(
  key: PortalKey,
): Promise<{ token: JWT; notBooked: boolean } | null> {
  const cookie = (await headers()).get("cookie");
  if (!cookie) return null;
  const cookieName = sessionCookieName(key);
  const token = await getToken({
    // The cookie header alone: getToken would otherwise fall back to an
    // Authorization bearer value, which is no portal cookie.
    req: { headers: new Headers({ cookie }) },
    secret: env.NEXTAUTH_SECRET,
    cookieName,
    salt: cookieName,
  });

  const portal: PortalDescriptor = PORTALS[key];
  const now = Date.now();
  if (
    !token ||
    !portal.scopes.includes(token.scope) ||
    (portal.snapshotClaims && !portal.snapshotClaims(token)) ||
    typeof token.id !== "string" ||
    typeof token.token !== "string" ||
    token.token === "" ||
    typeof token.tokenExpiry !== "number" ||
    token.tokenExpiry <= now + ACCESS_TOKEN_REFRESH_BUFFER_MS ||
    (typeof token.refreshTokenExpiry === "number" &&
      token.refreshTokenExpiry <= now) ||
    token.error
  ) {
    return null;
  }
  // The read gate: a cookie a late answer wrote back after its logout must
  // not hydrate a signed-in page (#5399).
  const state = await sessionFamilyState(token.token, portal.validation);
  if (!familyBacksSession(state)) return null;
  return { token, notBooked: state === "notBooked" };
}

/**
 * The layout snapshot of a portal: the same projection the session route
 * answers with (serverSessionFromJwt, then toBrowserSession), so the first
 * render shows what the provider's poll later confirms.
 */
export async function readPortalSessionSnapshot(
  key: PortalKey,
): Promise<PortalSessionSnapshot | null> {
  const fresh = await readFreshPortalToken(key);
  if (!fresh?.token.token) return null;
  const { token, notBooked } = fresh;
  const accessToken = fresh.token.token;
  const session = serverSessionFromJwt(token, {
    expires: new Date((token.exp ?? 0) * 1000).toISOString(),
    user: {
      id: token.id ?? "",
      name: token.name,
      email: token.email,
      image: token.picture,
    },
  });
  return {
    session: toBrowserSession(session),
    accessToken,
    notBooked,
  };
}
