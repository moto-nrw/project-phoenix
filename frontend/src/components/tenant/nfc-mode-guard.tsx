"use client";

import { FeatureOffPage } from "~/components/ui/feature-off-page";
import { useNFCEnabled } from "~/lib/tenant-context";

/**
 * NfcModeGuard — triggers Next.js 404 when the tenant does not use NFC.
 *
 * Use at route boundaries for classic NFC-only surfaces such as device
 * management and the legacy activities catalog. Navigation should already
 * hide those entries; this guards direct URL entry as well.
 */
export function NfcModeGuard({
  children,
  title = "Nicht eingeschaltet",
}: {
  children: React.ReactNode;
  title?: string;
}) {
  const nfcEnabled = useNFCEnabled();
  if (!nfcEnabled) {
    // `attendance.nfc_enabled` ist operator-only: die Schule kann es selbst
    // NICHT einschalten. Der Satz nennt deshalb moto, nicht die Leitung.
    return (
      <FeatureOffPage
        title={title}
        description="Diese Seite gehört zur Anwesenheit über NFC-Geräte. Für Ihre Schule ist sie nicht eingeschaltet."
      />
    );
  }
  return <>{children}</>;
}
