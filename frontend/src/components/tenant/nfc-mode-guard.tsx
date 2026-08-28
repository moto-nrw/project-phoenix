"use client";

import { useNFCEnabled } from "~/lib/tenant-context";
import { FeatureDisabledPage } from "./feature-disabled-page";

/**
 * NfcModeGuard — renders the "Funktion ausgeschaltet" page when the tenant
 * does not use NFC (#2624; previously notFound()).
 *
 * Use at route boundaries for classic NFC-only surfaces such as device
 * management and the legacy activities catalog. Navigation should already
 * hide those entries; this guards direct URL entry as well.
 */
export function NfcModeGuard({ children }: { children: React.ReactNode }) {
  const nfcEnabled = useNFCEnabled();
  if (!nfcEnabled) {
    return <FeatureDisabledPage />;
  }
  return <>{children}</>;
}
