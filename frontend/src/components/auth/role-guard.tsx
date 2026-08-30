"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { hasRole, isCaregiver } from "~/lib/auth-utils";
import { Loading } from "~/components/ui/loading";
import { ForbiddenPage } from "~/components/ui/forbidden-page";

interface RoleGuardProps {
  readonly variant: "adminOnly" | "staffOnly" | "staffOrAdmin";
  readonly children: React.ReactNode;
  readonly message?: string;
  /** Rendered while the session loads; pages pass their skeleton to avoid a spinner flash. */
  readonly fallback?: React.ReactNode;
}

export function RoleGuard({
  variant,
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

  const isAdmin = hasRole(session, "admin");
  const isAllowed =
    variant === "adminOnly"
      ? isAdmin
      : isCaregiver(session) || (variant === "staffOrAdmin" && isAdmin);

  if (!isAllowed) {
    return <ForbiddenPage message={message} />;
  }

  return <>{children}</>;
}
