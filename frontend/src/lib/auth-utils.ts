import type { Session } from "next-auth";

/**
 * Check if the user has a specific role
 */
export function hasRole(session: Session | null, role: string): boolean {
  return session?.user?.roles?.includes(role) ?? false;
}

export function hasAnyRole(
  session: Session | null,
  roles: readonly string[],
): boolean {
  return roles.some((role) => hasRole(session, role));
}

/**
 * Check if the user is an admin
 */
export function isAdmin(session: Session | null): boolean {
  return hasRole(session, "admin");
}

/**
 * Check if the user can access caregiver workflows.
 */
export function isCaregiver(session: Session | null): boolean {
  return hasRole(session, "user");
}

/**
 * Check if the user is authenticated
 */
export function isAuthenticated(session: Session | null): boolean {
  return !!session?.user?.token;
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
 */
export function requiresReauth(session: Session | null): boolean {
  return session?.error === "RefreshTokenExpired";
}
