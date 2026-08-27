"use client";

import { Skeleton } from "~/components/ui/skeleton";

function StaffCardSkeleton() {
  // Mirrors the staff card: avatar-less header (name lines + status badge),
  // supervision/qualification rows, footer hint, auf derselben p-4-Fläche.
  return (
    <div className="moto-content-surface w-full overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
      <div className="p-4">
        <div className="mb-3 flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-5 w-2/3 rounded" />
            <Skeleton className="h-4 w-1/2 rounded" />
            <Skeleton className="h-3 w-2/5 rounded" />
          </div>
          <Skeleton className="h-6 w-20 flex-shrink-0 rounded-full" />
        </div>
        <div className="flex-1 space-y-2">
          <Skeleton className="h-3.5 w-4/5 rounded" />
          <div className="flex flex-wrap gap-2">
            <Skeleton className="h-5 w-16 rounded" />
            <Skeleton className="h-5 w-20 rounded" />
          </div>
        </div>
      </div>
    </div>
  );
}

/**
 * Data-region skeleton: just the staff-card grid. The real header
 * (PageHeaderWithSearch) renders immediately regardless of loading state,
 * only this data-bound region skeletonizes while staff data loads.
 */
export function StaffCardsSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Mitarbeitende werden geladen"
      data-testid="staff-page-skeleton"
      className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3"
    >
      {Array.from({ length: 6 }, (_, i) => (
        <StaffCardSkeleton key={i} />
      ))}
    </div>
  );
}
