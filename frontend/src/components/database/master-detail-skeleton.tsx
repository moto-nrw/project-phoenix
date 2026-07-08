"use client";

import { MasterDetailLayout } from "./master-detail-layout";
import { Skeleton } from "~/components/ui/skeleton";
import { PageHeaderSkeleton } from "~/components/ui/page-header/PageHeaderSkeleton";

function ListRowSkeleton() {
  // Mirrors DatabaseListItem: title + subtitle lines, chevron slot.
  return (
    <div className="flex w-full items-center gap-3 border-b border-gray-100 px-4 py-2.5">
      <div className="min-w-0 flex-1 space-y-1.5">
        <Skeleton className="h-4 w-2/3 rounded" />
        <Skeleton className="h-3 w-1/2 rounded" />
      </div>
      <Skeleton className="h-4 w-4 shrink-0 rounded" />
    </div>
  );
}

/**
 * Loading placeholder for database master/detail pages. Renders the page
 * header strip plus the real MasterDetailLayout containers (identical
 * width/height behavior, including the mobile single-column variant) filled
 * with row skeletons, so the loaded page swaps in without layout shift.
 * All database pages use unselectedBehavior="expand", so the skeleton shows
 * the full-width list state that appears when data arrives.
 */
export function MasterDetailSkeleton({
  rowCount = 8,
  label = "Daten werden geladen",
}: Readonly<{ rowCount?: number; label?: string }>) {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label={label}
      data-testid="master-detail-skeleton"
      className="-mt-1.5 flex w-full flex-col"
    >
      <PageHeaderSkeleton />
      <div className="min-h-0 flex-1 pb-4">
        <MasterDetailLayout
          selectedId={null}
          onDeselect={() => undefined}
          unselectedBehavior="expand"
          list={
            <div>
              {Array.from({ length: rowCount }, (_, i) => (
                <ListRowSkeleton key={i} />
              ))}
            </div>
          }
          detail={null}
        />
      </div>
    </div>
  );
}
