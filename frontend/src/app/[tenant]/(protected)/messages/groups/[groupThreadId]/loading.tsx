"use client";

import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  ThreadComposerSkeleton,
  ThreadSkeleton,
} from "../../[threadId]/page-skeleton";

/**
 * Route-level loading UI einer Gruppennachricht (#4784): das Seitengerüst
 * rendert sofort, nur die Statuszeile, der Verlauf und das Eingabefeld
 * skelettieren.
 */
export default function GroupMessageLoading() {
  return (
    <TenantPage
      title="Gruppennachricht"
      statsLoading
      back
      backHref="/messages/groups"
      backLabel="Zurück zu Gruppennachrichten"
    >
      <SectionCard>
        <ThreadSkeleton />
        <ThreadComposerSkeleton />
      </SectionCard>
    </TenantPage>
  );
}
