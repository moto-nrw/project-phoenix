"use client";

import { TenantPage } from "~/components/ui/tenant-page";

/**
 * Eigene Ladehülle des Nachweisberichts (#3430): die der Objektseite darüber
 * hätte die Form der Mitteilung, nicht die des Berichts.
 */
export default function DeclarationReportLoading() {
  return (
    <TenantPage
      title="Nachweisbericht"
      back
      backHref="/parent-announcements"
      backLabel="Zurück zur Erklärung"
      statsLoading
      loading
      loadingLabel="Nachweisbericht wird geladen…"
    />
  );
}
