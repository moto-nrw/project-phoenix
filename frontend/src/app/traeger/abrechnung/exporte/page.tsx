"use client";

// Exporte für Kämmerei und Steuerbüro (#4693, #4694): je ausgestellten Lauf
// die Sollstellung als CSV mit der Feldzuordnung des Trägers und den
// DATEV-Buchungsstapel, sobald die DATEV-Werte vollständig sind, und das
// Exportprotokoll aller Dateien, auch der Lastschriftdateien. Jeder Abruf
// bekommt die nächste Dateinummer des Trägers. Nur das Recht
// „Finanz-Export“ sieht die Seite.

import { useCallback, useEffect, useMemo, useState } from "react";
import { billingTabs } from "~/components/traeger/billing-family-format";
import { EXPORT_RECIPIENT_PATH } from "~/components/traeger/export-handover-format";
import {
  BOOKING_ACCOUNTS_PATH,
  DATEV_SETTINGS_PATH,
  DatevNothingNewNotice,
  DatevReadinessNotice,
  DatevRefusedNotice,
  DatevRepeatDialog,
} from "~/components/traeger/finance-export-datev";
import {
  type DatevHandoverNotices,
  HandoverMenu,
  useFinanceHandover,
} from "~/components/traeger/finance-export-handover";
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
  type DatevDownloadOptions,
  type DatevReadiness,
  type DatevRunStates,
  type ExportProtocolEntry,
  type ExportRun,
  type FinanceExportPage,
  type FinanceExportRunPage,
  datevBookingProblems,
  downloadDatevBatch,
  datevRunKey,
  downloadSollstellung,
  fetchDatevRunStates,
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
  /** The new DATEV bookings of the loaded runs (#6433). */
  readonly datevRuns: DatevRunStates;
}

/** A DATEV download that found no new booking, and of which run. */
interface DatevNothingNew {
  readonly runLabel: string;
  readonly taken: boolean;
}

const NO_DATEV_RUNS: DatevRunStates = { ready: false, runs: new Map() };

/**
 * The DATEV states of the runs. A failure leaves the table with the last
 * DATEV file of each run, as before #6433, and is logged: the download
 * still names every refusal itself.
 */
async function readDatevRuns(
  runs: readonly ExportRun[],
  ready: boolean,
): Promise<DatevRunStates> {
  if (!ready) return NO_DATEV_RUNS;
  try {
    return await fetchDatevRunStates(runs);
  } catch (err) {
    logger.warn("traeger_datev_runs_load_failed", {
      error: err instanceof Error ? err.message : String(err),
    });
    return NO_DATEV_RUNS;
  }
}

/** The invoices of a refused DATEV download and the run they belong to. */
interface DatevRefusal {
  readonly runKind: ExportRun["runKind"];
  readonly runLabel: string;
  readonly problems: readonly DatevBookingProblem[];
}

const FILE_NAMES: Record<RunFileFormat, string> = {
  sollstellung: "die Sollstellung",
  datev: "die DATEV-Datei",
};

/** The toast after a download; a repeat says that it is one. */
function datevSuccess(
  filename: string,
  format: RunFileFormat,
  run: ExportRun,
  options: DatevDownloadOptions,
): string {
  if (format === "datev" && options.all) {
    return `${filename} ist heruntergeladen. Sie wiederholt alle Buchungen von ${run.label}.`;
  }
  return `${filename} ist heruntergeladen.`;
}

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
  const [nothingNew, setNothingNew] = useState<DatevNothingNew | null>(null);
  const [repeat, setRepeat] = useState<ExportRun | null>(null);
  // „Alle Buchungen erneut“ for the recipient instead of the browser.
  const [repeatHandover, setRepeatHandover] = useState(false);
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
          datevRuns: await readDatevRuns(runs.items, settings.datev.ready),
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

  // A DATEV handover that makes no file shows the notices of the download.
  const runs = data?.runs.items;
  const datevNotices = useMemo<DatevHandoverNotices>(
    () => ({
      onNothingNew: (run, taken) =>
        setNothingNew({ runLabel: run.label, taken }),
      onRefused: (run, problems) =>
        setRefusal({
          runKind:
            runs?.find(
              (item) =>
                item.pathKind === run.pathKind && item.runId === run.runId,
            )?.runKind ?? "care",
          runLabel: run.label,
          problems,
        }),
    }),
    [runs],
  );

  // Handover to the recipient of each format (ADR 0218, ADR 0222); a handover reloads
  // the runs and the protocol like a download.
  const handover = useFinanceHandover(
    () => void load(data?.runs.items.length, data?.protocol.items.length),
    datevNotices,
  );
  const openRepeat = (run: ExportRun, toRecipient: boolean) => {
    setRepeatHandover(toRecipient);
    setRepeat(run);
  };

  const loadMore = async (list: "runs" | "protocol") => {
    if (!data) return;
    const offset =
      list === "runs" ? data.runs.nextOffset : data.protocol.nextOffset;
    if (offset === null) return;
    setLoadingMore(list);
    try {
      if (list === "runs") {
        const next = await fetchExportRuns(offset);
        const datevRuns = await readDatevRuns(next.items, data.datev.ready);
        setData((current) =>
          current
            ? {
                ...current,
                runs: joinPages(current.runs, next),
                datevRuns: {
                  ready: datevRuns.ready,
                  runs: new Map([...current.datevRuns.runs, ...datevRuns.runs]),
                },
              }
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

  const download = async (
    format: RunFileFormat,
    run: ExportRun,
    options: DatevDownloadOptions = {},
  ) => {
    setDownloading(runFileKey(format, run));
    setRefusal(null);
    setNothingNew(null);
    try {
      const file =
        format === "datev"
          ? await downloadDatevBatch(run.pathKind, run.runId, options)
          : await downloadSollstellung(run.pathKind, run.runId);
      downloadBlob(file.blob, file.filename);
      setRepeat(null);
      success(datevSuccess(file.filename, format, run, options));
    } catch (err) {
      const problems =
        format === "datev" && err instanceof ApiError
          ? datevBookingProblems(err.details)
          : [];
      const code =
        format === "datev" && err instanceof ApiError ? err.code : undefined;
      if (
        code === "parent_billing.datev_no_new_bookings" ||
        code === "parent_billing.datev_bookings_taken"
      ) {
        // Nothing new to book: no fault, no number used. The notice above
        // the list says so and the reload shows the run's state.
        setRepeat(null);
        setNothingNew({
          runLabel: run.label,
          taken: code === "parent_billing.datev_bookings_taken",
        });
      } else if (problems.length > 0) {
        // A refusal of the content, not a fault: the notice above the list
        // names every refused invoice and what to do; a toast would repeat
        // it without the names.
        logger.warn("traeger_datev_download_refused", {
          documents: problems.length,
        });
        setRepeat(null);
        setRefusal({ runKind: run.runKind, runLabel: run.label, problems });
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
            href={traegerPath(EXPORT_RECIPIENT_PATH)}
            variant="outline"
            size="md"
          >
            Empfänger der Exporte
          </ButtonLink>
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
          <ButtonLink
            href={traegerPath(BOOKING_ACCOUNTS_PATH)}
            variant="outline"
            size="md"
          >
            Konten und Kostenstellen
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
          {handover.notice}
          {nothingNew ? (
            <DatevNothingNewNotice
              runLabel={nothingNew.runLabel}
              taken={nothingNew.taken}
              onDismiss={() => setNothingNew(null)}
            />
          ) : null}
          {refusal ? (
            <DatevRefusedNotice
              runKind={refusal.runKind}
              runLabel={refusal.runLabel}
              problems={refusal.problems}
              onDismiss={() => setRefusal(null)}
            />
          ) : null}
          <SectionCard
            title="Dateien je Lauf"
            description="Die Sollstellung ist für Ihre Kämmerei, die DATEV-Datei für Ihr Steuerbüro. Die DATEV-Datei enthält nur neue Buchungen."
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
              datev={{ ...data.datevRuns, settingsReady: data.datev.ready }}
              onDownload={(format, run) => void download(format, run)}
              onRepeat={(run) => openRepeat(run, false)}
              renderRunActions={(run) => (
                <HandoverMenu
                  run={run}
                  recipients={handover.recipients}
                  formats={
                    data.datev.ready
                      ? ["sollstellung", "datev"]
                      : ["sollstellung"]
                  }
                  busy={downloading !== null || handover.running !== null}
                  running={
                    handover.isRunning("sollstellung", run) ||
                    handover.isRunning("datev", run)
                  }
                  onHandOver={(format) => handover.handOver(format, run)}
                  onHandOverAll={
                    data.datev.ready &&
                    run.lastDatev &&
                    (data.datevRuns.runs.get(datevRunKey(run))?.fiscalYears
                      .length ?? 0) > 0
                      ? () => openRepeat(run, true)
                      : undefined
                  }
                />
              )}
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
          {handover.dialog}
          <DatevRepeatDialog
            key={repeat ? datevRunKey(repeat) : "closed"}
            runLabel={repeat?.label ?? null}
            fiscalYears={
              repeat
                ? (data.datevRuns.runs.get(datevRunKey(repeat))?.fiscalYears ??
                  [])
                : []
            }
            handover={repeatHandover}
            loading={
              repeat !== null &&
              (repeatHandover
                ? handover.isRunning("datev", repeat)
                : downloading === runFileKey("datev", repeat))
            }
            onClose={() => setRepeat(null)}
            onConfirm={(fiscalYear) => {
              if (!repeat) return;
              if (repeatHandover) {
                // The handover asks again itself when the recipient has it.
                setRepeat(null);
                handover.handOver("datev", repeat, { all: true, fiscalYear });
              } else void download("datev", repeat, { all: true, fiscalYear });
            }}
          />
        </>
      ) : null}
    </TenantPage>
  );
}
