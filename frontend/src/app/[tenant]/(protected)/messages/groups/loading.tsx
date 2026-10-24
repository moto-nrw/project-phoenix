"use client";

import { TenantPage } from "~/components/ui/tenant-page";
import { MessagesSkeleton } from "../page-skeleton";

/**
 * Route-level loading UI der Gruppennachrichten (#4784): das Seitengerüst
 * rendert sofort, nur die Statuszeile und die Liste skelettieren.
 */
export default function GroupMessagesLoading() {
  return (
    <TenantPage title="Gruppennachrichten" statsLoading>
      <MessagesSkeleton />
    </TenantPage>
  );
}
