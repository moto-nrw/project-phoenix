"use client";

import { Header } from "~/components/dashboard/header";
import { SchoolBottomNav } from "./school-bottom-nav";
import { SchoolSidebar } from "./school-sidebar";

/**
 * Die Hülle des Schul-Portals ("moto schule", #2207).
 *
 * Löst die geteilte AppShell des Personal-Portals ab, wie es die
 * Eltern-App mit `ParentShell` bereits getan hat: die Navigation eines
 * Portals gehört zu diesem Portal, nicht als Sonderzweig in die
 * Mitarbeiter-Navigation.
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
    <div className="relative min-h-screen">
      <div
        className="moto-dotted-background moto-dotted-background--app-fixed moto-dotted-background--fullscreen pointer-events-none z-0"
        aria-hidden="true"
      />

      <div className="sticky top-0 z-40">
        <Header />
      </div>

      <div className="relative z-10 flex">
        <SchoolSidebar />

        <main className="min-w-0 flex-1 p-4 pb-[calc(7rem+env(safe-area-inset-bottom))] md:p-8 md:pb-[calc(7rem+env(safe-area-inset-bottom))] lg:pb-8">
          <div className="relative z-10">{children}</div>
        </main>
      </div>

      <SchoolBottomNav />
    </div>
  );
}
