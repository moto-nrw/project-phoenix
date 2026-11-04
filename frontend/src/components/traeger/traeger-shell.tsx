"use client";

import { Header } from "~/components/dashboard/header";
import { PortalShell } from "~/components/ui/portal-shell";
import { TraegerBottomNav } from "./shell/traeger-bottom-nav";
import { TraegerSidebar } from "./shell/traeger-sidebar";
import { TraegerTestSystemNotice } from "./traeger-test-system-notice";

/**
 * Die Hülle des Träger-Portals ("moto träger", #4649).
 *
 * Gebaut wie die Hülle des Schul-Portals (#5145): den Rahmen liefert die
 * geteilte `PortalShell`, die Kopfzeile mit Seitentitel, Hilfe und
 * Profil-Menü („Abmelden") der geteilte `Header`, die Seiten stehen in der
 * Seitenleiste und auf dem Telefon in der unteren Leiste. Welche Seiten eine
 * Person sieht, entscheiden ihre Rechte (`traeger-nav-items.ts`).
 *
 * Die Kopfzeile bleibt auf jeder Breite sichtbar; sie trägt den einzigen
 * Weg aus dem Portal heraus.
 */
export function TraegerShell({
  children,
  testSystem = false,
}: {
  readonly children: React.ReactNode;
  /** Ein Test-Träger (#4658) bekommt über der Kopfzeile einen Hinweis. */
  readonly testSystem?: boolean;
}) {
  return (
    <PortalShell
      header={<Header />}
      topLayer={testSystem ? <TraegerTestSystemNotice /> : null}
      sidebar={<TraegerSidebar />}
      bottomNav={<TraegerBottomNav />}
    >
      {children}
    </PortalShell>
  );
}
