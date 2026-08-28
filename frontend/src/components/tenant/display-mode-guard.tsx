"use client";

import { useDisplayEnabled } from "~/lib/tenant-context";
import { FeatureDisabledPage } from "./feature-disabled-page";

/**
 * DisplayModeGuard — renders the "Funktion ausgeschaltet" page when the
 * tenant has not enabled the Info-Point Dashboard (display.enabled, opt-in
 * and off by default). Previously notFound() (#2624).
 *
 * Use at the /info-displays route boundary. Navigation already hides the
 * sidebar entry; this guards direct URL entry as well.
 */
export function DisplayModeGuard({ children }: { children: React.ReactNode }) {
  const displayEnabled = useDisplayEnabled();
  if (!displayEnabled) {
    return <FeatureDisabledPage />;
  }
  return <>{children}</>;
}
