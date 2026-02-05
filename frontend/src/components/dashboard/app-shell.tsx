"use client";

import { Header } from "./header";
import { Sidebar } from "./sidebar";
import { MobileBottomNav } from "./mobile-bottom-nav";

interface AppShellProps {
  readonly children: React.ReactNode;
}

/**
 * Persistent application shell rendered once in the (protected) layout.
 * Header, Sidebar, and MobileBottomNav stay mounted across navigations —
 * only the children (page content) swap.
 */
export function AppShell({ children }: AppShellProps) {
  return (
    <div className="relative min-h-screen">
      {/* Header - sticky positioning */}
      <div className="sticky top-0 z-40">
        <Header />
      </div>

      {/* Main content */}
      <div className="flex">
        {/* Desktop sidebar - only visible on lg+ screens */}
        <Sidebar className="hidden lg:block" />

        {/* Main content with bottom padding on mobile for bottom navigation */}
        <main className="flex-1 p-4 pb-24 md:p-8 lg:pb-8">{children}</main>
      </div>

      {/* Mobile bottom navigation */}
      <MobileBottomNav />
    </div>
  );
}
