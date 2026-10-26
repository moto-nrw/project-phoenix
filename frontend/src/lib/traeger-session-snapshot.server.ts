import type { Session } from "next-auth";
import {
  portalSessionSnapshot,
  readFreshPortalToken,
} from "~/lib/portal-session-snapshot.server";
import { traegerAuthConfig } from "~/server/auth/traeger-config";

/**
 * Read-only Träger session for TraegerProviders (#4991), so the first render
 * needs no GET /api/traeger/auth/session. Only the Träger portal's own cookie
 * counts, and only a token with scope "org" that names its Träger and no
 * school (ADR 0084): a school session opened through the Absprung lives in
 * the tenant cookie and never counts here.
 *
 * The snapshot carries no token (portalSessionSnapshot, #5005). Returns null
 * when the cookie is missing, foreign, expired or due for a refresh; the
 * provider then fetches the session in the browser as before.
 */
export async function readTraegerSessionSnapshot(): Promise<Session | null> {
  const token = await readFreshPortalToken(
    traegerAuthConfig.cookies.sessionToken.name,
    "org",
  );
  if (
    !token ||
    typeof token.orgId !== "number" ||
    token.orgId <= 0 ||
    token.tenantId != null
  ) {
    return null;
  }
  return portalSessionSnapshot(token);
}
