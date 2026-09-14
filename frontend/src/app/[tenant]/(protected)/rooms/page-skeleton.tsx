"use client";

import { CollectionGrid } from "~/components/ui/collection-grid";
import { Skeleton } from "~/components/ui/skeleton";

// Single skeleton card that matches the populated room card's outer
// shell: same rounded-2xl, same min-h-[180px], same flex layout so the
// page doesn't reshuffle on swap. Kit skeletons stand in for title row,
// meta line, status pill, two middle rows, and the footer hint.
function RoomCardSkeleton() {
  return (
    <div className="moto-content-surface relative overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      <div className="bg-moto-blue absolute inset-0 rounded-2xl opacity-[0.03]"></div>
      <div className="relative flex min-h-[180px] flex-col p-6">
        <div className="mb-3 flex items-start justify-between">
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-5 w-2/3 rounded" />
            <Skeleton className="h-3 w-1/3 rounded" />
          </div>
          <Skeleton className="ml-3 h-6 w-16 flex-shrink-0 rounded-full" />
        </div>
        <div className="flex-1 space-y-2">
          <Skeleton className="h-3 w-3/4 rounded" />
          <Skeleton className="h-3 w-1/2 rounded" />
        </div>
        <Skeleton className="mt-2 h-3 w-24 rounded" />
      </div>
    </div>
  );
}

export function RoomsGridSkeleton() {
  // Eight cards covers two rows on the largest grid (2xl: 4 columns);
  // smaller breakpoints fill more rows naturally. Same gap + column
  // breakpoints as the populated grid below so the swap is purely a
  // child-level change, not a container reshape.
  return (
    <CollectionGrid
      as="output"
      ariaLabel="Räume werden geladen"
      testId="rooms-grid-skeleton"
    >
      {Array.from({ length: 8 }).map((_, i) => (
        <RoomCardSkeleton key={i} />
      ))}
    </CollectionGrid>
  );
}
