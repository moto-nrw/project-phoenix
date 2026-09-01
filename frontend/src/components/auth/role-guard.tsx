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
   * Ohne Angabe entscheidet allein die Rolle.
   */
  readonly permission?: string;
  readonly children: React.ReactNode;
  readonly message?: string;
  /** Rendered while the session loads; pages pass their skeleton to avoid a spinner flash. */
  readonly fallback?: React.ReactNode;
}

export function RoleGuard({
  variant,
  permission,
  children,
  message,
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
  const hasExtraPermission =
    permission !== undefined && hasPermission(session, permission);
  const isAllowed =
    variant === "adminOnly"
      ? isAdmin || hasExtraPermission
      : isCaregiver(session) ||
        hasExtraPermission ||
        (variant === "staffOrAdmin" && isAdmin);

  if (!isAllowed) {
    return <ForbiddenPage message={message} />;
  }

  return <>{children}</>;
}
