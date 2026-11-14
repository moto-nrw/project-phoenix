"use client";

// Vorschau des Vertragsimports (#4723): jede Zeile der Datei mit ihrem Stand,
// dem Kind, der Betreuung, den Eltern und dem, was sie aufhält. Übernommen
// werden nur die bereiten Zeilen; ein Kind mit laufendem Vertrag bekommt
// keinen zweiten.

import { useMemo, useState } from "react";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  ContractImportResult,
  ContractImportRow,
} from "~/lib/traeger-care-contract-import-api";
import {
  CONTRACT_IMPORT_STATUS,
  type ContractImportFilter,
  contractCountLabel,
  contractImportChild,
  contractImportDebtorText,
  contractImportHintText,
  contractImportPeriod,
  contractImportProblemText,
  contractImportWeekdays,
  matchesContractImportFilter,
} from "./care-contract-import-format";

const IMPORT_PAGE_SIZE = 50;

function NotesCell({ row }: Readonly<{ row: ContractImportRow }>) {
  const lines = row.problems.map((problem) =>
    contractImportProblemText(problem, row),
  );
  if (row.status === "duplicate" && !row.hints.includes("imported_before")) {
    lines.unshift("Das Kind hat an dieser Schule schon einen Vertrag.");
  }
  // A hint matters only where the row leads somewhere: a moved start for a
  // row moto takes over, an earlier import for the row it leaves.
  const ready = row.status === "ready" || row.status === "imported";
  lines.push(
    ...row.hints
      .filter((hint) => (hint === "start_moved" ? ready : true))
      .map(contractImportHintText),
  );
  const debtor = contractImportDebtorText(row);
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

function footerText(result: ContractImportResult): string {
  const { counts } = result;
  const left = counts.conflict + counts.invalid;
  if (counts.ready === 0 && counts.duplicate > 0 && left === 0) {
    return "Alle Kinder der Datei haben schon ihren Vertrag.";
  }
  if (left > 0) {
    return `${left === 1 ? "1 Zeile bleibt" : `${left} Zeilen bleiben`} draußen. Korrigieren Sie die Datei und laden Sie sie noch einmal.`;
  }
  return "Die Verträge gelten sofort als abgeschlossen. Eltern bekommen keine Mail.";
}

export function CareContractImportPreviewCard({
  result,
  applying,
  disabled = false,
  onApply,
}: Readonly<{
  result: ContractImportResult;
  applying: boolean;
  disabled?: boolean;
  onApply: () => void;
}>) {
  const [filter, setFilter] = useState<ContractImportFilter>("all");
  const counts = result.counts;
  const shown = useMemo(
    () => result.rows.filter((row) => matchesContractImportFilter(row, filter)),
    [result.rows, filter],
  );
  const filters: { value: ContractImportFilter; label: string }[] = [
    { value: "all", label: `Alle ${result.rows.length}` },
    { value: "ready", label: `Bereit ${counts.ready + counts.imported}` },
    { value: "problems", label: `Fehler ${counts.conflict + counts.invalid}` },
    { value: "duplicate", label: `Vorhanden ${counts.duplicate}` },
  ];

  const columns: DataTableColumn<ContractImportRow>[] = [
    {
      key: "child",
      header: "Kind",
      render: (row) => (
        <span className="block min-w-0">
          <span className="block font-medium wrap-anywhere text-gray-900">
            {contractImportChild(row)}
          </span>
          <span className="block text-xs text-gray-500 sm:whitespace-nowrap">
            {`Zeile ${row.line} · ${
              row.childBirthDate
                ? `geb. ${formatDate(row.childBirthDate)}`
                : "ohne Geburtsdatum"
            }`}
          </span>
        </span>
      ),
      // The file's own order is the start; the name sorts on request.
      sortValue: (row) => `${row.childLastName} ${row.childFirstName}`,
      stacked: "title",
    },
    {
      key: "status",
      header: "Stand",
      render: (row) => (
        <StatusBadge
          tone={CONTRACT_IMPORT_STATUS[row.status].tone}
          label={CONTRACT_IMPORT_STATUS[row.status].label}
        />
      ),
      stacked: "meta",
    },
    {
      key: "care",
      header: "Betreuung",
      render: (row) => (
        <span className="block min-w-0">
          <span className="block wrap-anywhere text-gray-900">
            {row.moduleNames.length > 0 ? row.moduleNames.join(" und ") : "–"}
          </span>
          <span className="block text-xs wrap-anywhere text-gray-500">
            {row.schoolName || "–"}
            {row.weekdays.length > 0 ? (
              <span className="whitespace-nowrap">
                {` · ${contractImportWeekdays(row.weekdays)}`}
              </span>
            ) : null}
          </span>
        </span>
      ),
      sortValue: (row) => row.schoolName,
    },
    {
      key: "period",
      header: "Vertrag",
      render: (row) => (
        <span className="sm:whitespace-nowrap">
          {contractImportPeriod(row)}
        </span>
      ),
      sortValue: (row) => row.effectiveFrom,
    },
    {
      key: "party",
      header: "Eltern",
      render: (row) => (
        <span className="block min-w-0">
          <span className="block wrap-anywhere text-gray-900">
            {row.partyName || "–"}
          </span>
          <span className="block text-xs wrap-anywhere text-gray-500">
            {row.partyEmail || "ohne E-Mail"}
          </span>
        </span>
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
          : "So würde moto jede Zeile übernehmen. Gespeichert wird erst, wenn Sie übernehmen."
      }
      footer={
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-gray-600">{footerText(result)}</p>
          <Button
            type="button"
            variant="success"
            size="md"
            onClick={onApply}
            disabled={disabled || counts.ready === 0}
            disabledReason={
              disabled
                ? "Übernehmen geht erst, wenn moto die Datei geprüft hat."
                : "In der Datei ist keine Zeile bereit zum Übernehmen."
            }
            tooltip="Legt die bereiten Zeilen sofort als abgeschlossene Verträge an."
            isLoading={applying}
            loadingText="Wird übernommen …"
          >
            {contractCountLabel(counts.ready)} übernehmen
          </Button>
        </div>
      }
    >
      <div className="mb-4">
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
