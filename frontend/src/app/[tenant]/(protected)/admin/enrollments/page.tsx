"use client";

import { useCallback, useState } from "react";
import {
  AdminEnrollmentsList,
  type AdminEnrollmentsSummary,
} from "~/components/enrollment/admin-enrollments-list";
import { Skeleton } from "~/components/ui/skeleton";
import { PageIntro } from "~/components/ui/page-intro";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import { PhaseExpiryWarnings } from "~/components/enrollment/phase-expiry-warnings";

export default function AdminEnrollmentsPage() {
  const { isReady } = useRequireAdmin();
  // Statuszeile des Seitenkopfs: die Zahlen, die die Liste ohnehin lädt.
  const [summary, setSummary] = useState<AdminEnrollmentsSummary | null>(null);
  const handleSummaryChange = useCallback(
    (next: AdminEnrollmentsSummary | null) => setSummary(next),
    [],
  );
  const statusLine = summary
    ? [
        `${summary.activePhases} ${summary.activePhases === 1 ? "Phase" : "Phasen"} aktiv`,
        `${summary.requests} ${summary.requests === 1 ? "Anmeldung" : "Anmeldungen"}`,
        summary.openChangeRequests > 0
          ? `${summary.openChangeRequests} offene ${summary.openChangeRequests === 1 ? "Änderungsanfrage" : "Änderungsanfragen"}`
          : null,
      ]
        .filter(Boolean)
        .join(" · ")
    : null;

  return (
    <div className="w-full space-y-6">
      <PageIntro
        kicker="Anmeldungen"
        title="Überblick"
        description={statusLine ?? <Skeleton className="h-4 w-56" />}
      />
      {isReady ? (
        <>
          <DesktopOnlyNotice />
          <PhaseExpiryWarnings />
          <div className="hidden lg:block">
            <AdminEnrollmentsList onSummaryChange={handleSummaryChange} />
          </div>
        </>
      ) : (
        <SkeletonRegion label="Anmeldungen werden geladen">
          <ListSkeleton rows={6} avatar={false} />
        </SkeletonRegion>
      )}
    </div>
  );
}
