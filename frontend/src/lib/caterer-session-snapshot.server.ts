import type { Session } from "next-auth";
import {
  portalSessionSnapshot,
  readFreshPortalToken,
} from "~/lib/portal-session-snapshot.server";
import { catererAuthConfig } from "~/server/auth/caterer-config";

/**
 * Read-only caterer session for CatererProviders (#4991), so the first
 * render needs no GET /api/caterer/auth/session. Only the caterer portal's
 * own cookie counts, and only a token with scope "caterer". Caterer tokens
 * carry no school (ADR 0077): the backend opens each supplied school per
 * request, so none is required here.
 *
 * The snapshot carries no token (portalSessionSnapshot, #5005). Returns null
 * when the cookie is missing, foreign, expired or due for a refresh; the
 * provider then fetches the session in the browser as before.
 */
export async function readCatererSessionSnapshot(): Promise<Session | null> {
  const token = await readFreshPortalToken(
    catererAuthConfig.cookies.sessionToken.name,
    "caterer",
  );
  return token ? portalSessionSnapshot(token) : null;
}
