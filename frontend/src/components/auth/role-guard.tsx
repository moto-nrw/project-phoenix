"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import {
  hasEffectiveAdminScope,
  hasPermission,
  isCaregiver,
} from "~/lib/auth-utils";
import { Loading } from "~/components/ui/loading";
import { ForbiddenPage } from "~/components/ui/forbidden-page";

interface RoleGuardProps {
  readonly variant: "adminOnly" | "staffOnly" | "staffOrAdmin";
  /**
   * Berechtigung, die den Bereich zusätzlich zur Rolle öffnet (#2906).
   * Eine Liste öffnet, sobald eine der Berechtigungen vorliegt. Ohne
   * Angabe entscheidet allein die Rolle.
   */
  readonly permission?: string | readonly string[];
  readonly children: React.ReactNode;
  readonly message?: string;
  /** Die umgebende Seite hat bereits ein TenantPage-Gerüst. */
  readonly embedded?: boolean;
  /** Rendered while the session loads; pages pass their skeleton to avoid a spinner flash. */
  readonly fallback?: React.ReactNode;
}

export function RoleGuard({
  variant,
  permission,
  children,
  message,
  embedded = false,
  fallback,
}: RoleGuardProps) {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  if (status === "loading") {
    return <>{fallback ?? <Loading fullPage={false} />}</>;
  }

  const isAdmin = hasEffectiveAdminScope(session);
  const requiredPermissions =
    typeof permission === "string" ? [permission] : (permission ?? []);
  const hasExtraPermission = requiredPermissions.some((p) =>
    hasPermission(session, p),
  );
  const isAllowed =
    variant === "adminOnly"
      ? isAdmin || hasExtraPermission
      : isCaregiver(session) ||
        hasExtraPermission ||
        (variant === "staffOrAdmin" && isAdmin);

  if (!isAllowed) {
    return <ForbiddenPage message={message} embedded={embedded} />;
  }

  return <>{children}</>;
}
