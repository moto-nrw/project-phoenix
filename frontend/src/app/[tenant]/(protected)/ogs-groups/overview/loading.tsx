"use client";

import { TenantPage } from "~/components/ui/tenant-page";
import { OGS_GROUPS_LABEL } from "~/lib/ogs-groups-navigation";

/**
 * Eigene Ladegrenze der Auswahlseite: ohne sie erbte sie das Kinderkarten-
 * Skelett der Gruppenseite (`../loading.tsx`).
 */
export default function GroupOverviewLoading() {
  return (
    <TenantPage
      title={OGS_GROUPS_LABEL}
      loading
      loadingLabel="Gruppen werden geladen…"
    />
  );
}
