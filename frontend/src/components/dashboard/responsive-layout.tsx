"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { Header } from "./header";
import { Sidebar } from "./sidebar";
import { MobileBottomNav } from "./mobile-bottom-nav";

interface ResponsiveLayoutProps {
  readonly children: React.ReactNode;
}

/**
 * @deprecated Use AppShell via (protected)/layout.tsx instead.
 * This component is kept for backward compatibility with tests.
 */
export default function ResponsiveLayout({ children }: ResponsiveLayoutProps) {
  const { data: session, status } = useSession();
  const router = useRouter();

  // Check for invalid session and redirect
  useEffect(() => {
    if (status === "loading") return;

    // If session exists but token is empty, redirect to login
    if (session && !session.user?.token) {
      router.push("/");
    }
  }, [session, status, router]);

  return (
    <div className="relative min-h-screen">
      {/* Header - sticky positioning */}
      <div className="sticky top-0 z-40">
        <Header />
      </div>

      {/* Main content */}
      <div className="flex">
        {/* Desktop sidebar - only visible on md+ screens */}
        <Sidebar className="hidden lg:block" />

        {/* Main content with bottom padding on mobile for bottom navigation */}
        <main className="flex-1 p-4 pb-24 md:p-8 lg:pb-8">{children}</main>
      </div>

      {/* Mobile bottom navigation */}
      <MobileBottomNav />
    </div>
  );
}
