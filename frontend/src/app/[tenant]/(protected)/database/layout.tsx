"use client";

import { usePathname } from "next/navigation";
import { RoleGuard } from "~/components/auth/role-guard";
import { Loading } from "~/components/ui/loading";
import {
  DATABASE_PAGE_PERMISSIONS,
  DATABASE_SECTION,
  databasePagePermissions,
  matchesPathPrefix,
} from "~/lib/section-navigation";

/**
 * Berechtigungen, die den Bereich neben dem Adminzuschnitt öffnen (#2906,
 * #3469): jede Seite trägt das Recht ihrer Route, aus demselben Katalog wie
 * Seitenleiste, Mehr-Menü und Hub (`DATABASE_PAGE_PERMISSIONS`). Ohne das
 * Recht stünde die Seite offen und jede Aktion darin liefe in ein 403.
 *
 * Zwei Unterseiten des Personals weichen von ihrer Seite ab: der
 * Personal-Import hängt wie POST /api/import/teachers und die Vorlage an
 * users:create, der Eröffnungssalden-Import an der Zeitwirtschaft. Der Hub
 * selbst öffnet für jedes Recht einer seiner Seiten; er zeigt dann nur die
 * Kacheln, die die Person öffnen darf.
 */
const HUB_PERMISSIONS = [
  ...new Set(
    Object.keys(DATABASE_PAGE_PERMISSIONS).flatMap(databasePagePermissions),
  ),
];

function permissionForPath(
  pathname: string | null,
): string | readonly string[] | undefined {
  if (pathname === null) return undefined;
  // Der Pfad kann das Tenant-Präfix tragen; ab „/database“ ist er der Katalogpfad.
  const start = pathname.indexOf(DATABASE_SECTION.href);
  if (start < 0) return undefined;
  const path = pathname.slice(start);
  if (path === DATABASE_SECTION.href) return HUB_PERMISSIONS;
  if (matchesPathPrefix(path, "/database/personal/opening-balances")) {
    return "time_tracking:manage";
  }
  if (matchesPathPrefix(path, "/database/personal/import")) {
    return "users:create";
  }
  const page = Object.keys(DATABASE_PAGE_PERMISSIONS).find((href) =>
    matchesPathPrefix(path, href),
  );
  return page === undefined ? undefined : DATABASE_PAGE_PERMISSIONS[page];
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
