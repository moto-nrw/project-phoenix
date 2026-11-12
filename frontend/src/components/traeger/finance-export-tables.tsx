"use client";

// Die ausgestellten Läufe mit ihrer letzten Sollstellung und DATEV-Datei
// und das Exportprotokoll des Trägers (#4693, #4694). Jede heruntergeladene Datei steht im
// Protokoll mit ihrer Dateinummer, wer sie wann geholt hat und ob sie eine
// frühere Datei wiederholt.

import { Download, RotateCcw } from "lucide-react";
import { useState } from "react";
import { DATEV_READINESS_ID } from "~/components/traeger/finance-export-datev";
import { formatEuro } from "~/components/traeger/contribution-format";
import {
  FINANCE_FILE_KINDS,
  byLabel,
  exportedAtLabel,
} from "~/components/traeger/finance-export-format";
import { Button } from "~/components/ui/button";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import {
  type CostCenterFallback,
  type DatevRunState,
  type DatevRunStates,
  type ExportProtocolEntry,
  type ExportRun,
  datevRunKey,
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

/**
 * The last file of one format of a run, or that there is none yet. `waiting`
 * names a format whose file is not possible yet (DATEV without its values).
 */
function LastFile({
  entry,
  waiting = false,
}: Readonly<{ entry: ExportProtocolEntry | null; waiting?: boolean }>) {
  if (!entry) {
    return waiting ? (
      <StatusBadge label="Wartet auf DATEV-Werte" tone="gray" />
    ) : (
      <StatusBadge label="Noch nicht exportiert" tone="warning" />
    );
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

/** The id of the line that says why a run has no new DATEV booking. */
function datevStateId(run: ExportRun): string {
  return `datev-state-${run.pathKind}-${run.runId}`;
}

function bookingsLabel(count: number): string {
  return count === 1 ? "1 neue Buchung" : `${count} neue Buchungen`;
}

/**
 * Where the DATEV bookings of a run stand (#6433): how many are new, or that
 * every one is in a file, with the last DATEV file of the run.
 */
function DatevState({
  run,
  state,
  waiting,
}: Readonly<{
  run: ExportRun;
  state: DatevRunState | undefined;
  waiting: boolean;
}>) {
  if (waiting || !state) {
    return <LastFile entry={run.lastDatev} waiting={waiting} />;
  }
  const last = run.lastDatev;
  let badge = (
    <StatusBadge label={bookingsLabel(state.newBookings)} tone="blue" />
  );
  let line = last
    ? `Zuletzt Nr. ${last.fileNumber} am ${formatBerlinDate(last.exportedAt)}`
    : "Noch keine DATEV-Datei";
  if (state.newBookings === 0 && state.fiscalYears.length === 0) {
    badge = <StatusBadge label="Keine Buchung" tone="gray" />;
    line = "Keine Rechnung mit Betrag";
  } else if (state.newBookings === 0) {
    badge = <StatusBadge label="Alles exportiert" tone="green" />;
    line = last
      ? `Zuletzt Nr. ${last.fileNumber} am ${formatBerlinDate(last.exportedAt)}`
      : "Alle Buchungen sind in DATEV-Dateien";
  }
  return (
    <span className="min-w-0">
      <span className="block">{badge}</span>
      <span
        id={datevStateId(run)}
        className="mt-1 block text-sm text-gray-500 md:whitespace-nowrap"
      >
        {line}
      </span>
    </span>
  );
}

/** The DATEV download of a run and its menu with „Alle Buchungen erneut“. */
function DatevActions({
  run,
  state,
  ready,
  downloading,
  onDownload,
  onRepeat,
}: Readonly<{
  run: ExportRun;
  state: DatevRunState | undefined;
  ready: boolean;
  downloading: string | null;
  onDownload: (format: RunFileFormat, run: ExportRun) => void;
  onRepeat: (run: ExportRun) => void;
}>) {
  const nothingNew = state?.newBookings === 0;
  let describedBy: string | undefined;
  if (!ready) describedBy = DATEV_READINESS_ID;
  else if (nothingNew) describedBy = datevStateId(run);
  let blockedReason: string | undefined;
  if (!ready) {
    blockedReason = "Tragen Sie zuerst die fehlenden DATEV-Werte ein.";
  } else if (nothingNew) {
    blockedReason =
      state.fiscalYears.length > 0 && run.lastDatev
        ? "Es gibt keine neue Buchung; eine verlorene Datei holen Sie über das Menü."
        : "Dieser Lauf hat keine Rechnung mit Betrag, also keine Buchung.";
  }
  return (
    <>
      <Button
        type="button"
        variant="surface"
        size="compact"
        isLoading={downloading === runFileKey("datev", run)}
        loadingText="Wird erstellt"
        disabled={downloading !== null || !ready || nothingNew}
        disabledReason={blockedReason}
        tooltip="Lädt die neuen Buchungen als DATEV-Datei (CSV) mit neuer Dateinummer herunter."
        aria-describedby={describedBy}
        aria-label={
          state && state.newBookings > 0
            ? `${bookingsLabel(state.newBookings)} von ${run.label} als DATEV-Datei herunterladen`
            : `DATEV-Datei von ${run.label} herunterladen`
        }
        onClick={() => onDownload("datev", run)}
      >
        <Download className="mr-1.5 h-4 w-4" aria-hidden="true" />
        DATEV
      </Button>
      {ready && state && state.fiscalYears.length > 0 && run.lastDatev ? (
        <OverflowMenu
          ariaLabel={`Weitere DATEV-Aktionen für ${run.label}`}
          disabled={downloading !== null}
          items={[
            {
              label: "Alle Buchungen erneut",
              description:
                "Lädt alle Buchungen erneut, nur für eine verlorene Datei. moto fragt vorher nach.",
              icon: <RotateCcw className="h-4 w-4" aria-hidden />,
              onClick: () => onRepeat(run),
            },
          ]}
        />
      ) : ready ? (
        // Keeps the DATEV buttons of every row in one column.
        <span aria-hidden="true" className="w-9 shrink-0 max-sm:hidden" />
      ) : null}
    </>
  );
}

interface RunTableActions {
  readonly downloading: string | null;
  readonly datev: DatevRunStates & { readonly settingsReady: boolean };
  readonly onDownload: (format: RunFileFormat, run: ExportRun) => void;
  readonly onRepeat: (run: ExportRun) => void;
}

function runColumns({
  downloading,
  datev,
  onDownload,
  onRepeat,
}: RunTableActions): DataTableColumn<ExportRun>[] {
  const datevReady = datev.settingsReady;
  const stateOf = (run: ExportRun) =>
    datev.ready ? datev.runs.get(datevRunKey(run)) : undefined;
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
      header: "DATEV-Datei",
      sortValue: (run) => stateOf(run)?.newBookings ?? null,
      render: (run) => (
        <DatevState run={run} state={stateOf(run)} waiting={!datevReady} />
      ),
    },
    {
      key: "download",
      header: "Datei",
      headerSrOnly: true,
      align: "right",
      stackedLabel: false,
      render: (run) => (
        <span className="flex justify-end gap-2 whitespace-nowrap max-sm:flex-wrap max-sm:justify-start">
          <Button
            type="button"
            variant="surface"
            size="compact"
            isLoading={downloading === runFileKey("sollstellung", run)}
            loadingText="Wird erstellt"
            disabled={downloading !== null}
            aria-label={`Sollstellung von ${run.label} herunterladen`}
            tooltip="Lädt alle Rechnungen des Laufs als CSV-Datei mit neuer Dateinummer herunter."
            onClick={() => onDownload("sollstellung", run)}
          >
            <Download className="mr-1.5 h-4 w-4" aria-hidden="true" />
            Sollstellung
          </Button>
          <DatevActions
            run={run}
            state={stateOf(run)}
            ready={datevReady}
            downloading={downloading}
            onDownload={onDownload}
            onRepeat={onRepeat}
          />
        </span>
      ),
    },
  ];
}

/** The released runs with their last files and their downloads. */
export function ExportRunsTable({
  runs,
  ...actions
}: Readonly<{ runs: readonly ExportRun[] } & RunTableActions>) {
  return (
    <DataTable
      columns={runColumns(actions)}
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
  if (entry.datevAll) {
    notes.push(
      <StatusBadge key="all" label="Alle erneut: Wiederholung" tone="orange" />,
    );
  }
  if (entry.resolvedPositions > 0) {
    // Positions issued before the accounts were kept on them (#4821).
    notes.push(
      <StatusBadge
        key="resolved"
        wrap
        label={`${entry.resolvedPositions} ${entry.resolvedPositions === 1 ? "Position" : "Positionen"} mit Konten vom Exporttag`}
        title="Diese Positionen wurden ausgestellt, bevor moto Konten und Kostenstellen festgehalten hat. Die Datei nutzt die Werte vom Tag des Exports."
        tone="gray"
      />,
    );
  }
  if (entry.costCenterFallbacks.length > 0) {
    const count = entry.costCenterFallbacks.length;
    notes.push(
      <StatusBadge
        key="fallback"
        wrap
        label={`${count} ${count === 1 ? "Eintrag" : "Einträge"} mit Ersatz-Kostenstelle`}
        tone="orange"
      />,
    );
  }
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

const FALLBACK_REASONS: Record<CostCenterFallback["reason"], string> = {
  no_school_cost_center:
    "Die Schule der Woche hat an dem Tag keine Kostenstelle.",
  no_host_school: "Zur Woche gibt es keine Schule.",
};

const FALLBACKS_SHOWN = 5;

/**
 * The holiday weeks of a file that booked on the cost center of the holiday
 * care instead of the school of the week (#6624): document, week, reason and
 * the cost center used.
 */
function CostCenterFallbackList({
  fallbacks,
}: Readonly<{ fallbacks: readonly CostCenterFallback[] }>) {
  const [open, setOpen] = useState(false);
  if (fallbacks.length === 0) return null;
  const shown = open ? fallbacks : fallbacks.slice(0, FALLBACKS_SHOWN);
  return (
    <span className="mt-1.5 block text-sm wrap-anywhere text-gray-600">
      <ul className="space-y-1">
        {shown.map((fallback, index) => (
          <li
            key={`${fallback.number}-${fallback.weekStartsOn ?? ""}-${index}`}
          >
            <span className="font-medium text-gray-900">{fallback.number}</span>
            {fallback.weekStartsOn
              ? `, Woche ab ${formatDate(fallback.weekStartsOn)}`
              : null}
            {": "}
            {FALLBACK_REASONS[fallback.reason]}{" "}
            {fallback.costCenter
              ? `Gebucht auf ${fallback.costCenter}.`
              : "Gebucht ohne Kostenstelle."}
          </li>
        ))}
      </ul>
      {fallbacks.length > FALLBACKS_SHOWN ? (
        <Button
          type="button"
          variant="ghost"
          size="compact"
          aria-expanded={open}
          noTooltip="Das Wort nennt, was der Klick zeigt; nichts ändert sich."
          onClick={() => setOpen((value) => !value)}
        >
          {open
            ? "Weniger anzeigen"
            : `${fallbacks.length - FALLBACKS_SHOWN} weitere anzeigen`}
        </Button>
      ) : null}
      <span className="mt-1 block">
        Für neue Rechnungen: Kostenstelle der Schule unter „Konten und
        Kostenstellen“ eintragen.
      </span>
    </span>
  );
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
        <CostCenterFallbackList fallbacks={entry.costCenterFallbacks} />
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
