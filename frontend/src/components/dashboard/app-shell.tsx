"use client";

import { PortalShell } from "~/components/ui/portal-shell";
import { Header } from "./header";
import { Sidebar } from "./sidebar";
import { MobileBottomNav } from "./mobile-bottom-nav";

interface AppShellProps {
  readonly children: React.ReactNode;
}

/**
 * Persistent application shell rendered once in the (protected) layout.
 * Header, Sidebar, and MobileBottomNav stay mounted across navigations,
 * only the children (page content) swap.
 *
 * The frame itself lives in the shared `PortalShell`; this file only names
 * the staff portal's own navigation.
 *
 * Unter lg gibt es KEINE Kopfzeile — dieselbe Entscheidung wie in der
 * Eltern-App: die „Bereich › Seite"-Zeile dort war eine zweite Überschrift
 * über der Kopfkarte, die den Seitennamen ohnehin trägt. Statt ihrer bleibt
 * nur ein transparenter Safe-Area-Streifen; Profil und Abmelden wohnen im
 * „Mehr"-Menü der unteren Leiste.
 */
export function AppShell({ children }: AppShellProps) {
  return (
    <PortalShell
      header={<Header />}
      headerClassName="sticky top-0 z-40 hidden lg:block"
      backgroundClassName="moto-dotted-background--full"
      topLayer={
        <div
          data-staff-safe-area-top
          className="relative z-10 h-[env(safe-area-inset-top)] min-h-8 bg-transparent lg:hidden"
          aria-hidden="true"
        />
      }
      sidebar={<Sidebar className="hidden lg:block" />}
      bottomNav={<MobileBottomNav />}
    >
      {children}
    </PortalShell>
  );
}
