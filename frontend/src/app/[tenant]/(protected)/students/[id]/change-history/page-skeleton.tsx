"use client";

import { StudentHistoryLoadingBack } from "../student-history-loading-back";
import {
  SkeletonRegion,
  TableSkeleton,
  TenantPageHeaderSkeleton,
} from "~/components/ui/page-skeletons";

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
