"use client";

import { TenantPage } from "~/components/ui/tenant-page";
import { TeamChatSkeleton } from "./page-skeleton";

/**
 * Route-level loading UI: das Seitengerüst mit Titel rendert sofort, nur die
 * Statuszeile und die Liste skelettieren. Suche und Filter fehlen hier
 * bewusst, es gibt noch keinen Seitenzustand, den sie bedienen könnten.
 */
export default function TeamChatLoading() {
  return (
    <TenantPage title="Team-Chat" statsLoading>
      <TeamChatSkeleton />
    </TenantPage>
  );
}
