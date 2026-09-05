"use client";

import { Skeleton } from "~/components/ui/skeleton";
import {
  SkeletonRegion,
  TableSkeleton,
  TenantPageHeaderSkeleton,
} from "~/components/ui/page-skeletons";

export function ChangeHistorySkeleton() {
  return (
    <div className="w-full">
      <Skeleton className="mb-4 h-9 w-24 rounded-lg" />

      {/* Spiegelt die Kopfkarte des geladenen Zustands: Name und
          Klasse sind datengebunden, Kicker und Icon stehen fest. */}
      <TenantPageHeaderSkeleton leading />

      <SkeletonRegion label="Änderungsverlauf wird geladen">
        <TableSkeleton rows={8} columns={5} />
      </SkeletonRegion>
    </div>
  );
}
