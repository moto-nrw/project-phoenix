"use client";

import { useCallback, useState } from "react";
import {
  AdminEnrollmentsList,
  type AdminEnrollmentsSummary,
} from "~/components/enrollment/admin-enrollments-list";
import { TenantPage } from "~/components/ui/tenant-page";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
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
    <TenantPage
      title="Überblick"
      stats={statusLine}
      statsLoading={statusLine === null}
      loading={!isReady}
    >
      <DesktopOnlyNotice />
      <PhaseExpiryWarnings />
      <div className="hidden lg:block">
        <AdminEnrollmentsList onSummaryChange={handleSummaryChange} />
      </div>
    </TenantPage>
  );
}
