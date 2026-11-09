"use client";

// Exporte für die Kämmerei (#4693): je ausgestellten Lauf die
// Sollstellung als CSV mit der Feldzuordnung des Trägers und das
// Exportprotokoll aller Dateien, auch der Lastschriftdateien. Jeder Abruf
// bekommt die nächste Dateinummer des Trägers. Nur das Recht
// „Finanz-Export“ sieht die Seite.

import { useCallback, useEffect, useState } from "react";
import { billingTabs } from "~/components/traeger/billing-family-format";
import {
  ExportProtocolTable,
  ExportRunsTable,
} from "~/components/traeger/finance-export-tables";
import { ButtonLink } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type ExportProtocolEntry,
  type ExportRun,
  downloadSollstellung,
  fetchExportProtocol,
  fetchExportRuns,
  fetchExportSettings,
} from "~/lib/traeger-finance-exports-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerFinanceExportsPage" });

const INFO =
  "Hier laden Sie die Sollstellung für Ihre Kämmerei herunter. Welche Spalten die Datei hat, legen Sie unter „Feldzuordnung“ fest. Jede Datei bekommt eine neue Nummer und steht im Protokoll.";

interface ExportsData {
  readonly runs: ExportRun[];
  readonly protocol: ExportProtocolEntry[];
  readonly nextFileNumber: number;
}

export default function TraegerFinanceExportsPage() {
  const [data, setData] = useState<ExportsData | null>(null);
  const [downloading, setDownloading] = useState<string | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const errors = useApiErrorDisplay();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      const [runs, protocol, settings] = await Promise.all([
        fetchExportRuns(),
        fetchExportProtocol(),
        fetchExportSettings(),
      ]);
      setData({ runs, protocol, nextFileNumber: settings.nextFileNumber });
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
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const download = async (run: ExportRun) => {
    setDownloading(`${run.pathKind}/${run.runId}`);
    try {
      const file = await downloadSollstellung(run.pathKind, run.runId);
      downloadBlob(file.blob, file.filename);
      success(`${file.filename} ist heruntergeladen.`);
    } catch (err) {
      logger.error("traeger_sollstellung_download_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errors.show(err, { object: "die Sollstellung" });
    } finally {
      setDownloading(null);
      void load();
    }
  };

  const open = data?.runs.filter((run) => !run.lastSollstellung).length ?? 0;

  return (
    <TenantPage
      title="Exporte"
      info={INFO}
      tabs={billingTabs("exporte")}
      actions={
        <ButtonLink
          href={traegerPath("/traeger/abrechnung/exporte/feldzuordnung")}
          variant="outline"
          size="md"
        >
          Feldzuordnung
        </ButtonLink>
      }
      stats={
        data ? (
          <TenantPageStats
            items={[
              {
                value: data.runs.length,
                label: data.runs.length === 1 ? "Lauf" : "Läufe",
              },
              { value: open, label: "noch nicht exportiert" },
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
          <SectionCard
            title="Sollstellung je Lauf"
            description="Eine Zeile je Rechnung, Storno und Gutschrift des Laufs. Jeder Download ist eine neue Datei mit neuer Nummer."
            flush
          >
            <ExportRunsTable
              runs={data.runs}
              downloading={downloading}
              onDownload={(run) => void download(run)}
            />
          </SectionCard>
          <SectionCard
            title="Exportprotokoll"
            description="Jede Datei, die moto verlassen hat: Sollstellungen und Lastschriftdateien."
            flush
          >
            <ExportProtocolTable entries={data.protocol} />
          </SectionCard>
        </>
      ) : null}
    </TenantPage>
  );
}
