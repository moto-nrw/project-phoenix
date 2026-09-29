"use client";

import { useCallback, useState } from "react";
import {
  AdminEnrollmentsList,
  type AdminEnrollmentsSummary,
} from "~/components/enrollment/admin-enrollments-list";
import { TenantPage } from "~/components/ui/tenant-page";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { PhaseExpiryWarnings } from "~/components/enrollment/phase-expiry-warnings";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { useToast } from "~/contexts/ToastContext";
import { markAllAdminRequestsRead } from "~/lib/enrollment-admin-api";

export default function AdminEnrollmentsPage() {
  // Jede Anmeldungsroute verlangt config:manage (#3469).
  const { isReady } = useRequirePermission("config:manage");
  // Statuszeile des Seitenkopfs: die Zahlen, die die Liste ohnehin lädt.
  const [summary, setSummary] = useState<AdminEnrollmentsSummary | null>(null);
  const { success: toastSuccess, error: toastError } = useToast();
  // Lesestatus pro Person (#3778): markiert die ungelesenen Anmeldungen der
  // aktiven Phasen für die angemeldete Person.
  const handleMarkAllRead = useCallback(() => {
    markAllAdminRequestsRead()
      .then(() => toastSuccess("Alle Anmeldungen sind als gelesen markiert."))
      .catch((err: unknown) =>
        toastError(
          err instanceof Error
            ? err.message
            : "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
        ),
      );
  }, [toastSuccess, toastError]);
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
      actions={
        <OverflowMenu
          ariaLabel="Weitere Aktionen für Anmeldungen"
          items={[
            {
              label: "Alle als gelesen markieren",
              onClick: handleMarkAllRead,
            },
          ]}
        />
      }
    >
      <DesktopOnlyNotice />
      <PhaseExpiryWarnings />
      {/* Flex-Spalte, damit die letzte Fläche bis zur Unterkante wächst
          (`.moto-tenant-body`). */}
      <div className="hidden lg:flex lg:flex-col">
        <AdminEnrollmentsList onSummaryChange={handleSummaryChange} />
      </div>
    </TenantPage>
  );
}
