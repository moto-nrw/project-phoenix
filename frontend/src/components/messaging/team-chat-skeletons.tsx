"use client";

import { Skeleton } from "~/components/ui/skeleton";

/** Data-region skeleton for the chat window: header line plus a few bubbles. */
export function TeamThreadSkeleton({
  header = true,
}: {
  readonly header?: boolean;
}) {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Verlauf wird geladen"
      data-testid="team-thread-skeleton"
    >
      {header && (
        <div className="mb-4 space-y-2">
          <Skeleton shape="round" className="h-5 w-40" />
        </div>
      )}
      <div className="space-y-3">
        <Skeleton shape="card" className="h-14 w-3/5" />
        <Skeleton shape="card" className="ml-auto h-14 w-2/5" />
        <Skeleton shape="card" className="h-14 w-1/2" />
      </div>
    </div>
  );
}
