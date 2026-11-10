"use client";

// Die ausgestellten Läufe mit ihrer letzten Sollstellung und DATEV-Datei
// und das Exportprotokoll des Trägers (#4693, #4694). Jede heruntergeladene Datei steht im
// Protokoll mit ihrer Dateinummer, wer sie wann geholt hat und ob sie eine
// frühere Datei wiederholt.

import { Download } from "lucide-react";
import { DATEV_READINESS_ID } from "~/components/traeger/finance-export-datev";
import { formatEuro } from "~/components/traeger/contribution-format";
import {
  FINANCE_FILE_KINDS,
  byLabel,
  exportedAtLabel,
} from "~/components/traeger/finance-export-format";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  ExportProtocolEntry,
  ExportRun,
} from "~/lib/traeger-finance-exports-api";

function euro(cents: number) {
  return <span className="tabular-nums">{formatEuro(cents)}</span>;
}

function FileNumber({ value }: Readonly<{ value: number }>) {
  return (
    <span className="font-semibold text-gray-900 tabular-nums">
      Nr.&nbsp;{value}
    </span>
  );
}

/** The format a run file is downloaded in. */
export type RunFileFormat = "sollstellung" | "datev";

/** The key of a running download: format, kind and run. */
export function runFileKey(format: RunFileFormat, run: ExportRun): string {
  return `${format}/${run.pathKind}/${run.runId}`;
}

/** The last file of one format of a run, or that there is none yet. */
function LastFile({ entry }: Readonly<{ entry: ExportProtocolEntry | null }>) {
  if (!entry) {
    return <StatusBadge label="Noch nicht exportiert" tone="warning" />;
  }
  return (
    <span className="min-w-0">
      <span className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
        <FileNumber value={entry.fileNumber} />
        <span className="text-sm text-gray-500">
          {exportedAtLabel(entry.exportedAt)}
        </span>
      </span>
      <span className="block text-sm text-gray-500">
        {byLabel(entry.exportedByYou, entry.exportedByAccount)}
      </span>
    </span>
  );
}

function runColumns(
  downloading: string | null,
  datevReady: boolean,
  onDownload: (format: RunFileFormat, run: ExportRun) => void,
): DataTableColumn<ExportRun>[] {
  return [
    {
      key: "run",
      header: "Lauf",
      sortValue: (run) => run.issuedOn,
      render: (run) => (
        <span className="min-w-0">
          <span className="block font-medium text-gray-900">{run.label}</span>
          <span className="block text-sm text-gray-500">
            Ausgestellt am {formatDate(run.issuedOn)}
          </span>
        </span>
      ),
      stacked: "title",
    },
    {
      key: "invoices",
      header: "Rechnungen",
      align: "right",
      sortValue: (run) => run.invoiceCount,
      render: (run) => <span className="tabular-nums">{run.invoiceCount}</span>,
    },
    {
      key: "total",
      header: "Summe",
      align: "right",
      sortValue: (run) => run.totalCents,
      render: (run) => euro(run.totalCents),
      stacked: "meta",
    },
    {
      key: "last",
      header: "Letzte Sollstellung",
      sortValue: (run) => run.lastSollstellung?.fileNumber ?? null,
      render: (run) => <LastFile entry={run.lastSollstellung} />,
    },
    {
      key: "datev",
      header: "Letzte DATEV-Datei",
      sortValue: (run) => run.lastDatev?.fileNumber ?? null,
      render: (run) => <LastFile entry={run.lastDatev} />,
    },
    {
      key: "download",
      header: "Datei",
      headerSrOnly: true,
      align: "right",
      stackedLabel: false,
      render: (run) => (
        <span className="flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            variant="surface"
            size="compact"
            isLoading={downloading === runFileKey("sollstellung", run)}
            loadingText="Wird erstellt"
            disabled={downloading !== null}
            aria-label={`Sollstellung von ${run.label} herunterladen`}
            onClick={() => onDownload("sollstellung", run)}
          >
            <Download className="mr-1.5 h-4 w-4" aria-hidden="true" />
            Sollstellung
          </Button>
          <Button
            type="button"
            variant="surface"
            size="compact"
            isLoading={downloading === runFileKey("datev", run)}
            loadingText="Wird erstellt"
            disabled={downloading !== null || !datevReady}
            aria-describedby={datevReady ? undefined : DATEV_READINESS_ID}
            aria-label={`DATEV-Datei von ${run.label} herunterladen`}
            onClick={() => onDownload("datev", run)}
          >
            <Download className="mr-1.5 h-4 w-4" aria-hidden="true" />
            DATEV
          </Button>
        </span>
      ),
    },
  ];
}

/** The released runs with their last files and their downloads. */
export function ExportRunsTable({
  runs,
  downloading,
  datevReady,
  onDownload,
}: Readonly<{
  runs: readonly ExportRun[];
  downloading: string | null;
  datevReady: boolean;
  onDownload: (format: RunFileFormat, run: ExportRun) => void;
}>) {
  return (
    <DataTable
      columns={runColumns(downloading, datevReady, onDownload)}
      rows={[...runs]}
      getRowKey={(run) => `${run.pathKind}/${run.runId}`}
      stackedOnMobile
      emptyState={
        <EmptyState
          title="Noch kein ausgestellter Lauf."
          description="Läufe stehen hier, sobald Sie Rechnungen ausgestellt haben."
        />
      }
    />
  );
}

function entryNotes(entry: ExportProtocolEntry) {
  const notes = [];
  if (entry.repeatOf !== null) {
    notes.push(
      <StatusBadge
        key="repeat"
        label={`Gleicher Inhalt wie Nr. ${entry.repeatOf}`}
        tone="gray"
      />,
    );
  }
  if (entry.submittedAt) {
    notes.push(
      <StatusBadge
        key="submitted"
        label="Bei der Bank eingereicht"
        tone="green"
      />,
    );
  }
  return notes.length > 0 ? (
    <span className="flex flex-wrap gap-1.5">{notes}</span>
  ) : null;
}

const FILE_KIND_TONES = {
  sollstellung: "purple",
  pain008: "blue",
  datev: "cyan",
} as const;

const PROTOCOL_COLUMNS: DataTableColumn<ExportProtocolEntry>[] = [
  {
    key: "number",
    header: "Nr.",
    sortValue: (entry) => entry.fileNumber,
    render: (entry) => <FileNumber value={entry.fileNumber} />,
    stacked: "meta",
  },
  {
    key: "file",
    header: "Datei",
    render: (entry) => (
      <span className="min-w-0">
        <span className="block truncate font-mono text-sm text-gray-900">
          {entry.fileName}
        </span>
        <span className="block text-sm text-gray-500">{entry.runLabel}</span>
        {entryNotes(entry) ? (
          <span className="mt-1.5 block">{entryNotes(entry)}</span>
        ) : null}
      </span>
    ),
    stacked: "title",
  },
  {
    key: "format",
    header: "Art",
    render: (entry) => (
      <StatusBadge
        label={FINANCE_FILE_KINDS[entry.format]}
        tone={FILE_KIND_TONES[entry.format]}
      />
    ),
  },
  {
    key: "invoices",
    header: "Rechnungen",
    align: "right",
    render: (entry) => (
      <span className="tabular-nums">{entry.invoiceCount}</span>
    ),
    stacked: "hidden",
  },
  {
    key: "total",
    header: "Summe",
    align: "right",
    render: (entry) => euro(entry.totalCents),
  },
  {
    key: "exported",
    header: "Heruntergeladen",
    sortValue: (entry) => entry.exportedAt,
    render: (entry) => (
      <span className="min-w-0">
        <span className="block whitespace-nowrap tabular-nums">
          {exportedAtLabel(entry.exportedAt)}
        </span>
        <span className="block text-sm text-gray-500">
          {byLabel(entry.exportedByYou, entry.exportedByAccount)}
        </span>
      </span>
    ),
  },
];

/** Every file that left moto, newest first. */
export function ExportProtocolTable({
  entries,
}: Readonly<{ entries: readonly ExportProtocolEntry[] }>) {
  return (
    <DataTable
      columns={PROTOCOL_COLUMNS}
      rows={[...entries]}
      getRowKey={(entry) => entry.id}
      defaultSortKey="number"
      defaultSortDirection="desc"
      stackedOnMobile
      emptyState={
        <EmptyState
          title="Noch keine Datei heruntergeladen."
          description="Jede Sollstellung, DATEV-Datei und Lastschriftdatei steht hier mit ihrer Nummer."
        />
      }
    />
  );
}
