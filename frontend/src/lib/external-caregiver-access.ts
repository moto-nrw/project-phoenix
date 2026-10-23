import type { Session } from "next-auth";
import { hasPermission } from "~/lib/auth-utils";

/**
 * Whether the session may give an external person the access „Nur ansehen“
 * (ADR 0088). It invites into the role „Gast“, and granting a role needs
 * `users:manage`; with only `users:create` the backend refuses it.
 * `hasPermission` honours the admin wildcards.
 */
export function canGrantExternalViewAccess(session: Session | null): boolean {
  return hasPermission(session, "users:manage");
}
