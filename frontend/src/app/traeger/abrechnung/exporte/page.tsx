"use client";

// Exporte für Kämmerei und Steuerbüro (#4693, #4694): je ausgestellten Lauf
// die Sollstellung als CSV mit der Feldzuordnung des Trägers und den
// DATEV-Buchungsstapel, sobald die DATEV-Werte vollständig sind, und das
// Exportprotokoll aller Dateien, auch der Lastschriftdateien. Jeder Abruf
// bekommt die nächste Dateinummer des Trägers. Nur das Recht
// „Finanz-Export“ sieht die Seite.

import { useCallback, useEffect, useState } from "react";
import { billingTabs } from "~/components/traeger/billing-family-format";
import {
  DATEV_SETTINGS_PATH,
  DatevReadinessNotice,
  DatevRefusedNotice,
} from "~/components/traeger/finance-export-datev";
import {
  ExportProtocolTable,
  ExportRunsTable,
  type RunFileFormat,
  runFileKey,
} from "~/components/traeger/finance-export-tables";
import { Button, ButtonLink } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type DatevBookingProblem,
  type DatevReadiness,
  type ExportProtocolEntry,
  type ExportRun,
  type FinanceExportPage,
  type FinanceExportRunPage,
  datevBookingProblems,
  downloadDatevBatch,
  downloadSollstellung,
  fetchExportProtocol,
  fetchExportRuns,
  fetchExportSettings,
} from "~/lib/traeger-finance-exports-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerFinanceExportsPage" });

const INFO =
  "Hier laden Sie die Sollstellung für Ihre Kämmerei und die DATEV-Datei für Ihr Steuerbüro herunter. Welche Spalten die Sollstellung hat, legen Sie unter „Feldzuordnung“ fest. Jede Datei bekommt eine neue Nummer und steht im Protokoll.";

interface ExportsData {
  readonly runs: FinanceExportRunPage;
  readonly protocol: FinanceExportPage<ExportProtocolEntry>;
  readonly nextFileNumber: number;
  readonly datev: DatevReadiness;
}

/** The invoices of a refused DATEV download and the run they belong to. */
interface DatevRefusal {
  readonly runLabel: string;
  readonly problems: readonly DatevBookingProblem[];
}

const FILE_NAMES: Record<RunFileFormat, string> = {
  sollstellung: "die Sollstellung",
  datev: "die DATEV-Datei",
};

/** Joins the next page onto the pages read so far. */
function joinPages<P extends FinanceExportPage<unknown>>(read: P, next: P): P {
  return { ...next, items: [...read.items, ...next.items] };
}

/**
 * Reads pages from the first one until at least `count` entries or the end,
 * so a reload keeps the older pages a person already opened.
 */
async function readPages<P extends FinanceExportPage<unknown>>(
  fetchPage: (offset: number) => Promise<P>,
  count: number,
): Promise<P> {
  let pages = await fetchPage(0);
  while (pages.items.length < count && pages.nextOffset !== null) {
    pages = joinPages(pages, await fetchPage(pages.nextOffset));
  }
  return pages;
}

/** The line under a list that is longer than one page. */
function MorePages({
  shown,
  total,
  noun,
  loading,
  onMore,
}: Readonly<{
  shown: number;
  total: number;
  noun: string;
  loading: boolean;
  onMore: () => void;
}>) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <p className="text-sm text-gray-600">
        Sie sehen {shown} von {total} {noun}.
      </p>
      <Button
        type="button"
        variant="outline"
        size="md"
        isLoading={loading}
        loadingText="Wird geladen …"
        onClick={onMore}
      >
        Ältere anzeigen
      </Button>
    </div>
  );
}

export default function TraegerFinanceExportsPage() {
  const [data, setData] = useState<ExportsData | null>(null);
  const [downloading, setDownloading] = useState<string | null>(null);
  const [refusal, setRefusal] = useState<DatevRefusal | null>(null);
  const [loadingMore, setLoadingMore] = useState<"runs" | "protocol" | null>(
    null,
  );
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const errors = useApiErrorDisplay();
  const { success } = useToast();

  const load = useCallback(
    async (keepRuns = 0, keepProtocol = 0) => {
      try {
        const [runs, protocol, settings] = await Promise.all([
          readPages(fetchExportRuns, keepRuns),
          readPages(fetchExportProtocol, keepProtocol),
          fetchExportSettings(),
        ]);
        setData({
          runs,
          protocol,
          nextFileNumber: settings.nextFileNumber,
          datev: settings.datev,
        });
        clearLoadError();
      } catch (err) {
        logger.error("traeger_finance_exports_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        await showLoadError(err, {
          object: "die Exporte",
          retry: () => void load(),
        });
      }
    },
    [clearLoadError, showLoadError],
  );

  useEffect(() => {
    void load();
  }, [load]);

  const loadMore = async (list: "runs" | "protocol") => {
    if (!data) return;
    const offset =
      list === "runs" ? data.runs.nextOffset : data.protocol.nextOffset;
    if (offset === null) return;
    setLoadingMore(list);
    try {
      if (list === "runs") {
        const next = await fetchExportRuns(offset);
        setData((current) =>
          current
            ? { ...current, runs: joinPages(current.runs, next) }
            : current,
        );
      } else {
        const next = await fetchExportProtocol(offset);
        setData((current) =>
          current
            ? { ...current, protocol: joinPages(current.protocol, next) }
            : current,
        );
      }
    } catch (err) {
      logger.error("traeger_finance_exports_more_failed", {
        list,
        error: err instanceof Error ? err.message : String(err),
      });
      void errors.show(err, {
        object: list === "runs" ? "die älteren Läufe" : "die älteren Dateien",
      });
    } finally {
      setLoadingMore(null);
    }
  };

  const download = async (format: RunFileFormat, run: ExportRun) => {
    setDownloading(runFileKey(format, run));
    setRefusal(null);
    try {
      const file =
        format === "datev"
          ? await downloadDatevBatch(run.pathKind, run.runId)
          : await downloadSollstellung(run.pathKind, run.runId);
      downloadBlob(file.blob, file.filename);
      success(`${file.filename} ist heruntergeladen.`);
    } catch (err) {
      const problems =
        format === "datev" && err instanceof ApiError
          ? datevBookingProblems(err.details)
          : [];
      if (problems.length > 0) {
        // A refusal of the content, not a fault: the notice above the list
        // names every refused invoice and what to do; a toast would repeat
        // it without the names.
        logger.warn("traeger_datev_download_refused", {
          documents: problems.length,
        });
        setRefusal({ runLabel: run.label, problems });
      } else {
        logger.error("traeger_finance_export_download_failed", {
          format,
          error: err instanceof Error ? err.message : String(err),
        });
        void errors.show(err, { object: FILE_NAMES[format] });
      }
    } finally {
      setDownloading(null);
      void load(data?.runs.items.length, data?.protocol.items.length);
    }
  };

  return (
    <TenantPage
      title="Exporte"
      info={INFO}
      tabs={billingTabs("exporte")}
      actions={
        <>
          <ButtonLink
            href={traegerPath(DATEV_SETTINGS_PATH)}
            variant="outline"
            size="md"
          >
            DATEV-Werte
          </ButtonLink>
          <ButtonLink
            href={traegerPath("/traeger/abrechnung/exporte/feldzuordnung")}
            variant="outline"
            size="md"
          >
            Feldzuordnung
          </ButtonLink>
        </>
      }
      stats={
        data ? (
          <TenantPageStats
            items={[
              {
                value: data.runs.total,
                label: data.runs.total === 1 ? "Lauf" : "Läufe",
              },
              { value: data.runs.notExported, label: "noch nicht exportiert" },
              {
                value: `Nr. ${data.nextFileNumber}`,
                label: "ist die nächste Datei",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      error={pageLoad.error}
      loading={!data && !pageLoad.error}
    >
      {data ? (
        <>
          <DatevReadinessNotice readiness={data.datev} />
          {refusal ? (
            <DatevRefusedNotice
              runLabel={refusal.runLabel}
              problems={refusal.problems}
              onDismiss={() => setRefusal(null)}
            />
          ) : null}
          <SectionCard
            title="Dateien je Lauf"
            description="Die Sollstellung ist für Ihre Kämmerei, die DATEV-Datei für Ihr Steuerbüro. Jeder Download ist eine neue Datei mit neuer Nummer."
            flush
            footer={
              data.runs.nextOffset === null ? null : (
                <MorePages
                  shown={data.runs.items.length}
                  total={data.runs.total}
                  noun="Läufen"
                  loading={loadingMore === "runs"}
                  onMore={() => void loadMore("runs")}
                />
              )
            }
          >
            <ExportRunsTable
              runs={data.runs.items}
              downloading={downloading}
              datevReady={data.datev.ready}
              onDownload={(format, run) => void download(format, run)}
            />
          </SectionCard>
          <SectionCard
            title="Exportprotokoll"
            description="Jede Datei, die moto verlassen hat: Sollstellungen, DATEV-Dateien und Lastschriftdateien."
            flush
            footer={
              data.protocol.nextOffset === null ? null : (
                <MorePages
                  shown={data.protocol.items.length}
                  total={data.protocol.total}
                  noun="Dateien"
                  loading={loadingMore === "protocol"}
                  onMore={() => void loadMore("protocol")}
                />
              )
            }
          >
            <ExportProtocolTable entries={data.protocol.items} />
          </SectionCard>
        </>
      ) : null}
    </TenantPage>
  );
}
