"use client";

import { Suspense } from "react";
import { PhasesEditor } from "~/components/enrollment/phases-editor";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { SkeletonRegion, TableSkeleton } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

function PhasesEditorSkeleton() {
  return (
    <SkeletonRegion label="Anmeldephasen werden geladen" className="mt-4">
      <TableSkeleton columns={6} rows={5} />
    </SkeletonRegion>
  );
}

export default function EnrollmentPhasesPage() {
  const { isReady } = useRequireAdmin();
  const showSkeleton = !isReady;

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Anmeldephasen" />
      {showSkeleton ? (
        <PhasesEditorSkeleton />
      ) : (
        <>
          <DesktopOnlyNotice />
          <div className="hidden lg:block">
            <Suspense fallback={<PhasesEditorSkeleton />}>
              <PhasesEditor />
            </Suspense>
          </div>
        </>
      )}
    </div>
  );
}
