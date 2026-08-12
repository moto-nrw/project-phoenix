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
 * Match a single permission component (resource or action) against the value a
 * required permission demands. Mirrors the backend's `matchesPattern`
 * (backend/auth/authorize/permission.go): exact match, a bare `*`, or a
 * prefix wildcard like `group*`.
 */
function matchesPattern(pattern: string, required: string): boolean {
  if (pattern === required || pattern === "*") return true;
  if (pattern.endsWith("*")) {
    return required.startsWith(pattern.slice(0, -1));
  }
  return false;
}

/**
 * Check if the user holds a specific permission (e.g. "groups:read").
 *
 * This is a faithful port of the backend authorizer
 * (backend/auth/authorize/permission.go `hasPermission`), so a client-side
 * capability check can never silently diverge from what the backend will
 * actually allow. It honours the same wildcards the backend grants:
 *   - the admin wildcards `admin:*` / `*:*` (grant everything),
 *   - resource/action wildcards such as `groups:*` or `groups*`.
 *
 * Permissions are resolved server-side and carried in the JWT. Use this to
 * gate client-side work (e.g. polling) on what the backend will allow, rather
 * than firing requests that are guaranteed to 403.
 */
export function hasPermission(
  session: Session | null,
  permission: string,
): boolean {
  // Empty requirement always matches (mirrors the backend short-circuit).
  if (permission === "") return true;

  const permissions = session?.user?.permissions;
  if (!permissions) return false;

  // Admin wildcard grants everything.
  if (permissions.some((perm) => perm === "admin:*" || perm === "*:*")) {
    return true;
  }

  const requiredParts = permission.split(":");
  if (requiredParts.length !== 2) return false; // invalid format
  const [requiredResource, requiredAction] = requiredParts as [string, string];

  return permissions.some((perm) => {
    const parts = perm.split(":");
    if (parts.length !== 2) return false; // invalid format
    const [resource, action] = parts as [string, string];
    return (
      matchesPattern(resource, requiredResource) &&
      matchesPattern(action, requiredAction)
    );
  });
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
  return hasAnyRole(session, ["user", "teacher"]);
}

/**
 * A pure Lehrkraft account (#1772): holds the lehrkraft role and NOTHING that
 * unlocks other areas — no caregiver flows (user/teacher), no admin, and no
 * guest role (guests keep their own visible pages, e.g. Kindersuche). The one
 * shared definition for sidebar, mobile nav, header badge, and the login
 * redirect; do not re-derive it inline.
 */
export function rolesIndicateLehrkraftOnly(roles: readonly string[]): boolean {
  const held = new Set(roles.map((role) => role.toLowerCase()));
  return (
    held.has("lehrkraft") &&
    !held.has("user") &&
    !held.has("teacher") &&
    !held.has("admin") &&
    !held.has("guest")
  );
}

export function isLehrkraftOnly(session: Session | null): boolean {
  return rolesIndicateLehrkraftOnly(session?.user?.roles ?? []);
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
