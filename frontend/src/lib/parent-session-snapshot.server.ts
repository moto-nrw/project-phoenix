import type { Session } from "next-auth";
import { readFreshPortalToken } from "~/lib/portal-session-snapshot.server";
import { parentAuthConfig } from "~/server/auth/parent-config";

/**
 * Read-only parent session for ParentProviders (#4990), so the first render
 * needs no GET /api/parent/auth/session. Only the parents portal's own cookie
 * counts, and only a token with scope "parent"; parent tokens carry no school,
 * so none is required. Per-child rights are still checked by the backend on
 * every request.
 *
 * The result is serialized into the HTML and RSC payload, so it carries no
 * access token, refresh token or recovery proof (#5005): only what the
 * portal renders from. Parent API calls go through BFF routes that read the
 * cookie themselves; code that needs the token calls getSession().
 * Returns null when the cookie is missing, foreign, expired or due for a
 * refresh; the provider then fetches the session in the browser as before.
 */
export async function readParentSessionSnapshot(): Promise<Session | null> {
  const token = await readFreshPortalToken(
    parentAuthConfig.cookies.sessionToken.name,
    "parent",
  );
  if (!token) return null;

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
      scope: token.scope,
    },
  };
}
