"use client";

import { RoleGuard } from "~/components/auth/role-guard";
import { MasterDetailSkeleton } from "~/components/database/master-detail-skeleton";

export default function DatabaseLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <RoleGuard
      variant="adminOnly"
      message="Du verfügst nicht über die notwendigen Berechtigungen, um die Datenverwaltung aufzurufen."
      fallback={<MasterDetailSkeleton />}
    >
      {children}
    </RoleGuard>
  );
}
