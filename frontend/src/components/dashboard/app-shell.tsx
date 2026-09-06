"use client";

import { PortalShell } from "~/components/ui/portal-shell";
import { StaffPreviewBanner } from "~/components/staff-preview/staff-preview-banner";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
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
 *
 * Mitarbeiter-Vorschau (#2893): während einer aktiven Vorschau liegt ein
 * fester Hinweisstreifen (h-12) über allem. Die gesamte Shell rückt um
 * dieselbe Höhe nach unten, damit der Streifen auf jeder Seite sichtbar
 * bleibt und nichts verdeckt.
 */
export function AppShell({ children }: AppShellProps) {
  const previewActive = useShellAuthSafe()?.isPreview === true;

  return (
    <div className={previewActive ? "pt-12" : undefined}>
      <PortalShell
        header={<Header />}
        headerClassName={
          previewActive
            ? "sticky top-12 z-40 hidden lg:block"
            : "sticky top-0 z-40 hidden lg:block"
        }
        backgroundClassName="moto-dotted-background--full"
        // Flex-Spalte, damit `TenantPage` bis zur Unterkante wächst und
        // seine letzte Fläche den Bildschirm füllt (kein Seitenrest aus
        // nacktem Punktraster unter einer einzeiligen Karte).
        contentClassName="flex flex-1 flex-col"
        topLayer={
          <>
            <StaffPreviewBanner />
            <div
              data-staff-safe-area-top
              className="relative z-10 h-[env(safe-area-inset-top)] bg-transparent lg:hidden"
              aria-hidden="true"
            />
          </>
        }
        // `min-h-0!`: die Seitenleiste bringt ein eigenes `min-h-screen` mit.
        // In der Flex-Zeile der Shell, die ohnehin bis zur Unterkante reicht,
        // schob das jede Seite um die Höhe der Kopfzeile (65 px) unter den
        // Bildschirmrand -- die wachsende letzte Fläche endete dann unter der
        // Kante statt an ihr. Die Zeile bestimmt die Höhe, nicht die Leiste.
        sidebar={<Sidebar className="hidden min-h-0! lg:block" />}
        bottomNav={<MobileBottomNav />}
      >
        {children}
      </PortalShell>
    </div>
  );
}
