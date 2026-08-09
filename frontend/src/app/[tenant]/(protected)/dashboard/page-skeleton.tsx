"use client";

import { Skeleton } from "~/components/ui/skeleton";

// Content-shaped placeholder for the auth-session loading gate — mirrors the
// real shell (greeting, stat-card grid, info-card grid) so there is no layout
// shift once data arrives.
export function DashboardSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Übersicht wird geladen"
      data-testid="dashboard-skeleton"
      className="w-full"
    >
      <div className="mb-6 md:mb-8">
        <div className="ml-6 space-y-2">
          <Skeleton className="h-8 w-64 rounded-full" />
          <Skeleton className="h-4 w-48 rounded-full" />
        </div>
      </div>

      <div className="mb-6 grid grid-cols-2 gap-3 md:mb-8 md:grid-cols-3 md:gap-4 xl:grid-cols-4">
        {[0, 1, 2, 3, 4, 5, 6, 7].map((item) => (
          <div
            key={item}
            className="moto-content-surface rounded-3xl border p-4 shadow-sm md:p-6"
          >
            <div className="mb-3 flex items-start justify-between">
              <Skeleton className="h-10 w-10 rounded-2xl md:h-12 md:w-12" />
            </div>
            <div className="space-y-2">
              <Skeleton className="h-3 w-2/3 rounded-full" />
              <Skeleton className="h-7 w-1/2 rounded-full" />
            </div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 items-stretch gap-4 md:gap-6 lg:grid-cols-2 xl:grid-cols-3">
        {[0, 1, 2].map((item) => (
          <div
            key={item}
            className="moto-content-surface rounded-3xl border p-4 shadow-sm md:p-6"
          >
            <div className="mb-4 flex items-center gap-2">
              <Skeleton className="h-8 w-8 rounded-xl" />
              <Skeleton className="h-5 w-32 rounded-full" />
            </div>
            <div className="space-y-2">
              <Skeleton className="h-12 w-full rounded-xl" />
              <Skeleton className="h-12 w-full rounded-xl" />
              <Skeleton className="h-12 w-full rounded-xl" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
