"use client";

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
 */
export function AppShell({ children }: AppShellProps) {
  return (
    <div className="relative min-h-screen">
      <div
        className="moto-dotted-background moto-dotted-background--app-fixed moto-dotted-background--fullscreen pointer-events-none z-0"
        aria-hidden="true"
      />
      <div
        className="pointer-events-none fixed inset-x-0 top-0 z-30 h-14 bg-white/95 backdrop-blur-md lg:hidden"
        aria-hidden="true"
      />
      {/* Header - sticky positioning */}
      <div className="sticky top-0 z-40">
        <Header />
      </div>

      {/* Main content */}
      <div className="relative z-10 flex">
        {/* Desktop sidebar - only visible on lg+ screens */}
        <Sidebar className="hidden lg:block" />

        {/* Main content with bottom padding on mobile for bottom navigation */}
        <main className="min-w-0 flex-1 p-4 pb-[calc(7rem+env(safe-area-inset-bottom))] md:p-8 md:pb-[calc(7rem+env(safe-area-inset-bottom))] lg:pb-8">
          <div className="relative z-10">{children}</div>
        </main>
      </div>

      {/* Mobile bottom navigation */}
      <MobileBottomNav />
    </div>
  );
}
