"use client";

import { Skeleton } from "~/components/ui/skeleton";

// Mirrors uebersicht-tab.tsx: 3 KpiCards, then a 2-column chart-card grid
// (Tagesvergleich line chart + Saldo-Verlauf area chart), then a donut +
// legend card — so the loaded tab swaps in without layout shift. Lives in
// components/ (not the staff/[id] route) so both the route-level skeleton
// and the tab's own data-refetch loading state can import it.
export function UebersichtTabSkeleton() {
  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {Array.from({ length: 3 }, (_, i) => (
          <div
            key={i}
            className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-sm"
          >
            <Skeleton className="h-3 w-24 rounded" />
            <Skeleton className="mt-2 h-7 w-20 rounded" />
            <Skeleton className="mt-3 h-3 w-32 rounded" />
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
        {Array.from({ length: 2 }, (_, i) => (
          <div
            key={i}
            className="rounded-3xl border border-gray-100/50 bg-white/90 p-4 shadow-sm sm:p-6"
          >
            <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <Skeleton className="h-3 w-40 rounded" />
              <Skeleton className="h-9 w-40 rounded-lg" />
            </div>
            <Skeleton className="h-64 w-full rounded-xl" />
          </div>
        ))}
      </div>

      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-4 shadow-sm sm:p-6">
        <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Skeleton className="h-3 w-32 rounded" />
          <Skeleton className="h-9 w-40 rounded-lg" />
        </div>
        <div className="grid grid-cols-1 items-center gap-6 md:grid-cols-2">
          <Skeleton className="mx-auto h-64 w-64 rounded-full" />
          <div className="flex flex-col gap-3">
            {Array.from({ length: 4 }, (_, i) => (
              <div key={i} className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  <Skeleton className="h-2.5 w-2.5 rounded-full" />
                  <Skeleton className="h-3 w-20 rounded" />
                </div>
                <Skeleton className="h-3 w-16 rounded" />
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
