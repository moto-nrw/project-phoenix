"use client";

import { CollectionGrid } from "~/components/ui/collection-grid";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";

function StaffCardSkeleton() {
  // Mirrors the staff card: avatar-less header (name lines + status badge),
  // supervision/qualification rows, footer hint, auf derselben p-4-Fläche.
  return (
    <SectionCard padding="none" className="w-full">
      <div className="p-4">
        <div className="mb-3 flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-5 w-2/3" />
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-3 w-2/5" />
          </div>
          <Skeleton shape="round" className="h-6 w-20 flex-shrink-0" />
        </div>
        <div className="flex-1 space-y-2">
          <Skeleton className="h-3.5 w-4/5" />
          <div className="flex flex-wrap gap-2">
            <Skeleton className="h-5 w-16" />
            <Skeleton className="h-5 w-20" />
          </div>
        </div>
      </div>
    </SectionCard>
  );
}

/**
 * Data-region skeleton: just the staff-card grid. The real header
 * (TenantPage search row) renders immediately regardless of loading state,
 * only this data-bound region skeletonizes while staff data loads.
 */
export function StaffCardsSkeleton() {
  return (
    <CollectionGrid
      role="status"
      ariaBusy
      ariaLabel="Mitarbeitende werden geladen"
      testId="staff-page-skeleton"
    >
      {Array.from({ length: 6 }, (_, i) => (
        <StaffCardSkeleton key={i} />
      ))}
    </CollectionGrid>
  );
}
