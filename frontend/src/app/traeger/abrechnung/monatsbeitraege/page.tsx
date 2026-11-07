"use client";

// Monatsbeiträge des Trägers (#4684): Ein Monatslauf rechnet jeden Monat
// nach dem gültigen Betreuungsvertrag und der Preisliste ab. Die Abrechnung
// prüft die Vorschau und gibt den Lauf frei. Nur die Rolle mit dem Recht
// „Abrechnung“ sieht die Seite.

import { CalendarRange, Plus } from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { billingTabs } from "~/components/traeger/billing-family-format";
import {
  CARE_RUN_STATUS_LABELS,
  CARE_RUN_STATUS_TONES,
  MONTH_RULE_LABELS,
  formatMonth,
  invoiceCountLabel,
} from "~/components/traeger/care-billing-format";
import { CareRunStartModal } from "~/components/traeger/care-run-start-modal";
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
  type CareTariffs,
  fetchCareRuns,
  fetchCareTariffs,
} from "~/lib/traeger-care-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerMonatsbeitraegePage" });

const COLUMNS: DataTableColumn<CareRun>[] = [
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
    stacked: "meta",
  },
  {
    key: "issued",
    header: "Ausgestellt am",
    render: (run) => (run.issuedOn ? formatDate(run.issuedOn) : "–"),
    stacked: "meta",
  },
  {
    key: "rule",
    header: "Monatswechsel",
    render: (run) => (run.monthRule ? MONTH_RULE_LABELS[run.monthRule] : "–"),
    stacked: "meta",
  },
];

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
        object: "die Monatsbeiträge",
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
      title="Abrechnung"
      info="Hier rechnen Sie die Betreuung Monat für Monat ab. Der Beitrag ist die Summe der gebuchten Wochentage im Vertrag, mit den Preisen aus der Preisliste. Sie prüfen die Vorschau und geben den Lauf dann frei."
      stats={
        runs.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: runs.length,
                label: runs.length === 1 ? "Monatslauf" : "Monatsläufe",
              },
              { value: released.length, label: "freigegeben" },
              { value: formatEuro(billedCents), label: "abgerechnet" },
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
            suggestedMonth={data.list.suggestedMonth}
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
            columns={COLUMNS}
            rows={[...runs]}
            getRowKey={(run) => run.id}
            stackedOnMobile
            stackedLayout="row"
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
