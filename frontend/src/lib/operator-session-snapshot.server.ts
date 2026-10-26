import type { Session } from "next-auth";
import {
  portalSessionSnapshot,
  readFreshPortalToken,
} from "~/lib/portal-session-snapshot.server";
import { operatorAuthConfig } from "~/server/auth/operator-config";

/**
 * Read-only operator session for OperatorProviders (#4991), so the first
 * render needs no GET /api/operator/auth/session. Only the operator portal's
 * own cookie counts, and only a token with scope "platform": a tenant cookie
 * on the same request is never read as a fallback.
 *
 * The snapshot carries no token (portalSessionSnapshot, #5005). Returns null
 * when the cookie is missing, foreign, expired or due for a refresh; the
 * provider then fetches the session in the browser as before.
 */
export async function readOperatorSessionSnapshot(): Promise<Session | null> {
  const token = await readFreshPortalToken(
    operatorAuthConfig.cookies.sessionToken.name,
    "platform",
  );
  return token ? portalSessionSnapshot(token) : null;
}
