import type { Session } from "next-auth";

/**
 * Whether a signed-in session still grants access. The session callback
 * strips a session whose refresh failed down to an empty token. A portal
 * layout's server snapshot carries no token at all (#4990, #4991, #5005):
 * the server already checked that cookie, and BFF routes read the cookie
 * themselves, so a missing token does not mean signed out.
 */
export function hasSessionAccess(
  session: Session | null | undefined,
): session is Session {
  return session?.user !== undefined && session.user.token !== "";
}
