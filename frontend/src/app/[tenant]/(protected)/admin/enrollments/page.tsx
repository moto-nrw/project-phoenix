"use client";

import { useCallback, useEffect, useState } from "react";
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
import {
  fetchEmailSubscription,
  setEmailSubscription,
} from "~/lib/notification-preferences-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AdminEnrollmentsPage" });

/** E-Mail „Neue Anmeldung" an die Person selbst, zum Einschalten (#3780). */
const ENROLLMENT_EMAIL = "enrollment_submitted";
const SAVE_FAILED =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";

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
        toastError(err instanceof Error ? err.message : SAVE_FAILED),
      );
  }, [toastSuccess, toastError]);
  // Eigener Stand der E-Mail bei neuer Anmeldung. Lässt er sich nicht laden,
  // fehlt der Eintrag, statt einen falschen Haken zu zeigen.
  const [emailState, setEmailState] = useState<
    "loading" | "on" | "off" | "unavailable"
  >("loading");
  useEffect(() => {
    if (!isReady) return;
    let cancelled = false;
    fetchEmailSubscription(ENROLLMENT_EMAIL)
      .then((enabled) => {
        if (!cancelled) setEmailState(enabled ? "on" : "off");
      })
      .catch((err: unknown) => {
        logger.warn("enrollment_email_subscription_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) setEmailState("unavailable");
      });
    return () => {
      cancelled = true;
    };
  }, [isReady]);
  const handleToggleEmail = useCallback(() => {
    if (emailState !== "on" && emailState !== "off") return;
    const next = emailState === "off";
    setEmailState(next ? "on" : "off");
    setEmailSubscription(ENROLLMENT_EMAIL, next)
      .then(() =>
        toastSuccess(
          next
            ? "Sie bekommen jetzt bei jeder neuen Anmeldung eine E-Mail."
            : "Sie bekommen keine E-Mail mehr bei neuen Anmeldungen.",
        ),
      )
      .catch(() => {
        setEmailState(next ? "off" : "on");
        toastError(SAVE_FAILED);
      });
  }, [emailState, toastSuccess, toastError]);
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
            ...(emailState === "unavailable"
              ? []
              : [
                  { kind: "separator" } as const,
                  {
                    kind: "checkbox" as const,
                    label: "E-Mail an mich bei neuer Anmeldung",
                    checked: emailState === "on",
                    disabled: emailState === "loading",
                    onClick: handleToggleEmail,
                  },
                ]),
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
