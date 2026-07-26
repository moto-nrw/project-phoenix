"use client";

import { notFound } from "next/navigation";
import { useDisplayEnabled } from "~/lib/tenant-context";

/**
 * DisplayModeGuard — triggers Next.js 404 when the tenant has not enabled
 * the Info-Point Dashboard (display.enabled, opt-in and off by default).
 *
 * Use at the /info-displays route boundary. Navigation already hides the
 * sidebar entry; this guards direct URL entry as well.
 */
export function DisplayModeGuard({ children }: { children: React.ReactNode }) {
  const displayEnabled = useDisplayEnabled();
  if (!displayEnabled) {
    notFound();
  }
  return <>{children}</>;
}
