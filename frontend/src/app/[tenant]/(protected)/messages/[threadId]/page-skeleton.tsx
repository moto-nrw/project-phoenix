"use client";

import { Skeleton } from "~/components/ui/skeleton";

// Mirrors the real thread layout: back-nav, header row (guardian name +
// subtitle vs. "Zum Kinderprofil" button), the scrollable chat bubble list,
// and the composer bar, so the loaded page swaps in without layout shift.
export function ThreadSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Unterhaltung wird geladen"
      data-testid="thread-skeleton"
      className="-mt-1.5 flex min-h-[20rem] w-full flex-col overflow-hidden"
    >
      <Skeleton className="mb-4 h-5 w-40 rounded" />

      <div className="moto-content-surface flex min-h-0 flex-1 flex-col rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
        <div className="mb-4 flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-6 w-40 rounded" />
            <Skeleton className="h-4 w-56 rounded" />
          </div>
          <Skeleton className="h-9 w-36 flex-shrink-0 rounded-lg" />
        </div>

        <div className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton
              key={i}
              className={`h-12 rounded-2xl ${
                i % 2 === 0 ? "w-2/3" : "ml-auto w-1/2"
              }`}
            />
          ))}
        </div>

        <div className="mt-4">
          <Skeleton className="h-12 w-full rounded-full" />
        </div>
      </div>
    </div>
  );
}
