"use client";

import { Skeleton } from "~/components/ui/skeleton";

/** Data-region skeleton for the chat window: header line plus a few bubbles. */
export function TeamThreadSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Verlauf wird geladen"
      data-testid="team-thread-skeleton"
    >
      <div className="mb-4 space-y-2">
        <Skeleton className="h-5 w-40 rounded-full" />
      </div>
      <div className="space-y-3">
        <Skeleton className="h-14 w-3/5 rounded-2xl" />
        <Skeleton className="ml-auto h-14 w-2/5 rounded-2xl" />
        <Skeleton className="h-14 w-1/2 rounded-2xl" />
      </div>
    </div>
  );
}
