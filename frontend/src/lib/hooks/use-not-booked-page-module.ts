"use client";

import { usePathname } from "next/navigation";

import { moduleOfPath } from "~/lib/module-register";
import type { ModuleKey } from "~/lib/modules";
import { normalizeTenantPathname } from "~/lib/tenant-path";
import {
  useBookedModules,
  useTenantRoutingModeSafe,
  useTenantSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";

/**
 * Das nicht gebuchte Modul der aktuellen Seite (#4894), sonst `null`.
 *
 * Welche Seite zu welchem Modul gehört, sagt das Modul-Register. Solange die
 * Schuldaten noch nicht geladen sind, gilt die Seite als offen: sonst stünde
 * auf jeder Modulseite kurz die Erklärung, und das Backend sperrt ohnehin
 * selbst. Außerhalb des Mandanten (kein TenantProvider) gibt es nichts zu
 * prüfen.
 */
export function useNotBookedPageModule(): ModuleKey | null {
  const tenant = useTenantSafe()?.tenant;
  const booked = useBookedModules();
  const pathname = normalizeTenantPathname(
    usePathname() ?? "/",
    useTenantSlugSafe(),
    useTenantRoutingModeSafe(),
  );
  if (!tenant) return null;
  const pageModule = moduleOfPath(pathname);
  return pageModule !== null && !booked.includes(pageModule)
    ? pageModule
    : null;
}
