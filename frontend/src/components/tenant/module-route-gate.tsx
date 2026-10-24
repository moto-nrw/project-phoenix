"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

import { getPageTitle } from "~/components/dashboard/header/breadcrumb-utils";
import { ModuleNotBookedState } from "~/components/ui/module-not-booked-state";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { moduleOfPath } from "~/lib/module-register";
import { normalizeTenantPathname } from "~/lib/tenant-path";
import {
  useBookedModules,
  useTenantRoutingModeSafe,
  useTenantSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";

/**
 * Ruft jemand eine Seite eines Moduls auf, das die Schule nicht gebucht hat
 * (Lesezeichen, alter Link, Eintrag mit Schloss), steht statt der Seite der
 * erklärende Zustand (#4894): Titel der Seite, darunter, zu welchem Modul sie
 * gehört und wer es buchen kann. Kein 404, keine Fehlermeldung.
 *
 * Welche Seite zu welchem Modul gehört, sagt das Modul-Register. Solange die
 * Schuldaten noch nicht geladen sind, rendert die Seite: sonst stünde auf
 * jeder Modulseite kurz der Zustand, und das Backend sperrt ohnehin selbst.
 */
export function ModuleRouteGate({
  children,
}: {
  readonly children: ReactNode;
}) {
  const tenant = useTenantSafe()?.tenant;
  const booked = useBookedModules();
  const pathname = normalizeTenantPathname(
    usePathname(),
    useTenantSlugSafe(),
    useTenantRoutingModeSafe(),
  );
  const pageModule = moduleOfPath(pathname);

  if (!tenant || pageModule === null || booked.includes(pageModule)) {
    return <>{children}</>;
  }

  return (
    <TenantPage title={getPageTitle(pathname)}>
      <SectionCard>
        <ModuleNotBookedState module={pageModule} />
      </SectionCard>
    </TenantPage>
  );
}
