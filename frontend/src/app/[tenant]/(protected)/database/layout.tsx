"use client";

import { usePathname } from "next/navigation";
import { RoleGuard } from "~/components/auth/role-guard";
import { Loading } from "~/components/ui/loading";

/**
 * Berechtigungen, die den Bereich neben der Leitungsrolle öffnen (#2906).
 * Die Personalseite gehört zu den beiden Personal-Berechtigungen, mit denen
 * das Backend auch GET /api/staff beantwortet. Der Personal-Import dahinter
 * hängt wie POST /api/import/teachers und die Vorlage an users:create, der
 * Eröffnungssalden-Import an der Zeitwirtschaft. Ohne diese Rechte antwortet
 * das Backend ohnehin mit 403.
 */
const PERSONNEL_PAGE_PERMISSIONS = [
  "staff:manage",
  "staff:stammdaten",
] as const;

function permissionForPath(
  pathname: string | null,
): string | readonly string[] | undefined {
  if (pathname === null) return undefined;
  if (pathname.endsWith("/database/personal/opening-balances")) {
    return "time_tracking:manage";
  }
  if (pathname.endsWith("/database/personal/import")) {
    return "users:create";
  }
  if (pathname.endsWith("/database/personal")) {
    return PERSONNEL_PAGE_PERMISSIONS;
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
