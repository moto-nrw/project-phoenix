"use client";

// Zeitkonten-Ansicht der /staff-Mitarbeiterliste (#1417 Tranche 2a).
//
// Warum eine Tabelle und nicht die bestehenden Karten: vier Zahlenspalten pro
// Person passen nicht in eine Karte, und der eigentliche Zweck der Ansicht ist
// Sortieren ("wer hat über 20 Plusstunden"). Die Karten bleiben unangetastet
// als Status-Ansicht daneben — beide beantworten verschiedene Fragen und haben
// verschiedene Berechtigungen.
//
// Diese Ansicht wird nur mit time_tracking:manage gerendert; ohne die
// Berechtigung feuert der Request gar nicht erst.

import { useMemo } from "react";
import { ChevronLeft, ChevronRight, Lock } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { Input } from "~/components/ui/input";
import { formatSignedDuration } from "~/components/staff/staff-time-views";
import { formatLocalizedDate } from "~/lib/localized-date-format";
import { employmentTypeLabels } from "~/lib/staff-helpers";
import { formatDuration } from "~/lib/time-tracking-helpers";
import type { TimeAccountRow } from "~/lib/staff-overview-api";

/** Saldo-Presets in Minuten. Trifft den echten Anwendungsfall besser als zwei
 *  freie Zahlenfelder, die niemand ausfüllt; die Freieingabe bleibt darunter
 *  für den Rest. `null` heißt "keine Grenze". */
export const saldoPresets = [
  { id: "all", label: "Alle", min: null, max: null },
  { id: "minus", label: "Minusstunden", min: null, max: -1 },
  { id: "plus", label: "Plusstunden", min: 1, max: null },
  { id: "plus20", label: "über +20 Std.", min: 20 * 60 + 1, max: null },
] as const;

export type SaldoPresetId = (typeof saldoPresets)[number]["id"];

const monthNames = [
  "Januar",
  "Februar",
  "März",
  "April",
  "Mai",
  "Juni",
  "Juli",
  "August",
  "September",
  "Oktober",
  "November",
  "Dezember",
];

function formatOverviewMonth(year: number, month: number): string {
  return `${monthNames[month - 1] ?? month} ${year}`;
}

function saldoClass(minutes: number): string {
  if (minutes < 0) return "text-red-600";
  if (minutes > 0) return "text-[#4a7a15]";
  return "text-gray-600";
}

/** Abschluss-Zustand des angezeigten Monats (#1417). `null` solange der
 *  Status lädt — dann erscheint weder Badge noch Button, statt kurz falsch
 *  "offen" zu behaupten. */
interface MonthCloseState {
  readonly closed: boolean;
  /** Zeitstempel des Abschlusses (erste Snapshot-Zeile), "" wenn offen. */
  readonly closedAt: string;
  readonly closedCount: number;
}

interface Props {
  readonly rows: TimeAccountRow[];
  readonly year: number;
  readonly month: number;
  /** Monatsnavigation: der Endpoint kann seit #1988 jeden vergangenen Monat. */
  readonly onPrevMonth: () => void;
  readonly onNextMonth: () => void;
  readonly canGoNextMonth: boolean;
  readonly monthClose: MonthCloseState | null;
  readonly monthCloseError: string | null;
  readonly onRetryMonthClose: () => void;
  /** True, wenn der angezeigte Monat vorbei ist — nur dann kann er
   *  abgeschlossen werden (Backend: ErrMonthNotClosable). */
  readonly monthIsOver: boolean;
  readonly onCloseMonth: () => void;
  readonly isLoading: boolean;
  readonly error: string | null;
  readonly onRowClick: (row: TimeAccountRow) => void;
  /** Nur die Freieingabe der Saldo-Grenzen; die Presets stehen darüber. */
  readonly saldoPreset: SaldoPresetId;
  readonly onSaldoPresetChange: (preset: SaldoPresetId) => void;
  readonly customSaldoHours: string;
  readonly onCustomSaldoHoursChange: (value: string) => void;
  readonly showCustomSaldo: boolean;
  readonly onShowCustomSaldoChange: (show: boolean) => void;
  readonly paginationResetKey?: string;
}

export function StaffTimeAccountsTable({
  rows,
  year,
  month,
  onPrevMonth,
  onNextMonth,
  canGoNextMonth,
  monthClose,
  monthCloseError,
  onRetryMonthClose,
  monthIsOver,
  onCloseMonth,
  isLoading,
  error,
  onRowClick,
  saldoPreset,
  onSaldoPresetChange,
  customSaldoHours,
  onCustomSaldoHoursChange,
  showCustomSaldo,
  onShowCustomSaldoChange,
  paginationResetKey,
}: Props) {
  const columns = useMemo<DataTableColumn<TimeAccountRow>[]>(
    () => [
      {
        key: "name",
        header: "Name",
        sortValue: (row) => `${row.lastName} ${row.firstName}`,
        render: (row) => (
          <span className="font-medium text-gray-900">
            {row.lastName}, {row.firstName}
          </span>
        ),
      },
      {
        key: "employment",
        header: "Beschäftigung",
        sortValue: (row) => employmentTypeLabels[row.employmentType] ?? "",
        render: (row) => (
          <span className="text-gray-600">
            {employmentTypeLabels[row.employmentType] ?? "–"}
          </span>
        ),
      },
      {
        key: "soll",
        header: "Soll",
        align: "right",
        sortValue: (row) => row.sollMinutes,
        render: (row) => (
          <span className="text-gray-700 tabular-nums">
            {formatDuration(row.sollMinutes)}
          </span>
        ),
      },
      {
        key: "ist",
        header: "Ist",
        align: "right",
        sortValue: (row) => row.istMinutes,
        render: (row) => (
          <span className="text-gray-700 tabular-nums">
            {formatDuration(row.istMinutes)}
          </span>
        ),
      },
      {
        key: "saldo",
        header: "Saldo",
        align: "right",
        sortValue: (row) => row.balanceMinutes,
        render: (row) => (
          <span
            className={`font-semibold tabular-nums ${saldoClass(row.balanceMinutes)}`}
          >
            {formatSignedDuration(row.balanceMinutes)}
          </span>
        ),
      },
      {
        key: "vacation",
        header: "Resturlaub",
        align: "right",
        sortValue: (row) => row.remainingVacationDays,
        render: (row) => (
          <span className="text-gray-700 tabular-nums">
            {row.remainingVacationDays.toLocaleString("de-DE", {
              maximumFractionDigits: 1,
            })}{" "}
            <span className="text-xs text-gray-400">Tage</span>
          </span>
        ),
      },
    ],
    [],
  );

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-1.5">
            <Button
              type="button"
              size="icon"
              variant="ghost"
              onClick={onPrevMonth}
              aria-label="Vorheriger Monat"
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <p className="text-sm font-semibold text-gray-800">
              Zeitkonten — {formatOverviewMonth(year, month)}
            </p>
            <Button
              type="button"
              size="icon"
              variant="ghost"
              onClick={onNextMonth}
              disabled={!canGoNextMonth}
              aria-label="Nächster Monat"
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
            {monthClose?.closed && (
              <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-semibold text-gray-700">
                <Lock className="h-3 w-3" />
                Abgeschlossen am{" "}
                {formatLocalizedDate(monthClose.closedAt, "de")}
              </span>
            )}
          </div>
          <p className="text-xs text-gray-500">
            Soll bis heute, Ist und Saldo aus dem Stundenkonto. Krankheit und
            Urlaub sind mit dem Tagessoll gutgeschrieben.
          </p>
        </div>
        {/* Sichtbare Chip-Leiste statt ghost-Buttons: als ghost sahen die
            inaktiven Presets wie Fließtext aus und niemand erkannte sie als
            Filter. */}
        <div className="flex flex-wrap items-center gap-2">
          {saldoPresets.map((preset) => (
            <Button
              key={preset.id}
              type="button"
              size="compact"
              variant={
                !showCustomSaldo && saldoPreset === preset.id
                  ? "primary"
                  : "outline"
              }
              onClick={() => {
                onCustomSaldoHoursChange("");
                onShowCustomSaldoChange(false);
                onSaldoPresetChange(preset.id);
              }}
            >
              {preset.label}
            </Button>
          ))}
          <Button
            type="button"
            size="compact"
            variant={showCustomSaldo ? "primary" : "outline"}
            onClick={() => {
              onCustomSaldoHoursChange("");
              if (!showCustomSaldo) {
                onSaldoPresetChange("all");
              }
              onShowCustomSaldoChange(!showCustomSaldo);
            }}
            aria-expanded={showCustomSaldo}
          >
            {showCustomSaldo ? "Grenze ausblenden" : "Eigene Grenze"}
          </Button>
          {monthClose !== null && !monthClose.closed && (
            <Button
              type="button"
              size="compact"
              variant="secondary"
              onClick={onCloseMonth}
              disabled={!monthIsOver}
              title={
                monthIsOver
                  ? undefined
                  : "Der laufende Monat kann erst nach Monatsende abgeschlossen werden."
              }
            >
              <Lock className="mr-1 h-3 w-3" />
              Monat abschließen
            </Button>
          )}
          {/* Nach dem Reopen einer einzelnen Person gilt der Monat weiter als
              abgeschlossen (aktive Snapshots der übrigen). Ohne diesen Weg
              ließe sich das wieder geöffnete Konto nie erneut einfrieren; der
              Backend-Abschluss ist idempotent und überspringt bereits
              abgeschlossene Konten. */}
          {monthClose !== null && monthClose.closed && monthIsOver && (
            <Button
              type="button"
              size="compact"
              variant="ghost"
              onClick={onCloseMonth}
            >
              Erneut abschließen
            </Button>
          )}
        </div>
      </div>
      {monthClose !== null && !monthClose.closed && !monthIsOver && (
        <p className="text-xs text-gray-500">
          Der Abschluss friert den Stand zum Monatsende ein. Für den laufenden
          Monat gibt es diesen Stand noch nicht; abschließen geht ab dem 1. des
          Folgemonats.
        </p>
      )}

      {monthCloseError && (
        <div className="flex flex-wrap items-center gap-2">
          <div className="min-w-64 flex-1">
            <Alert type="error" message={monthCloseError} />
          </div>
          <Button
            type="button"
            size="compact"
            variant="outline"
            onClick={onRetryMonthClose}
          >
            Abschlussstatus erneut laden
          </Button>
        </div>
      )}

      {showCustomSaldo && (
        <div className="flex flex-wrap items-center gap-2">
          <label
            htmlFor="saldo-min-hours"
            className="text-xs font-medium text-gray-600"
          >
            Saldo mindestens (Stunden)
          </label>
          <Input
            id="saldo-min-hours"
            type="number"
            inputMode="numeric"
            className="w-28"
            value={customSaldoHours}
            onChange={(event) => onCustomSaldoHoursChange(event.target.value)}
            placeholder="z.B. -10"
          />
        </div>
      )}

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
          {error}
        </div>
      )}

      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(row) => row.staffId}
        onRowClick={onRowClick}
        isLoading={isLoading}
        defaultSortKey="name"
        // Trägerbetrieb mit mehreren hundert Mitarbeitenden: die Tabelle
        // rendert nur die ersten 50 Zeilen und lädt auf Klick nach. Sortiert
        // wird trotzdem über den vollen Datensatz, nicht über die Seite.
        pageSize={50}
        paginationResetKey={paginationResetKey}
        emptyState={
          <div className="py-10 text-center">
            <p className="font-medium text-gray-900">
              Keine Zeitkonten gefunden
            </p>
            <p className="text-sm text-gray-600">
              Filter zurücksetzen oder einen anderen Beschäftigungstyp wählen.
            </p>
          </div>
        }
      />
    </div>
  );
}
