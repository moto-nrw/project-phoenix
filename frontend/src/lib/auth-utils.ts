/**
 * Auth utility functions for BetterAuth sessions.
 *
 * BetterAuth uses cookies for session management.
 * For role/permission checks, use the async functions from auth-client.ts instead:
 * - isAdmin() - Check if user is admin
 * - isSupervisor() - Check if user is supervisor
 * - getActiveRole() - Get the user's current role
 */

/**
 * Session type for BetterAuth (compatible interface)
 */
export interface BetterAuthSessionUser {
  id: string;
  email: string;
  name: string | null;
  firstName?: string;
  roles?: string[];
  isAdmin?: boolean;
  isTeacher?: boolean;
}

export interface BetterAuthSession {
  user: BetterAuthSessionUser;
  error?: string;
}

type Session = BetterAuthSession;

/**
 * Check if the user has a specific role
 * Note: For BetterAuth, roles should be fetched via getActiveRole() from auth-client
 */
export function hasRole(session: Session | null, role: string): boolean {
  return session?.user?.roles?.includes(role) ?? false;
}

/**
 * Check if the user is an admin
 * Note: For BetterAuth, use isAdmin() from auth-client instead
 */
export function isAdmin(session: Session | null): boolean {
  return session?.user?.isAdmin ?? false;
}

/**
 * Check if the user is a teacher
 * Note: For BetterAuth, use role checks from auth-client instead
 */
export function isTeacher(session: Session | null): boolean {
  return session?.user?.isTeacher ?? false;
}

/**
 * Check if the user is authenticated
 * BetterAuth: User is authenticated if session exists with user data
 */
export function isAuthenticated(session: Session | null): boolean {
  return !!session?.user;
}

/**
 * Get the user's display name
 */
export function getUserDisplayName(session: Session | null): string {
  if (!session?.user) return "User";

  if (session.user.firstName) {
    return session.user.firstName;
  }

  return session.user.name ?? session.user.email ?? "User";
}

/**
 * Get the user's roles as a comma-separated string
 */
export function getUserRolesDisplay(session: Session | null): string {
  if (!session?.user?.roles || session.user.roles.length === 0) {
    return "No roles";
  }

  return session.user.roles.join(", ");
}

/**
 * Check if the session has an error that requires re-authentication
 * Note: BetterAuth handles session expiration via cookies
 */
export function requiresReauth(session: Session | null): boolean {
  return session?.error === "RefreshTokenExpired";
}
