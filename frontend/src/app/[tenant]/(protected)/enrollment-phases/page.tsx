"use client";

import { Suspense } from "react";
import { PhasesEditor } from "~/components/enrollment/phases-editor";
import { SkeletonRegion, TableSkeleton } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

function PhasesEditorSkeleton() {
  return (
    <SkeletonRegion label="Anmeldephasen werden geladen">
      <TableSkeleton columns={6} rows={5} />
    </SkeletonRegion>
  );
}

export default function EnrollmentPhasesPage() {
  const { isReady } = useRequireAdmin();
  const showSkeleton = !isReady;

  return (
    <div className="-mt-1.5 w-full">
      {showSkeleton ? (
        <PhasesEditorSkeleton />
      ) : (
        <Suspense fallback={<PhasesEditorSkeleton />}>
          <PhasesEditor />
        </Suspense>
      )}
    </div>
  );
}
