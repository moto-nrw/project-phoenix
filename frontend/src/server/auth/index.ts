/**
 * Server-side Authentication Helpers for BetterAuth
 *
 * This module provides server-side session access for API routes.
 * It replaces the previous NextAuth.js `auth()` function.
 *
 * Key Differences from NextAuth:
 * - No JWT tokens in session - BetterAuth uses secure HTTP-only cookies
 * - Session is validated by checking the BetterAuth session cookie
 * - Backend requests should forward cookies, not Authorization headers
 */

import { cookies } from "next/headers";

/**
 * Session user type for BetterAuth
 * Note: No `token` property - auth is handled via cookies
 */
export interface SessionUser {
  id: string;
  email: string;
  name: string | null;
}

/**
 * Session type returned by auth()
 * Simplified compared to NextAuth - no tokens, just user info
 */
export interface Session {
  user: SessionUser;
}

/**
 * Get the current session from BetterAuth cookies.
 * This is the server-side equivalent of useSession().
 *
 * @returns Session object if authenticated, null otherwise
 *
 * @example
 * ```ts
 * const session = await auth();
 * if (!session?.user) {
 *   return new Response("Unauthorized", { status: 401 });
 * }
 * ```
 */
export async function auth(): Promise<Session | null> {
  const cookieStore = await cookies();

  // BetterAuth session cookie name
  const sessionCookie = cookieStore.get("better-auth.session_token");

  if (!sessionCookie?.value) {
    return null;
  }

  // For now, we trust the presence of the session cookie
  // The Go backend will validate the session when we forward the cookie
  // We can optionally decode the cookie here if needed

  // Return a minimal session object
  // The actual user info is validated by the backend
  return {
    user: {
      id: "session-active",
      email: "",
      name: null,
    },
  };
}

/**
 * Get all cookies as a header string for forwarding to the backend.
 * Use this instead of extracting tokens.
 *
 * @returns Cookie header string
 *
 * @example
 * ```ts
 * const cookieHeader = await getCookieHeader();
 * await fetch(backendUrl, {
 *   headers: { Cookie: cookieHeader }
 * });
 * ```
 */
export async function getCookieHeader(): Promise<string> {
  const cookieStore = await cookies();
  return cookieStore.toString();
}

/**
 * Check if the user has an active session.
 * Quick check without full session retrieval.
 *
 * @returns true if session cookie exists
 */
export async function hasActiveSession(): Promise<boolean> {
  const session = await auth();
  return session !== null;
}
