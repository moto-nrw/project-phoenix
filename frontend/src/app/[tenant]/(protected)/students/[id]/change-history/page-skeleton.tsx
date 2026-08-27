"use client";

import { Skeleton } from "~/components/ui/skeleton";
import { SkeletonRegion, TableSkeleton } from "~/components/ui/page-skeletons";

export function ChangeHistorySkeleton() {
  return (
    <div className="w-full">
      <Skeleton className="mb-4 h-9 w-24 rounded-lg" />

      {/* Spiegelt die Kopfkarte (PageIntro) des geladenen Zustands: Name und
          Klasse sind datengebunden, Kicker und Icon stehen fest. */}
      <div className="moto-content-surface mb-6 rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <div className="flex min-w-0 gap-3">
          <Skeleton className="h-12 w-12 flex-shrink-0 rounded-xl" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-3 w-32 rounded" />
            <Skeleton className="h-7 w-48 rounded" />
            <Skeleton className="h-4 w-36 rounded" />
          </div>
        </div>
      </div>

      <SkeletonRegion label="Änderungsverlauf wird geladen">
        <TableSkeleton rows={8} columns={5} />
      </SkeletonRegion>
    </div>
  );
}
