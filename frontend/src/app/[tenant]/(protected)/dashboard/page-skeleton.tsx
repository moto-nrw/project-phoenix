"use client";

import { TenantPage } from "~/components/ui/tenant-page";
import { getTimeBasedGreeting } from "~/lib/greeting";

/**
 * Ladezustand der Startseite. Er kommt aus dem Seitengeruest selbst
 * (`TenantPage loading`) und nicht mehr aus einem eigenen Seiten-Skelett:
 * dieselbe Kopfkarte, dieselben Platzhalterflaechen wie auf jeder anderen
 * Flaeche des Portals. Der Gruss steht schon hier, damit der Titel beim
 * Eintreffen der Sitzung nur um den Vornamen waechst und nicht wechselt.
 */
export function DashboardSkeleton() {
  return (
    <TenantPage
      title={getTimeBasedGreeting()}
      prominent
      loading
      testId="dashboard-skeleton"
    />
  );
}
