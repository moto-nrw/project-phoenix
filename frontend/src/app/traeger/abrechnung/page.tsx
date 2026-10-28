"use client";

// Abrechnung des Trägers (#4668): Rechnungsnummern und Abrechnungsläufe der
// Ferienbetreuung. Ein Lauf rechnet einen Ferienzeitraum im Voraus ab; nach
// der Prüfung gibt die Abrechnung ihn frei. Nur die Rolle mit dem Recht
// „Abrechnung“ sieht die Seite.

import { Plus, Receipt } from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { BillingRunStartModal } from "~/components/traeger/billing-run-start-modal";
import {
  BILLING_RUN_STATUS_LABELS,
  BILLING_RUN_STATUS_TONES,
} from "~/components/traeger/billing-format";
import { formatEuro } from "~/components/traeger/contribution-format";
import { formatDateRange } from "~/components/traeger/holiday-period-format";
import { NumberRangeCard } from "~/components/traeger/number-range-card";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { Alert } from "~/components/ui/alert";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type BillingPeriod,
  type BillingRun,
  fetchBillingPeriods,
  fetchBillingRuns,
  fetchHolidayNumberRange,
  type NumberRange,
} from "~/lib/traeger-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerAbrechnungPage" });

const COLUMNS: DataTableColumn<BillingRun>[] = [
  {
    key: "period",
    header: "Ferienzeitraum",
    render: (run) => (
      <span className="font-medium text-gray-900">{run.periodName}</span>
    ),
    stacked: "title",
  },
  {
    key: "days",
    header: "Betreuung",
    render: (run) => formatDateRange(run.serviceFrom, run.serviceUntil),
    stacked: "meta",
  },
  {
    key: "status",
    header: "Status",
    render: (run) => (
      <StatusBadge
        label={BILLING_RUN_STATUS_LABELS[run.status]}
        tone={BILLING_RUN_STATUS_TONES[run.status]}
      />
    ),
    stacked: "meta",
  },
  {
    key: "invoices",
    header: "Rechnungen",
    render: (run) =>
      run.documentCount === null
        ? "–"
        : `${run.documentCount} · ${formatEuro(run.totalCents ?? 0)}`,
    stacked: "meta",
  },
  {
    key: "issued",
    header: "Ausgestellt am",
    render: (run) => (run.issuedOn ? formatDate(run.issuedOn) : "–"),
    stacked: "meta",
  },
];

interface PageData {
  readonly runs: BillingRun[];
  readonly periods: BillingPeriod[];
  readonly numberRange: NumberRange;
}

export default function TraegerAbrechnungPage() {
  const router = useRouter();
  const [data, setData] = useState<PageData | null>(null);
  const [starting, setStarting] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      const [runs, periods, numberRange] = await Promise.all([
        fetchBillingRuns(),
        fetchBillingPeriods(),
        fetchHolidayNumberRange(),
      ]);
      setData({ runs, periods, numberRange });
      clearLoadError();
    } catch (err) {
      logger.error("traeger_billing_runs_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Abrechnung",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const openRun = (run: BillingRun) =>
    router.push(traegerPath(`/traeger/abrechnung/${run.id}`));

  const released =
    data?.runs.filter((run) => run.status === "released").length ?? 0;
  const noPeriod = data !== null && data.periods.length === 0;

  return (
    <TenantPage
      title="Abrechnung"
      info="Hier rechnen Sie die Ferienbetreuung im Voraus ab, nach den bestätigten Buchungen. Ein Lauf erstellt die Rechnungen für einen Ferienzeitraum. Sie prüfen die Vorschau und geben den Lauf dann frei."
      stats={
        data && data.runs.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: data.runs.length,
                label:
                  data.runs.length === 1
                    ? "Abrechnungslauf"
                    : "Abrechnungsläufe",
              },
              { value: released, label: "freigegeben" },
            ]}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      actions={
        <Button
          type="button"
          variant="primary"
          size="md"
          disabled={!data || noPeriod}
          aria-describedby={noPeriod ? "billing-run-needs-period" : undefined}
          onClick={() => setStarting(true)}
        >
          <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
          Lauf starten
        </Button>
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !data}
      overlays={
        data ? (
          <BillingRunStartModal
            isOpen={starting}
            periods={data.periods}
            onClose={() => setStarting(false)}
            onStarted={(run) => {
              setStarting(false);
              openRun(run);
            }}
          />
        ) : null
      }
    >
      {data ? (
        <>
          {noPeriod ? (
            <Alert
              id="billing-run-needs-period"
              type="info"
              message="Es gibt noch keinen veröffentlichten Ferienzeitraum. Ein Lauf braucht einen. Ferienzeiträume legt die Verwaltung unter „Ferien“ an."
            />
          ) : null}
          <DataTable
            columns={COLUMNS}
            rows={data.runs}
            getRowKey={(run) => run.id}
            stackedOnMobile
            stackedLayout="row"
            onRowClick={openRun}
            emptyState={
              <EmptyState
                icon={<Receipt className="h-12 w-12" aria-hidden="true" />}
                title="Noch kein Abrechnungslauf."
              />
            }
          />
          <NumberRangeCard numberRange={data.numberRange} onChanged={load} />
        </>
      ) : null}
    </TenantPage>
  );
}
