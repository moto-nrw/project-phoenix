"use client";

// Abrechnung des Trägers (#4668): Abrechnungsläufe der Ferienbetreuung. Das
// Muster der Rechnungsnummern liegt in den Abrechnungsvorgaben (#5685). Ein Lauf rechnet einen Ferienzeitraum im Voraus ab; nach
// der Prüfung gibt die Abrechnung ihn frei. Nur die Rolle mit dem Recht
// „Abrechnung“ sieht die Seite. Der Filter „Ferienzeitraum“ steht in der
// Adresse (`?ferienzeitraum=`), damit „Zur Abrechnung“ im Kopf eines
// Ferienzeitraums hierher führen kann (#5688).

import { Plus, Receipt } from "lucide-react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useState } from "react";
import { BillingRunStartModal } from "~/components/traeger/billing-run-start-modal";
import {
  BILLING_RUN_STATUS_LABELS,
  BILLING_RUN_STATUS_TONES,
} from "~/components/traeger/billing-format";
import { BILLING_PERIOD_PARAM } from "~/components/traeger/holiday-period-billing-link";
import { formatEuro } from "~/components/traeger/contribution-format";
import { formatDateRange } from "~/components/traeger/holiday-period-format";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { Alert } from "~/components/ui/alert";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
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
}

interface PeriodOption {
  readonly value: string;
  readonly label: string;
}

/**
 * Die Ferienzeiträume, nach denen sich die Liste filtern lässt: die mit einem
 * Lauf und die, für die sich einer starten lässt, je einmal.
 */
function periodOptions(data: PageData): PeriodOption[] {
  const names = new Map<string, string>();
  for (const run of data.runs) names.set(run.holidayPeriodId, run.periodName);
  for (const period of data.periods) names.set(period.id, period.name);
  return [...names].map(([value, label]) => ({ value, label }));
}

export default function TraegerAbrechnungPage() {
  // `useSearchParams` braucht eine Suspense-Grenze; bis dahin lädt die Seite.
  return (
    <Suspense fallback={<TenantPage title="Abrechnung" loading />}>
      <BillingRunsPage />
    </Suspense>
  );
}

function BillingRunsPage() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const periodFilter = searchParams?.get(BILLING_PERIOD_PARAM) ?? "";
  const [data, setData] = useState<PageData | null>(null);
  const [starting, setStarting] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      const [runs, periods] = await Promise.all([
        fetchBillingRuns(),
        fetchBillingPeriods(),
      ]);
      setData({ runs, periods });
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

  const setPeriodFilter = (periodId: string) => {
    const next = new URLSearchParams(searchParams?.toString());
    if (periodId) next.set(BILLING_PERIOD_PARAM, periodId);
    else next.delete(BILLING_PERIOD_PARAM);
    const query = next.toString();
    router.replace(query ? `${pathname}?${query}` : pathname, {
      scroll: false,
    });
  };

  const released =
    data?.runs.filter((run) => run.status === "released").length ?? 0;
  const noPeriod = data !== null && data.periods.length === 0;
  const options = data ? periodOptions(data) : [];
  const filterName = options.find(
    (option) => option.value === periodFilter,
  )?.label;
  const shownRuns = periodFilter
    ? (data?.runs.filter((run) => run.holidayPeriodId === periodFilter) ?? [])
    : (data?.runs ?? []);
  // Ein gefilterter Zeitraum, für den sich ein Lauf starten lässt.
  const startablePeriod = data?.periods.some(
    (period) => period.id === periodFilter,
  );

  const filters: FilterConfig[] =
    options.length > 0
      ? [
          {
            id: "period",
            label: "Ferienzeitraum",
            type: "dropdown",
            value: periodFilter,
            onChange: (value) => setPeriodFilter(value as string),
            options: [{ value: "", label: "Alle Ferienzeiträume" }, ...options],
          },
        ]
      : [];
  const activeFilters: ActiveFilter[] = periodFilter
    ? [
        {
          id: "period",
          label: filterName ?? "Ausgewählter Ferienzeitraum",
          onRemove: () => setPeriodFilter(""),
        },
      ]
    : [];

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
      filters={filters}
      activeFilters={activeFilters}
      onClearAllFilters={() => setPeriodFilter("")}
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
            initialPeriodId={periodFilter}
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
              message="Es gibt noch keinen veröffentlichten Ferienzeitraum. Ein Lauf braucht einen. Ferienzeiträume legt die Verwaltung unter „Ferienbetreuung“ an."
            />
          ) : null}
          <DataTable
            columns={COLUMNS}
            rows={shownRuns}
            getRowKey={(run) => run.id}
            stackedOnMobile
            stackedLayout="row"
            onRowClick={openRun}
            emptyState={
              periodFilter ? (
                <EmptyState
                  icon={<Receipt className="h-12 w-12" aria-hidden="true" />}
                  title={
                    filterName
                      ? `Noch kein Abrechnungslauf für ${filterName}.`
                      : "Noch kein Abrechnungslauf für diesen Ferienzeitraum."
                  }
                  description={
                    startablePeriod
                      ? "Starten Sie ihn oben mit „Lauf starten“. Der Zeitraum ist dort schon gewählt."
                      : "Einen Lauf gibt es erst, wenn der Ferienzeitraum veröffentlicht ist."
                  }
                />
              ) : (
                <EmptyState
                  icon={<Receipt className="h-12 w-12" aria-hidden="true" />}
                  title="Noch kein Abrechnungslauf."
                />
              )
            }
          />
        </>
      ) : null}
    </TenantPage>
  );
}
