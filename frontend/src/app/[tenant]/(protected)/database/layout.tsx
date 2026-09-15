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

/**
 * Die Stammdaten-Kataloge (#3114) hängen an dem Recht, mit dem das Backend
 * ihre Schreibzugriffe beantwortet — sonst stünde die Seite offen und jede
 * Aktion darin liefe in ein 403.
 */
const CATALOG_PAGE_PERMISSIONS: Readonly<Record<string, string>> = {
  "/database/categories": "activities:manage_categories",
  "/database/planning-tracks": "schedules:manage",
  "/database/shift-types": "time_tracking:manage",
  "/database/absence-types": "time_tracking:manage",
};

function permissionForPath(
  pathname: string | null,
): string | readonly string[] | undefined {
  if (pathname === null) return undefined;
  for (const [suffix, permission] of Object.entries(CATALOG_PAGE_PERMISSIONS)) {
    if (pathname.endsWith(suffix)) return permission;
  }
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
      message="Sie verfügen nicht über die notwendigen Berechtigungen, um die Datenverwaltung aufzurufen."
      fallback={
        <Loading message="Berechtigungen werden geprüft…" fullPage={false} />
      }
    >
      {children}
    </RoleGuard>
  );
}
