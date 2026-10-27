import type { Session } from "next-auth";

/**
 * Whether a signed-in session still grants access. The browser never sees
 * a token (#5005): BFF routes read it from the session cookie themselves.
 * A session whose access token is gone (refresh failed or expired) always
 * carries an error, so the error is the signal.
 */
export function hasSessionAccess(
  session: Session | null | undefined,
): session is Session {
  return session?.user !== undefined && session.error === undefined;
}

/**
 * Identifies whose data a session shows: account, school, scope and rights.
 * Providers that once used the access token as their change signal key on
 * this instead (#5005); a token rotation that changes none of it reloads
 * nothing. Null while the session grants no access.
 */
export function sessionIdentityKey(
  session: Session | null | undefined,
): string | null {
  if (!hasSessionAccess(session)) return null;
  const { id, tenantId, scope, roles, permissions } = session.user;
  return JSON.stringify([
    id,
    tenantId ?? null,
    scope ?? "",
    roles ?? [],
    permissions ?? [],
  ]);
}
