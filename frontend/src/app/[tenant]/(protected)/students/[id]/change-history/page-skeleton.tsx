"use client";

import { StudentHistoryLoadingBack } from "../student-history-loading-back";
import {
  SkeletonRegion,
  TenantPageHeaderSkeleton,
} from "~/components/ui/page-skeletons";
import { DataTableSkeleton as TableSkeleton } from "~/components/ui/data-table";

export function ChangeHistorySkeleton() {
  return (
    <div className="w-full">
      {/* Spiegelt die Kopfkarte des geladenen Zustands: Name und
          Klasse sind datengebunden, Kicker und Icon stehen fest. */}
      <TenantPageHeaderSkeleton leading back={<StudentHistoryLoadingBack />} />

      <SkeletonRegion label="Änderungsverlauf wird geladen">
        <TableSkeleton rows={8} columns={5} />
      </SkeletonRegion>
    </div>
  );
}
