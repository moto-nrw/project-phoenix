"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { isAdmin } from "~/lib/auth-utils";
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

  // The tenant shell only has admin and user (Betreuer) accounts today.
  // Guardians use a separate account table and cannot log in here.
  // staffOnly therefore means !isAdmin — if a third role enters the
  // tenant shell, this check needs revisiting.
  const isAllowed =
    variant === "adminOnly" ? isAdmin(session) : !isAdmin(session);

  if (!isAllowed) {
    return <ForbiddenPage message={message} />;
  }

  return <>{children}</>;
}
