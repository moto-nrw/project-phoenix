"use client";

import { notFound } from "next/navigation";
import { useNFCEnabled } from "~/lib/tenant-context";

/**
 * NfcModeGuard — triggers Next.js 404 when the tenant does not use NFC.
 *
 * Use at route boundaries for classic NFC-only surfaces such as device
 * management and the legacy activities catalog. Navigation should already
 * hide those entries; this guards direct URL entry as well.
 */
export function NfcModeGuard({ children }: { children: React.ReactNode }) {
  const nfcEnabled = useNFCEnabled();
  if (!nfcEnabled) {
    notFound();
  }
  return <>{children}</>;
}
