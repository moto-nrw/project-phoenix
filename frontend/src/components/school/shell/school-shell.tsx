"use client";

import { Header } from "~/components/dashboard/header";
import { PortalShell } from "~/components/ui/portal-shell";
import { SchoolBottomNav } from "./school-bottom-nav";
import { SchoolSidebar } from "./school-sidebar";

/**
 * Die Hülle des Schul-Portals ("moto schule", #2207).
 *
 * Der Rahmen kommt aus der geteilten `PortalShell`; hier steht nur, was das
 * Schul-Portal von den anderen unterscheidet: seine eigene Navigation. Die
 * Navigation eines Portals gehört zu diesem Portal, nicht als Sonderzweig in
 * die Mitarbeiter-Navigation — die Eltern-App hat das mit `ParentShell`
 * vorgemacht.
 *
 * Die Kopfzeile bleibt geteilt und auf jeder Breite sichtbar — sie trägt
 * den Seitentitel und das Profil-Menü mit "Abmelden", also den einzigen
 * Weg aus dem Portal heraus.
 */
export function SchoolShell({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <PortalShell
      header={<Header />}
      sidebar={<SchoolSidebar />}
      bottomNav={<SchoolBottomNav />}
    >
      {children}
    </PortalShell>
  );
}
