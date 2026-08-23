"use client";

import { BackButton } from "~/components/ui/back-button";
import { Skeleton } from "~/components/ui/skeleton";

// Mirrors StudentDetailHeader (avatar + name + meta rows + location badge),
// the checkout/checkin action-card row, the tab bar, and a Stammdaten-shaped
// field section, so the loaded page swaps in without layout shift.
//
// `referrer` is real chrome, not data-bound (it comes straight off the
// `?from=` query param) — when the caller already knows it, render the real
// BackButton instead of a placeholder so it's clickable immediately. The
// route-level loading.tsx renders this with no `referrer` (it has no access
// to the page's search params), so that placeholder path stays.
export function StudentDetailSkeleton({
  referrer,
}: Readonly<{ referrer?: string }>) {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Kind wird geladen"
      data-testid="student-detail-skeleton"
      className="mx-auto max-w-7xl"
    >
      {referrer ? (
        <BackButton referrer={referrer} />
      ) : (
        <Skeleton className="mb-4 h-9 w-24 rounded-lg" />
      )}

      <div className="mb-6 flex items-end justify-between gap-4">
        <div className="ml-0 flex flex-1 items-center gap-4 sm:ml-6">
          <Skeleton className="h-16 w-16 flex-shrink-0 rounded-full" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-7 w-48 rounded" />
            <Skeleton className="h-4 w-32 rounded" />
            <Skeleton className="h-4 w-56 rounded" />
          </div>
        </div>
        <div className="mr-0 flex-shrink-0 pb-3 sm:mr-4">
          <Skeleton className="h-8 w-28 rounded-full" />
        </div>
      </div>

      <div className="mb-4 flex gap-3 sm:mb-6 sm:gap-4">
        <Skeleton className="h-20 w-full rounded-2xl" />
        <Skeleton className="h-20 w-full rounded-2xl" />
      </div>

      <div className="overflow-x-auto border-b border-gray-200">
        <div className="flex w-max gap-6 pb-px">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-5 w-24 rounded" />
          ))}
        </div>
      </div>

      <div className="mt-4 space-y-4 sm:mt-6 sm:space-y-6">
        <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-6">
          <Skeleton className="mb-4 h-5 w-40 rounded" />
          <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
            {Array.from({ length: 6 }, (_, field) => (
              <div key={field} className="space-y-1.5">
                <Skeleton className="h-3 w-20 rounded" />
                <Skeleton className="h-4 w-32 rounded" />
              </div>
            ))}
          </dl>
        </div>
      </div>
    </div>
  );
}
