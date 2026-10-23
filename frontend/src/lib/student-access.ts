import type { Session } from "next-auth";
import { hasPermission } from "~/lib/auth-utils";

/**
 * Either permission opens the read-only child views (Alle Kinder, the child
 * page, OGS groups). Staff carries `users:read`; the role „Gast“ carries only
 * `students:read` (ADR 0085), which opens the child routes but no parent
 * data, notes, exports or messages. Those stay on `users:read`.
 */
export const STUDENT_READ_PERMISSIONS = [
  "users:read",
  "students:read",
] as const;

/** Whether the session may open the read-only child views. */
export function canReadStudents(session: Session | null): boolean {
  return STUDENT_READ_PERMISSIONS.some((permission) =>
    hasPermission(session, permission),
  );
}

// A session minted before permission claims keeps the action; the backend
// still decides. An explicit permission list is authoritative.
function grantedOrUnclaimed(session: Session | null, permission: string) {
  if (!session?.user) return false;
  const permissions = session.user.permissions;
  return !Array.isArray(permissions) || hasPermission(session, permission);
}

/**
 * Whether the session may check children in and out on the web (`users:checkin`,
 * the gate of POST /api/students/{id}/school-checkin). The role „Gast“ lacks
 * it (ADR 0085), so the check-in mode stays away instead of failing on every
 * tap.
 */
export function canCheckInChildren(session: Session | null): boolean {
  return grantedOrUnclaimed(session, "users:checkin");
}

/**
 * Whether the session may export the child list (POST /api/students/export,
 * `users:read`). The role „Gast“ reads children without exports (ADR 0085).
 */
export function canExportStudents(session: Session | null): boolean {
  return grantedOrUnclaimed(session, "users:read");
}
