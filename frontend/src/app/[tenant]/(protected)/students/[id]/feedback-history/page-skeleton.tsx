"use client";

import { Skeleton } from "~/components/ui/skeleton";

// Mirrors the header block, filter-pills row, and the proportion-bar +
// legend + chart card of the real page, so it swaps in without layout shift.
export function FeedbackHistorySkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Feedbackhistorie wird geladen"
      data-testid="feedback-history-skeleton"
      className="mx-auto max-w-7xl"
    >
      <Skeleton className="mb-4 h-9 w-24 rounded-lg" />

      <div className="mb-6 ml-6 space-y-2">
        <Skeleton className="h-8 w-48 rounded" />
        <Skeleton className="h-4 w-32 rounded" />
        <div className="mt-2 flex items-center gap-2">
          <Skeleton className="h-4 w-4 rounded-full" />
          <Skeleton className="h-4 w-56 rounded" />
        </div>
      </div>

      <div className="mb-6 flex flex-wrap gap-2">
        {Array.from({ length: 5 }, (_, i) => (
          <Skeleton key={i} className="h-8 w-24 rounded-full" />
        ))}
      </div>

      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <div className="p-4 sm:p-6 md:p-8">
          <Skeleton className="mb-4 h-5 w-40 rounded" />

          <Skeleton className="mt-4 h-3 w-full rounded-full" />

          <div className="mt-3 flex flex-wrap gap-x-5 gap-y-1">
            {Array.from({ length: 3 }, (_, i) => (
              <div key={i} className="flex items-center gap-1.5">
                <Skeleton className="h-2.5 w-2.5 rounded-full" />
                <Skeleton className="h-4 w-16 rounded" />
              </div>
            ))}
          </div>

          <Skeleton className="mt-6 h-[180px] w-full rounded-lg sm:h-[220px]" />
        </div>
      </div>
    </div>
  );
}
