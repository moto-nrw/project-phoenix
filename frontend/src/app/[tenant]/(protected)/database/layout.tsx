"use client";

import { usePathname } from "next/navigation";
import { RoleGuard } from "~/components/auth/role-guard";
import { Loading } from "~/components/ui/loading";

/**
 * Berechtigung, die den Bereich neben der Leitungsrolle öffnet (#2906).
 * Die Personalseite gehört zu staff:manage, der Eröffnungssalden-Import
 * dahinter zur Zeitwirtschaft — ohne diese Rechte antwortet das Backend
 * ohnehin mit 403.
 */
function permissionForPath(pathname: string | null): string | undefined {
  if (pathname === null) return undefined;
  if (pathname.endsWith("/database/personal/opening-balances")) {
    return "time_tracking:manage";
  }
  if (pathname.endsWith("/database/personal")) {
    return "staff:manage";
  }
  return undefined;
}

export default function DatabaseLayout({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const pathname = usePathname();

  return (
    <RoleGuard
      variant="adminOnly"
      permission={permissionForPath(pathname ?? null)}
      message="Du verfügst nicht über die notwendigen Berechtigungen, um die Datenverwaltung aufzurufen."
      fallback={
        <Loading message="Berechtigungen werden geprüft…" fullPage={false} />
      }
    >
      {children}
    </RoleGuard>
  );
}
