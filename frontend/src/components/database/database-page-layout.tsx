"use client";

import type { ReactNode } from "react";
import { ResponsiveLayout } from "~/components/dashboard";
import { Loading } from "~/components/ui/loading";
import { MobileBackButton } from "~/components/ui/mobile-back-button";

interface DatabasePageLayoutProps {
  /** Whether the page is in a loading state */
  loading: boolean;
  /** Whether the auth session is loading */
  sessionLoading: boolean;
  /** Page content to render when not loading */
  children: ReactNode;
  /** Optional className for the content wrapper */
  className?: string;
}

/**
 * Shared layout wrapper for database management pages.
 * Handles loading states, responsive layout, and mobile back button.
 *
 * Extracted to eliminate code duplication across database pages.
 */
export function DatabasePageLayout({
  loading,
  sessionLoading,
  children,
  className = "w-full",
}: DatabasePageLayoutProps) {
  if (sessionLoading || loading) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className={className}>
        <MobileBackButton />
        {children}
      </div>
    </ResponsiveLayout>
  );
}
