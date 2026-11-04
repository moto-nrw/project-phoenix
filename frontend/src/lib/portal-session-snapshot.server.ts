import type { Session } from "next-auth";
import type { JWT } from "next-auth/jwt";
import { getToken } from "next-auth/jwt";
import { headers } from "next/headers";
import { env } from "~/env";
import { ACCESS_TOKEN_REFRESH_BUFFER_MS } from "~/server/auth/shared";
import {
  portalOfScope,
  sessionFamilyState,
} from "~/server/auth/token-validation";

/**
 * A decoded, fresh portal JWT. It still holds the access and refresh tokens:
 * each portal reader maps it through portalSessionSnapshot() before it
 * becomes a client prop (#5005).
 */
export type FreshPortalToken = JWT & {
  id: string;
  token: string;
  scope: string;
  tokenExpiry: number;
};

/**
 * Decodes one portal's Auth.js session cookie without running its refresh
 * callback (#4990). A Server Component cannot persist a rotated cookie, so
 * calling the portal's auth() in a layout could consume the refresh token and
 * strand the browser on the old cookie (#1938).
 *
 * Only the named cookie is read, decrypted with its own salt: another
 * portal's cookie on the same request (the domain-wide tenant cookie, for
 * example) neither decrypts nor counts. The token must carry the portal's
 * scope, be outside the refresh window and belong to a family Identity &
 * Access confirms live (#5399); otherwise the caller returns no snapshot and
 * SessionProvider fetches the session through the portal's response-aware
 * route as before.
 */
export async function readFreshPortalToken(
  cookieName: string,
  scope: string,
): Promise<FreshPortalToken | null> {
  const cookie = (await headers()).get("cookie");
  if (!cookie) return null;

  const token = await getToken({
    // The cookie header alone: getToken would otherwise fall back to an
    // Authorization bearer value, which is no portal cookie.
    req: { headers: new Headers({ cookie }) },
    secret: env.NEXTAUTH_SECRET,
    cookieName,
    salt: cookieName,
  });

  const now = Date.now();
  if (
    token?.scope !== scope ||
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
  // A cookie a late answer wrote back after its logout must not hydrate a
  // signed-in page (#5399): only a family Identity & Access confirms live
  // makes a snapshot; otherwise the provider asks the session route.
  if (
    (await sessionFamilyState(token.token, portalOfScope(token.scope))) !==
    "live"
  ) {
    return null;
  }

  return token as FreshPortalToken;
}

/**
 * Maps a fresh portal token to the Session a portal layout hands its
 * SessionProvider. The result is serialized into the HTML and RSC payload,
 * so it carries only what the portal renders from: no access token, refresh
 * token or recovery proof, and no part of one (#5005). API calls go through
 * BFF routes that read the cookie themselves; code that needs the token
 * calls getSession(). The poll after four minutes replaces the snapshot with
 * the session route's answer as before.
 */
export function portalSessionSnapshot(token: FreshPortalToken): Session {
  return {
    expires: new Date((token.exp ?? 0) * 1000).toISOString(),
    user: {
      id: token.id,
      name: token.name,
      email: token.email ?? "",
      roles: Array.isArray(token.roles) ? token.roles : [],
      permissions: Array.isArray(token.permissions) ? token.permissions : [],
      firstName:
        typeof token.firstName === "string" ? token.firstName : undefined,
      isAdmin: token.isAdmin === true,
      tenantId: typeof token.tenantId === "number" ? token.tenantId : undefined,
      orgId: typeof token.orgId === "number" ? token.orgId : undefined,
      scope: token.scope,
    },
  };
}
