"use client";

import { Header } from "~/components/dashboard/header";
import { PortalShell } from "~/components/ui/portal-shell";

/**
 * Die Hülle des Caterer-Portals (ADR 0077).
 *
 * Den Rahmen liefert die geteilte `PortalShell`, die Kopfzeile mit Profil-Menü
 * und „Abmelden“ der geteilte `Header`. Das Portal hat nur die eine Seite
 * „Bestellungen“, deshalb gibt es keine Seitennavigation und keine mobile
 * Leiste.
 */
export function CatererShell({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <PortalShell header={<Header />} sidebar={null} bottomNav={null}>
      {children}
    </PortalShell>
  );
}
