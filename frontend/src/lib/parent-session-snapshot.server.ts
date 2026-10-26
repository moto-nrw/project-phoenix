import type { Session } from "next-auth";
import {
  portalSessionSnapshot,
  readFreshPortalToken,
} from "~/lib/portal-session-snapshot.server";
import { parentAuthConfig } from "~/server/auth/parent-config";

/**
 * Read-only parent session for ParentProviders (#4990), so the first render
 * needs no GET /api/parent/auth/session. Only the parents portal's own cookie
 * counts, and only a token with scope "parent"; parent tokens carry no school,
 * so none is required. Per-child rights are still checked by the backend on
 * every request.
 *
 * The snapshot carries no token (portalSessionSnapshot, #5005). Returns null
 * when the cookie is missing, foreign, expired or due for a refresh; the
 * provider then fetches the session in the browser as before.
 */
export async function readParentSessionSnapshot(): Promise<Session | null> {
  const token = await readFreshPortalToken(
    parentAuthConfig.cookies.sessionToken.name,
    "parent",
  );
  return token ? portalSessionSnapshot(token) : null;
}
