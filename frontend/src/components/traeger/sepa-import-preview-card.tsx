"use client";

// Vorschau des Mandatsimports (#6198): jede Zeile der Datei mit ihrem Stand,
// der Familie, dem Konto (nur verdeckt) und dem, was sie aufhält. Zeilen, bei
// denen die Datei nicht sagt, ob schon abgebucht wurde, beantwortet das Büro
// hier; moto rät das nie. Übernommen werden nur die bereiten Zeilen.

import { useEffect, useMemo, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { MandateSequence } from "~/lib/traeger-sepa-api";
import type { ImportResult, ImportRow } from "~/lib/traeger-sepa-import-api";
import {
  IMPORT_STATUS,
  type ImportFilter,
  importDebtorNumberText,
  importChangesLabel,
  importableCount,
  importHintText,
  importPayerName,
  importProblemText,
  matchesImportFilter,
} from "./sepa-import-format";

const IMPORT_PAGE_SIZE = 50;

type Answers = Readonly<Record<number, MandateSequence>>;

const ANSWER_ITEMS: { value: MandateSequence; label: string }[] = [
  { value: "first", label: "Nein" },
  { value: "recurring", label: "Ja" },
];

function SequenceCell({
  row,
  answer,
  disabled,
  onAnswer,
}: Readonly<{
  row: ImportRow;
  answer: MandateSequence | undefined;
  disabled: boolean;
  onAnswer: (line: number, sequence: MandateSequence) => void;
}>) {
  if (row.status === "clarify" || answer !== undefined) {
    return (
      <span className="block w-full">
        {/* While the table is stacked (below md) the question stands above the full-width choice. */}
        <span className="mb-1.5 block text-sm text-gray-600 md:hidden">
          Schon abgebucht?
        </span>
        <SegmentedControl
          ariaLabel={`Zeile ${row.line}: Wurde schon abgebucht?`}
          items={ANSWER_ITEMS.map((item) => ({ ...item, disabled }))}
          value={answer ?? null}
          onChange={(sequence) => onAnswer(row.line, sequence)}
          fullWidth
        />
      </span>
    );
  }
  if (row.sequence === "") return <span className="text-gray-400">–</span>;
  return (
    <span className="block">
      <span className="text-gray-900">
        {row.sequence === "recurring"
          ? "Ja, Folgelastschrift"
          : "Nein, Erstlastschrift"}
      </span>
      {row.lastCollectedOn ? (
        <span className="block text-xs text-gray-500">
          zuletzt {formatDate(row.lastCollectedOn)}
        </span>
      ) : null}
    </span>
  );
}

function NotesCell({ row }: Readonly<{ row: ImportRow }>) {
  const lines = [
    ...row.problems.map(importProblemText),
    // A hint matters only for a row the import can take over; a sibling's
    // row of the same mandate is not one (#7009).
    ...(row.problems.length === 0 && row.status !== "same_mandate"
      ? row.hints.map(importHintText)
      : []),
  ];
  if (row.status === "duplicate") lines.unshift("Dieses Mandat gibt es schon.");
  if (row.status === "same_mandate" && row.sameMandateLine !== null) {
    // Siblings of one family share its mandate (#7009).
    lines.unshift(
      `Gleiches Mandat wie Zeile ${row.sameMandateLine}. moto legt es einmal an.`,
    );
  }
  const debtor = importDebtorNumberText(row);
  if (debtor) lines.push(debtor);
  if (lines.length === 0) return <span className="text-gray-400">–</span>;
  return (
    <ul className="space-y-0.5 text-sm text-gray-700">
      {lines.map((line) => (
        <li key={line}>{line}</li>
      ))}
    </ul>
  );
}

/**
 * Rows without IBAN and reference are children whose family pays without
 * a mandate (#7009): only number conflicts appear as rows.
 */
function withoutMandateText(count: number): string {
  return count === 1
    ? "1 Zeile ohne Mandat: Sie hat keine IBAN und keine Mandatsreferenz. moto legt dafür kein Mandat an. Fehlende Debitorennummern werden übernommen, solange es keinen Konflikt gibt. Konflikte sehen Sie mit Zeilennummer unten."
    : `${count} Zeilen ohne Mandat: Sie haben keine IBAN und keine Mandatsreferenz. moto legt dafür kein Mandat an. Fehlende Debitorennummern werden übernommen, solange es keinen Konflikt gibt. Konflikte sehen Sie mit Zeilennummer unten.`;
}

export function SepaImportPreviewCard({
  result,
  answers,
  applying,
  disabled = false,
  onAnswer,
  onApply,
}: Readonly<{
  result: ImportResult;
  answers: Answers;
  applying: boolean;
  disabled?: boolean;
  onAnswer: (line: number, sequence: MandateSequence) => void;
  onApply: () => void;
}>) {
  const [filter, setFilter] = useState<ImportFilter>("all");
  const filtersRef = useRef<HTMLDivElement>(null);
  const answerFocus = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (disabled || !answerFocus.current) return;
    const previous = answerFocus.current;
    answerFocus.current = null;
    // Do not steal focus if the person moved elsewhere during the check.
    if (
      document.activeElement !== document.body &&
      document.activeElement !== previous
    )
      return;
    // An answered row may have left the current filter.
    const target = previous.isConnected
      ? previous
      : filtersRef.current?.querySelector<HTMLElement>('[aria-pressed="true"]');
    target?.focus({ preventScroll: true });
  }, [disabled]);

  const answer = (line: number, sequence: MandateSequence) => {
    answerFocus.current =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    onAnswer(line, sequence);
  };
  const counts = result.counts;
  const importable = importableCount(result);
  const openQuestions = counts.clarify;
  const shown = useMemo(
    () => result.rows.filter((row) => matchesImportFilter(row, filter)),
    [result.rows, filter],
  );
  const filters: { value: ImportFilter; label: string }[] = [
    { value: "all", label: `Alle ${result.rows.length}` },
    { value: "ready", label: `Bereit ${counts.ready + counts.imported}` },
    { value: "clarify", label: `Klären ${counts.clarify}` },
    { value: "problems", label: `Fehler ${counts.conflict + counts.invalid}` },
    {
      value: "duplicate",
      label: `Vorhanden ${counts.duplicate + counts.same_mandate}`,
    },
  ];

  const columns: DataTableColumn<ImportRow>[] = [
    {
      key: "line",
      header: "Zeile",
      render: (row) => (
        <span className="text-gray-500 tabular-nums">{row.line}</span>
      ),
      sortValue: (row) => row.line,
    },
    {
      key: "family",
      header: "Familie",
      render: (row) => (
        <span className="block min-w-0">
          <span className="block font-medium wrap-anywhere text-gray-900">
            {importPayerName(row)}
          </span>
          <span className="block text-xs wrap-anywhere text-gray-500">
            {row.reference || "–"}
          </span>
        </span>
      ),
      sortValue: (row) => importPayerName(row),
      stacked: "title",
    },
    {
      key: "status",
      header: "Stand",
      render: (row) => (
        <StatusBadge
          tone={IMPORT_STATUS[row.status].tone}
          label={IMPORT_STATUS[row.status].label}
        />
      ),
      stacked: "meta",
    },
    {
      key: "signed",
      header: "Unterschrieben",
      render: (row) => (row.signedOn ? formatDate(row.signedOn) : "–"),
      sortValue: (row) => row.signedOn,
    },
    {
      key: "account",
      header: "Konto",
      render: (row) => (
        <span className="block">
          <span className="block font-mono text-sm text-gray-900 sm:whitespace-nowrap">
            {row.ibanMasked || "–"}
          </span>
          <span className="block text-xs wrap-anywhere text-gray-500">
            {row.accountHolder || "–"}
          </span>
        </span>
      ),
    },
    {
      key: "sequence",
      header: "Schon abgebucht?",
      // The answer needs the full row width on a phone; the cell names
      // its question itself there.
      stackedLabel: false,
      render: (row) => (
        <SequenceCell
          row={row}
          answer={answers[row.line]}
          disabled={applying || disabled}
          onAnswer={answer}
        />
      ),
    },
    {
      key: "notes",
      header: "Hinweis",
      render: (row) => <NotesCell row={row} />,
    },
  ];

  return (
    <SectionCard
      title="Prüfen und übernehmen"
      description={
        disabled
          ? "Die Vorschau zeigt den letzten geprüften Stand. Übernehmen ist erst nach der Prüfung möglich."
          : "So würde moto jede Zeile übernehmen. Geändert wird erst, wenn Sie übernehmen."
      }
      footer={
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-gray-600">
            {openQuestions > 0
              ? `Noch ${openQuestions} ${openQuestions === 1 ? "Zeile" : "Zeilen"} offen. Offene Zeilen bleiben draußen.`
              : "Mandate mit Fehlern bleiben draußen. Debitorennummern prüft moto getrennt."}
          </p>
          <Button
            type="button"
            variant="success"
            size="md"
            onClick={onApply}
            disabled={disabled || importable === 0}
            disabledReason={
              disabled
                ? "Übernehmen geht erst, wenn moto die Datei geprüft hat."
                : "In der Datei ist keine Zeile bereit zum Übernehmen."
            }
            tooltip="Übernimmt die bereiten Zeilen sofort in die Verträge der Familien."
            isLoading={applying}
            loadingText="Wird übernommen …"
          >
            {importChangesLabel(counts.ready, counts.debtorNumbers)} übernehmen
          </Button>
        </div>
      }
    >
      {counts.withoutMandate > 0 ? (
        <Alert
          type="info"
          className="mb-4"
          message={withoutMandateText(counts.withoutMandate)}
        />
      ) : null}
      <div ref={filtersRef} className="mb-4">
        <SegmentedControl
          ariaLabel="Zeilen zeigen"
          fullWidth
          wrap
          items={filters}
          value={filter}
          onChange={setFilter}
        />
      </div>
      <DataTable
        columns={columns}
        rows={[...shown]}
        getRowKey={(row) => row.line}
        stackedOnMobile
        stackedLayout="lines"
        dense
        // A file may hold 5.000 rows: show them in pages; the counts and the
        // import still cover every row.
        pageSize={IMPORT_PAGE_SIZE}
        paginationResetKey={filter}
        emptyState={
          <EmptyState
            variant="compact"
            title="Keine Zeilen"
            description="In dieser Auswahl gibt es keine Zeile."
          />
        }
      />
    </SectionCard>
  );
}
