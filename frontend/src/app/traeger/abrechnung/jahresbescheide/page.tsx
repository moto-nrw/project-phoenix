"use client";

// Jahresbescheide des Trägers (#6219): Ein Jahreslauf setzt den Beitrag für
// die Monate eines Jahres in einem Gebührenbescheid je Vertrag fest, mit
// Monatsraten zum Monatsersten. Für diese Monate stellt der Monatslauf keine
// Betreuungsrechnung aus. Nur die Rolle mit dem Recht „Abrechnung“ sieht die Seite.

import { FileText, Plus } from "lucide-react";
import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { billingTabs } from "~/components/traeger/billing-family-format";
import {
  NOTICE_RUN_STATUS_LABELS,
  NOTICE_RUN_STATUS_TONES,
  noticeCountLabel,
  noticePeriod,
} from "~/components/traeger/care-notice-format";
import { formatMonth } from "~/components/traeger/care-billing-format";
import { runSchoolsLabel } from "~/components/traeger/care-review-format";
import { formatEuro } from "~/components/traeger/contribution-format";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareSchool,
  fetchCareTariffs,
} from "~/lib/traeger-care-billing-api";
import {
  type CareNoticeRun,
  type CareNoticeRunList,
  fetchCareNoticeRuns,
  startCareNoticeRun,
} from "~/lib/traeger-care-notices-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerJahresbescheidePage" });

// The start dialog loads when it opens.
const CareNoticeRunStartModal = dynamic(
  () =>
    import("~/components/traeger/care-notice-run-start-modal").then(
      (module) => module.CareNoticeRunStartModal,
    ),
  { ssr: false },
);

function runColumns(
  schools: readonly CareSchool[],
): DataTableColumn<CareNoticeRun>[] {
  return [
    {
      key: "period",
      header: "Zeitraum",
      render: (run) => (
        <span className="font-medium text-gray-900 tabular-nums">
          {noticePeriod(run.periodFrom, run.periodUntil)}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "status",
      header: "Status",
      render: (run) => (
        <StatusBadge
          label={
            run.status === "released" && run.noticeCount === 0
              ? "Abgeschlossen"
              : NOTICE_RUN_STATUS_LABELS[run.status]
          }
          tone={NOTICE_RUN_STATUS_TONES[run.status]}
        />
      ),
      stacked: "meta",
    },
    {
      key: "schools",
      header: "Schulen",
      render: (run) => (
        <span className="text-gray-700">
          {runSchoolsLabel(run.schoolIds, schools)}
        </span>
      ),
      stacked: "meta",
    },
    {
      key: "notices",
      header: "Bescheide",
      render: (run) =>
        run.noticeCount === null ? (
          "–"
        ) : (
          <span className="tabular-nums">
            {noticeCountLabel(run.noticeCount)} ·{" "}
            {formatEuro(run.totalCents ?? 0)}
          </span>
        ),
      stacked: "meta",
    },
    {
      key: "issued",
      header: "Abgeschlossen am",
      render: (run) => (run.issuedOn ? formatDate(run.issuedOn) : "–"),
      stacked: "meta",
    },
  ];
}

interface PageData {
  readonly list: CareNoticeRunList;
  readonly schools: readonly CareSchool[];
}

export default function TraegerJahresbescheidePage() {
  const router = useRouter();
  const [data, setData] = useState<PageData | null>(null);
  const [starting, setStarting] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      const [list, tariffs] = await Promise.all([
        fetchCareNoticeRuns(),
        fetchCareTariffs(),
      ]);
      setData({ list, schools: tariffs.schools });
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_notice_runs_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Jahresbescheide",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const openRun = (run: CareNoticeRun) =>
    router.push(traegerPath(`/traeger/abrechnung/jahresbescheide/${run.id}`));

  const runs = data?.list.runs ?? [];
  const released = runs.filter((run) => run.status === "released");
  const notices = released.reduce(
    (sum, run) => sum + (run.noticeCount ?? 0),
    0,
  );

  return (
    <TenantPage
      title="Abrechnung"
      info="Hier setzen Sie den Beitrag für mehrere Monate eines Jahres in einem Bescheid fest. Jede Familie bekommt je Vertrag einen Bescheid mit Monatsraten. Für diese Monate berechnet der Monatslauf keine Betreuung erneut. Essen rechnet er weiterhin ab."
      stats={
        runs.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: runs.length,
                label: runs.length === 1 ? "Jahreslauf" : "Jahresläufe",
              },
              { value: notices, label: "Bescheide ausgestellt" },
            ]}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      tabs={billingTabs("jahresbescheide")}
      actions={
        <Button
          type="button"
          variant="primary"
          size="md"
          disabled={!data}
          tooltip="Öffnet ein Fenster, um einen Jahreslauf für Jahr und Schulen zu starten."
          onClick={() => setStarting(true)}
        >
          <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
          Jahreslauf starten
        </Button>
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !data}
      overlays={
        data ? (
          <CareNoticeRunStartModal
            isOpen={starting}
            schools={data.schools}
            initialMonth={data.list.suggestedMonth}
            suggestedYear={data.list.suggestedYear}
            onClose={() => setStarting(false)}
            onStart={startCareNoticeRun}
            onStarted={(run: CareNoticeRun) => {
              setStarting(false);
              openRun(run);
            }}
          />
        ) : null
      }
    >
      {data ? (
        <DataTable
          columns={runColumns(data.schools)}
          rows={[...runs]}
          getRowKey={(run) => run.id}
          stackedOnMobile
          stackedLayout="row"
          onRowClick={openRun}
          emptyState={
            <EmptyState
              icon={<FileText className="h-12 w-12" aria-hidden="true" />}
              title="Noch kein Jahreslauf."
              description={`Starten Sie oben einen Lauf, zum Beispiel ab ${formatMonth(data.list.suggestedMonth)}.`}
            />
          }
        />
      ) : null}
    </TenantPage>
  );
}
