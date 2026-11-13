"use client";

// Fälligkeiten eines Monatslaufs in der Prüfphase (#4689, #5916): je
// Zahlungsweg das Datum, an dem die Rechnungen fällig sind. Eine eigene
// Seite, damit die Seite des Laufs den Datumswähler nicht lädt. Nach der
// Freigabe ändern sich die Daten nicht mehr.

import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { formatMonth } from "~/components/traeger/care-billing-format";
import { careDebitItems } from "~/components/traeger/care-debit-hints";
import { RunDueDatesForm } from "~/components/traeger/run-due-dates-form";
import { usePrenotificationDays } from "~/components/traeger/use-prenotification-days";
import { Alert } from "~/components/ui/alert";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type CareRunDetail,
  fetchCareRun,
} from "~/lib/traeger-care-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerCareRunDueDatesPage" });

export default function TraegerCareRunDueDatesPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const runId = params?.id ?? "";
  const [run, setRun] = useState<CareRunDetail | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const runHref = traegerPath(`/traeger/abrechnung/monatsbeitraege/${runId}`);

  const load = useCallback(async () => {
    try {
      setRun(await fetchCareRun(runId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_run_due_dates_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Monatslauf",
        retry: () => void load(),
      });
    }
  }, [runId, showLoadError, clearLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  // The pre-notification days name the rule under the direct debit date.
  const prenotificationDays = usePrenotificationDays();

  return (
    <TenantPage
      title="Fälligkeiten"
      back
      backHref={runHref}
      backLabel="Zurück zum Monatslauf"
      stats={run ? <span>{formatMonth(run.billingMonth)}</span> : null}
      statsLoading={!run && !pageLoad.error}
      error={pageLoad.error}
      loading={!pageLoad.error && !run}
    >
      {run && run.status !== "calculated" ? (
        <Alert
          type="info"
          message="Der Lauf ist freigegeben. Die Rechnungen haben ihre Fälligkeit schon und ändern sich nicht mehr."
        />
      ) : null}
      {run && run.status === "calculated" ? (
        <RunDueDatesForm
          kind="care"
          runId={run.id}
          dueDates={run.dueDates ?? []}
          debitItems={careDebitItems(run.invoices)}
          prenotificationDays={prenotificationDays}
          issuedOn={run.invoiceDate}
          onCancel={() => router.push(runHref)}
          onSaved={() => router.push(runHref)}
        />
      ) : null}
    </TenantPage>
  );
}
