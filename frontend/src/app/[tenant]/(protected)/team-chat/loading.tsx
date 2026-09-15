"use client";

import { TenantPage } from "~/components/ui/tenant-page";

/**
 * Route-level loading UI: das Seitengeruest mit Titel rendert sofort,
 * Statuszeile und Liste kommen aus den Zustaenden des Geruests. Kein eigenes
 * Seiten-Skelett mehr — die Ladeflaeche sieht ueberall im Portal gleich aus.
 */
export default function TeamChatLoading() {
  return <TenantPage title="Team-Chat" statsLoading loading />;
}
