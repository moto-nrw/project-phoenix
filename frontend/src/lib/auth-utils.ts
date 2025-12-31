import type { Session } from "next-auth";

/**
 * Check if the user has a specific role
 */
export function hasRole(session: Session | null, role: string): boolean {
  return session?.user?.roles?.includes(role) ?? false;
}

/**
 * Check if the user is an admin
 * Uses the static isAdmin flag from JWT for performance
 */
export function isAdmin(session: Session | null): boolean {
  return session?.user?.isAdmin ?? false;
}

/**
 * Check if the user is a teacher
 * Uses the static isTeacher flag from JWT for performance
 */
export function isTeacher(session: Session | null): boolean {
  return session?.user?.isTeacher ?? false;
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
