"use client";

import { BackButton } from "~/components/ui/back-button";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { ThreadSkeleton, ThreadComposerSkeleton } from "./page-skeleton";

/**
 * Route-level loading UI: das Seitengerüst rendert sofort, nur die
 * Statuszeile, der Verlauf und das Eingabefeld skelettieren.
 */
export default function MessageThreadLoading() {
  return (
    <TenantPage title="Unterhaltung" statsLoading>
      <BackButton referrer="/messages" />

      <div className="flex min-h-[20rem] w-full flex-col overflow-hidden">
        <SectionCard
          className="flex min-h-0 flex-1 flex-col"
          bodyClassName="flex min-h-0 flex-1 flex-col"
        >
          <ThreadSkeleton />
          <ThreadComposerSkeleton />
        </SectionCard>
      </div>
    </TenantPage>
  );
}
