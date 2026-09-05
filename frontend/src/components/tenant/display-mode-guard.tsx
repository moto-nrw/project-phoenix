"use client";

import { FeatureOffPage } from "~/components/ui/feature-off-page";
import { useDisplayEnabled } from "~/lib/tenant-context";

/**
 * DisplayModeGuard — triggers Next.js 404 when the tenant has not enabled
 * the Info-Point Dashboard (display.enabled, opt-in and off by default).
 *
 * Use at the /info-displays route boundary. Navigation already hides the
 * sidebar entry; this guards direct URL entry as well.
 */
export function DisplayModeGuard({
  children,
  title = "Info-Displays",
}: {
  children: React.ReactNode;
  title?: string;
}) {
  const displayEnabled = useDisplayEnabled();
  if (!displayEnabled) {
    // Anders als NFC ist diese Einstellung nicht operator-vorbehalten — die
    // Schule kann sie selbst einschalten. Der Satz nennt deshalb die
    // Einstellungen und nicht moto.
    return (
      <FeatureOffPage
        title={title}
        description="Info-Displays zeigen Anwesenheit auf einem großen Bildschirm im Eingangsbereich."
        whoCanEnable="Ihre Leitung kann sie in den Einstellungen einschalten."
      />
    );
  }
  return <>{children}</>;
}
