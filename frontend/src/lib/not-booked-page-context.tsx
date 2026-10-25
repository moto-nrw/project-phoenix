"use client";

import { usePathname } from "next/navigation";
import { createContext, useContext, type ReactNode } from "react";

import { moduleOfPath } from "~/lib/module-register";
import type { ModuleKey } from "~/lib/modules";
import { normalizeTenantPathname } from "~/lib/tenant-path";
import {
  useBookedModules,
  useTenantRoutingModeSafe,
  useTenantSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";

const NotBookedPageContext = createContext<ModuleKey | null>(null);

/**
 * Ermittelt für die geschützten Seiten der Mitarbeiter-Oberfläche, ob die
 * aktuelle Seite zu einem Modul gehört, das die Schule nicht gebucht hat
 * (Modul-Register, #4894). `TenantPage` liest das Ergebnis und zeigt dann die
 * Erklärung statt des Inhalts.
 *
 * Der Anbieter sitzt im Layout, die Anzeige im Seitengerüst: so laden reine
 * Weiterleitungsseiten das Gerüst nicht mit, und ein Gerüst ohne Anbieter
 * (Tests, andere Portale) gilt als offen.
 *
 * Solange die Schuldaten noch nicht geladen sind, gilt die Seite als offen:
 * sonst stünde auf jeder Modulseite kurz die Erklärung, und das Backend
 * sperrt ohnehin selbst.
 */
export function NotBookedPageProvider({
  children,
}: {
  readonly children: ReactNode;
}) {
  const tenant = useTenantSafe()?.tenant;
  const booked = useBookedModules();
  const pathname = normalizeTenantPathname(
    usePathname() ?? "/",
    useTenantSlugSafe(),
    useTenantRoutingModeSafe(),
  );
  const pageModule = tenant ? moduleOfPath(pathname) : null;
  const notBooked =
    pageModule !== null && !booked.includes(pageModule) ? pageModule : null;
  return (
    <NotBookedPageContext.Provider value={notBooked}>
      {children}
    </NotBookedPageContext.Provider>
  );
}

/** Das nicht gebuchte Modul der aktuellen Seite, sonst `null`. */
export function useNotBookedPageModule(): ModuleKey | null {
  return useContext(NotBookedPageContext);
}
