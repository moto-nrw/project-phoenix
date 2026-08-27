"use client";

import { BackButton } from "~/components/ui/back-button";
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
        <div className="moto-content-surface flex min-h-0 flex-1 flex-col rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
          <ThreadSkeleton />
          <ThreadComposerSkeleton />
        </div>
      </div>
    </TenantPage>
  );
}
