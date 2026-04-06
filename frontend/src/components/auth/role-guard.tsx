"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { hasRole, isCaregiver } from "~/lib/auth-utils";
import { Loading } from "~/components/ui/loading";
import { ForbiddenPage } from "~/components/ui/forbidden-page";

interface RoleGuardProps {
  readonly variant: "adminOnly" | "staffOnly";
  readonly children: React.ReactNode;
  readonly message?: string;
}

export function RoleGuard({ variant, children, message }: RoleGuardProps) {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  if (status === "loading") {
    return <Loading fullPage={false} />;
  }

  const isAllowed =
    variant === "adminOnly" ? hasRole(session, "admin") : isCaregiver(session);

  if (!isAllowed) {
    return <ForbiddenPage message={message} />;
  }

  return <>{children}</>;
}
