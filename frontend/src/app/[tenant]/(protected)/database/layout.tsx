"use client";

import { RoleGuard } from "~/components/auth/role-guard";
import { Loading } from "~/components/ui/loading";

export default function DatabaseLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <RoleGuard
      variant="adminOnly"
      message="Sie verfügen nicht über die notwendigen Berechtigungen, um die Datenverwaltung aufzurufen."
      fallback={
        <Loading message="Berechtigungen werden geprüft…" fullPage={false} />
      }
    >
      {children}
    </RoleGuard>
  );
}
