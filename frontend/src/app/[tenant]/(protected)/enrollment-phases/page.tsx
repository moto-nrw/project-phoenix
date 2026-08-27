"use client";

import { Suspense } from "react";
import { PhasesEditor } from "~/components/enrollment/phases-editor";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { PageIntro } from "~/components/ui/page-intro";
import { Skeleton } from "~/components/ui/skeleton";
import { formatStatusDate } from "~/lib/date-helpers";
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
    <div className="w-full space-y-4">
      {showSkeleton ? (
        <>
          {/* Im geladenen Zustand trägt der Editor die Kopfkarte, weil die
              Aktion „Neue Anmeldephase“ an seinen Zustand gebunden ist. */}
          <PageIntro
            kicker="Anmeldungen"
            title="Anmeldephasen"
            description={<Skeleton className="h-4 w-52" />}
          />
          <PhasesEditorSkeleton />
        </>
      ) : (
        <>
          {/* Auf Mobil zeigt die Seite nur den Hinweis; die Kopfkarte des
              Editors ist dort ausgeblendet, deshalb steht sie hier. */}
          <div className="lg:hidden">
            <PageIntro
              kicker="Anmeldungen"
              title="Anmeldephasen"
              description={formatStatusDate()}
            />
          </div>
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
