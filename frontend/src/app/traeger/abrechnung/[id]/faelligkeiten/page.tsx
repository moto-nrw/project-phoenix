"use client";

// Fälligkeiten eines Ferienlaufs vor der Freigabe (#4689): je
// Zahlungsweg das Datum, an dem die Rechnungen fällig sind. Eine eigene
// Seite, damit die Seite des Laufs den Datumswähler nicht lädt. Nach der
// Freigabe ändern sich die Daten nicht mehr.

import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { holidayDebitItems } from "~/components/traeger/care-debit-hints";
import { RunDueDatesForm } from "~/components/traeger/run-due-dates-form";
import { usePrenotificationDays } from "~/components/traeger/use-prenotification-days";
import { Alert } from "~/components/ui/alert";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type BillingRunDetail,
  fetchBillingRun,
} from "~/lib/traeger-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerHolidayRunDueDatesPage" });

export default function TraegerHolidayRunDueDatesPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const runId = params?.id ?? "";
  const [run, setRun] = useState<BillingRunDetail | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const runHref = traegerPath(`/traeger/abrechnung/${runId}`);

  const load = useCallback(async () => {
    try {
      setRun(await fetchBillingRun(runId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_holiday_run_due_dates_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Abrechnungslauf",
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
      backLabel="Zurück zum Abrechnungslauf"
      stats={run ? <span>{run.periodName}</span> : null}
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
          kind="holiday"
          runId={run.id}
          dueDates={run.dueDates ?? []}
          debitItems={holidayDebitItems(run.documents)}
          prenotificationDays={prenotificationDays}
          onCancel={() => router.push(runHref)}
          onSaved={() => router.push(runHref)}
        />
      ) : null}
    </TenantPage>
  );
}
