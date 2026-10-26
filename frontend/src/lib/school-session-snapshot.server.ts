import type { Session } from "next-auth";
import {
  portalSessionSnapshot,
  readFreshPortalToken,
} from "~/lib/portal-session-snapshot.server";
import { schoolAuthConfig } from "~/server/auth/school-config";

/**
 * Read-only school session for SchoolProviders (#4991), so the first render
 * needs no GET /api/school/auth/session. Only the school portal's own cookie
 * counts, and only a token with scope "school" that names its school: school
 * tokens are tenant-bound, and the portal keys its data cache on that school.
 * The backend still checks the school on every request.
 *
 * The snapshot carries no token (portalSessionSnapshot, #5005). Returns null
 * when the cookie is missing, foreign, expired or due for a refresh; the
 * provider then fetches the session in the browser as before.
 */
export async function readSchoolSessionSnapshot(): Promise<Session | null> {
  const token = await readFreshPortalToken(
    schoolAuthConfig.cookies.sessionToken.name,
    "school",
  );
  if (!token || typeof token.tenantId !== "number" || token.tenantId <= 0) {
    return null;
  }
  return portalSessionSnapshot(token);
}
