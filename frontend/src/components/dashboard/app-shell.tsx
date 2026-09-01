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
 * the staff portal's own navigation. The extra top layer is the white strip
 * that covers the page behind the mobile header.
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
          previewActive ? "sticky top-12 z-40" : "sticky top-0 z-40"
        }
        topLayer={
          <>
            <StaffPreviewBanner />
            <div
              className={`pointer-events-none fixed inset-x-0 z-30 h-14 bg-white/95 backdrop-blur-md lg:hidden ${
                previewActive ? "top-12" : "top-0"
              }`}
              aria-hidden="true"
            />
          </>
        }
        sidebar={<Sidebar className="hidden lg:block" />}
        bottomNav={<MobileBottomNav />}
      >
        {children}
      </PortalShell>
    </div>
  );
}
