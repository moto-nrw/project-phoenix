import type { JWT } from "next-auth/jwt";
import { getToken } from "next-auth/jwt";
import { headers } from "next/headers";
import { env } from "~/env";
import { ACCESS_TOKEN_REFRESH_BUFFER_MS } from "~/server/auth/shared";

/**
 * A decoded, fresh portal JWT. It still holds the access and refresh tokens:
 * each portal reader maps it to a Session without any token before it
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
 * scope and be outside the refresh window; otherwise the caller returns no
 * snapshot and SessionProvider fetches the session through the portal's
 * response-aware route as before.
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

  return token as FreshPortalToken;
}
