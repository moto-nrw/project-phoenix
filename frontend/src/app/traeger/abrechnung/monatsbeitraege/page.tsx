"use client";

// Monatsbeiträge des Trägers (#4684): Ein Monatslauf rechnet jeden Monat
// nach dem gültigen Betreuungsvertrag und der Preisliste ab. Die Abrechnung
// wählt Monat, Schulen und Rechnungsdatum (#5916), prüft die Vorschau und
// gibt den Lauf frei. Nur die Rolle mit dem Recht „Abrechnung“ sieht die
// Seite.

import { CalendarRange, Plus } from "lucide-react";
import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  billingAreaTitle,
  billingTabs,
} from "~/components/traeger/billing-family-format";
import {
  CARE_RUN_STATUS_LABELS,
  CARE_RUN_STATUS_TONES,
  MONTH_RULE_LABELS,
  addMonths,
  formatMonth,
  monthRange,
  invoiceCountLabel,
} from "~/components/traeger/care-billing-format";
import { runSchoolsLabel } from "~/components/traeger/care-review-format";
import { formatEuro } from "~/components/traeger/contribution-format";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareRun,
  type CareRunList,
  type CareSchool,
  type CareTariffs,
  fetchCareRuns,
  fetchCareTariffs,
  startCareRun,
} from "~/lib/traeger-care-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerMonatsbeitraegePage" });

// The start dialog loads when it opens (#5916).
const CareRunStartModal = dynamic(
  () =>
    import("~/components/traeger/care-run-start-modal").then(
      (module) => module.CareRunStartModal,
    ),
  { ssr: false },
);

function runColumns(
  schools: readonly CareSchool[],
): DataTableColumn<CareRun>[] {
  return [
    {
      key: "month",
      header: "Monat",
      render: (run) => (
        <span className="font-medium text-gray-900">
          {formatMonth(run.billingMonth)}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "status",
      header: "Status",
      render: (run) => (
        <StatusBadge
          label={CARE_RUN_STATUS_LABELS[run.status]}
          tone={CARE_RUN_STATUS_TONES[run.status]}
        />
      ),
      stacked: "meta",
    },
    {
      // On a phone the line under the month: the schools, then what the run
      // issued and when (only the status is meta there).
      key: "schools",
      header: "Schulen",
      render: (run) => (
        <span className="text-gray-700">
          {runSchoolsLabel(run.schoolIds, schools)}
        </span>
      ),
      stackedLabel: false,
      stackedRender: (run) =>
        [
          runSchoolsLabel(run.schoolIds, schools),
          run.documentCount === null
            ? null
            : `${invoiceCountLabel(run.documentCount)} · ${formatEuro(run.totalCents ?? 0)}`,
          run.issuedOn ? `ausgestellt am ${formatDate(run.issuedOn)}` : null,
        ]
          .filter(Boolean)
          .join(" · "),
    },
    {
      key: "invoices",
      header: "Rechnungen",
      render: (run) =>
        run.documentCount === null ? (
          "–"
        ) : (
          <span className="tabular-nums">
            {invoiceCountLabel(run.documentCount)} ·{" "}
            {formatEuro(run.totalCents ?? 0)}
          </span>
        ),
      stacked: "hidden",
    },
    {
      key: "issued",
      header: "Ausgestellt am",
      render: (run) => (run.issuedOn ? formatDate(run.issuedOn) : "–"),
      stacked: "hidden",
    },
    {
      key: "rule",
      header: "Monatswechsel",
      render: (run) => (run.monthRule ? MONTH_RULE_LABELS[run.monthRule] : "–"),
      stacked: "hidden",
    },
  ];
}

/** Twelve months back and three ahead of the suggested month, latest first. */
function startMonths(suggested: string) {
  return monthRange(addMonths(suggested, -12), 16)
    .reverse()
    .map((value) => ({ value, label: formatMonth(value) }));
}

interface PageData {
  readonly list: CareRunList;
  readonly tariffs: CareTariffs;
}

export default function TraegerMonatsbeitraegePage() {
  const router = useRouter();
  const [data, setData] = useState<PageData | null>(null);
  const [starting, setStarting] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      const [list, tariffs] = await Promise.all([
        fetchCareRuns(),
        fetchCareTariffs(),
      ]);
      setData({ list, tariffs });
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_runs_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Liste der Monatsbeiträge",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const openRun = (run: CareRun) =>
    router.push(traegerPath(`/traeger/abrechnung/monatsbeitraege/${run.id}`));

  const runs = data?.list.runs ?? [];
  const released = runs.filter((run) => run.status === "released");
  const billedCents = released.reduce(
    (sum, run) => sum + (run.totalCents ?? 0),
    0,
  );
  const noPrices = data !== null && data.tariffs.tariffs.length === 0;

  return (
    <TenantPage
      title={billingAreaTitle("monatsbeitraege")}
      info="Hier rechnen Sie die Betreuung Monat für Monat ab. Der Beitrag ist die Summe der gebuchten Wochentage im Vertrag, mit den Preisen aus der Preisliste. Sie wählen Monat und Schulen, prüfen die Vorschau und geben den Lauf dann frei."
      stats={
        runs.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: runs.length,
                label: runs.length === 1 ? "Monatslauf" : "Monatsläufe",
              },
              { value: released.length, label: "freigegeben" },
              { value: formatEuro(billedCents), label: "ausgestellt" },
            ]}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      tabs={billingTabs("monatsbeitraege")}
      actions={
        <Button
          type="button"
          variant="primary"
          size="md"
          disabled={!data}
          onClick={() => setStarting(true)}
        >
          <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
          Monatslauf starten
        </Button>
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !data}
      overlays={
        data ? (
          <CareRunStartModal
            isOpen={starting}
            months={startMonths(data.list.suggestedMonth)}
            initialMonth={data.list.suggestedMonth}
            onClose={() => setStarting(false)}
            onStart={async (input) => {
              const run = await startCareRun(input);
              setStarting(false);
              openRun(run);
            }}
          />
        ) : null
      }
    >
      {data ? (
        <>
          {noPrices ? (
            <Alert
              type="info"
              message="Es gibt noch keine Preise. Ein Lauf braucht für jeden gebuchten Wochentag einen Preis."
              action={
                <ButtonLink
                  href={traegerPath("/traeger/abrechnung/preise")}
                  variant="surface"
                  size="compact"
                >
                  Preisliste öffnen
                </ButtonLink>
              }
            />
          ) : null}
          <DataTable
            columns={runColumns(data.tariffs.schools)}
            rows={[...runs]}
            getRowKey={(run) => run.id}
            stackedOnMobile
            stackedLayout="row"
            stackedDetailKey="schools"
            onRowClick={openRun}
            emptyState={
              <EmptyState
                icon={
                  <CalendarRange className="h-12 w-12" aria-hidden="true" />
                }
                title="Noch kein Monatslauf."
                description={`Starten Sie oben einen Lauf, zum Beispiel für ${formatMonth(data.list.suggestedMonth)}.`}
              />
            }
          />
        </>
      ) : null}
    </TenantPage>
  );
}
