import type { Session } from "next-auth";
import { hasPermission } from "~/lib/auth-utils";

/**
 * Either permission opens the read-only child views (Alle Kinder, the child
 * page, OGS groups). Staff carries `users:read`; the role „Gast“ carries only
 * `students:read` (ADR 0083), which opens the child routes but no parent
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
