"use client";

import { TenantPage } from "~/components/ui/tenant-page";

/**
 * Route-level loading UI: dasselbe Gerüst wie die Seite, nur mit Skelett an
 * Statuszeile und Inhalt, damit die Navigation ein durchgehendes Skelett
 * zeigt statt erst der Gruppen-Ladeanzeige und dann dem Seitenskelett.
 */
export default function ActivitiesLoading() {
  return <TenantPage title="Aktivitäten" statsLoading loading />;
}
