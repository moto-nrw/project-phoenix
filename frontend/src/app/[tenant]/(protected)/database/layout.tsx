"use client";

import { RoleGuard } from "~/components/auth/role-guard";
import { DatabaseSectionSkeleton } from "./section-skeleton";

export default function DatabaseLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <RoleGuard
      variant="adminOnly"
      message="Du verfügst nicht über die notwendigen Berechtigungen, um die Datenverwaltung aufzurufen."
      fallback={<DatabaseSectionSkeleton />}
    >
      {children}
    </RoleGuard>
  );
}
