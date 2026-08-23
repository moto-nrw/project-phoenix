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
 * the staff portal's own navigation. The extra top layer is the white strip
 * that covers the page behind the mobile header.
 */
export function AppShell({ children }: AppShellProps) {
  return (
    <PortalShell
      header={<Header />}
      topLayer={
        <div
          className="pointer-events-none fixed inset-x-0 top-0 z-30 h-14 bg-white/95 backdrop-blur-md lg:hidden"
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
