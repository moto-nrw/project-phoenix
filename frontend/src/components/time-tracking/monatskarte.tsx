"use client";

// Monatskarte (#1842): the consolidated month summary for the Zeiterfassung
// month view. Shows Übertrag, Soll, Ist, absence credits (Krank/Urlaub
// separately), the Dienstplan coverage (display-only), and the resulting
// Saldo. Everything is computed live by the backend — the Übertrag updates
// automatically when past months are corrected.

import { formatSignedDuration } from "~/components/staff/staff-time-views";
import { formatDuration } from "~/lib/time-tracking-helpers";
import type { MonthSummary } from "~/lib/time-tracking-helpers";

const MONTH_NAMES = [
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
] as const;

export function monthLabel(year: number, month: number): string {
  return `${MONTH_NAMES[month - 1] ?? ""} ${year}`;
}

function formatDays(days: number): string {
  const value = days.toLocaleString("de-DE", { maximumFractionDigits: 1 });
  return `${value} ${days === 1 ? "Tag" : "Tage"}`;
}

function deltaClass(minutes: number): string {
  if (minutes > 0) return "text-[#70b525]";
  if (minutes < 0) return "text-red-600";
  return "text-gray-700";
}

function SummaryRow({
  label,
  value,
  hint,
  valueClass,
  emphasis,
}: {
  readonly label: string;
  readonly value: string;
  readonly hint?: string;
  readonly valueClass?: string;
  readonly emphasis?: boolean;
}) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-1.5">
      <span
        className={
          emphasis
            ? "text-sm font-semibold text-gray-900"
            : "text-sm text-gray-600"
        }
      >
        {label}
        {hint && (
          <span className="ml-2 text-xs font-normal text-gray-400">{hint}</span>
        )}
      </span>
      <span
        className={`text-sm tabular-nums ${emphasis ? "font-bold" : "font-medium"} ${valueClass ?? "text-gray-900"}`}
      >
        {value}
      </span>
    </div>
  );
}

export interface MonatskarteProps {
  readonly summary: MonthSummary | null;
  readonly isLoading: boolean;
  readonly error?: string | null;
  /** True while the viewed month is the current calendar month. */
  readonly isCurrentMonth?: boolean;
}

export function Monatskarte({
  summary,
  isLoading,
  error,
  isCurrentMonth = false,
}: MonatskarteProps) {
  if (isLoading) {
    return (
      <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-6">
        <div className="h-40 animate-pulse rounded-lg bg-gray-100" />
      </div>
    );
  }
  if (error) {
    return (
      <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-6">
        <p className="text-sm text-red-600">{error}</p>
      </div>
    );
  }
  if (!summary) return null;

  const creditedTotal =
    summary.creditedSickMinutes +
    summary.creditedVacationMinutes +
    summary.creditedOtherMinutes;

  const planCoverage = (() => {
    if (summary.plannedShiftMinutes === null) {
      return { value: "kein Dienstplan gepflegt", hint: undefined };
    }
    const diff = summary.plannedShiftMinutes - summary.targetMinutes;
    const hint =
      diff === 0
        ? "deckt das Soll"
        : diff > 0
          ? `${formatDuration(diff)} über Soll verplant`
          : `${formatDuration(Math.abs(diff))} unter Soll verplant`;
    return { value: formatDuration(summary.plannedShiftMinutes), hint };
  })();

  return (
    <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-6">
      <h3 className="mb-3 text-sm font-semibold text-gray-900">
        Monatskarte {monthLabel(summary.year, summary.month)}
      </h3>

      <div className="divide-y divide-gray-100">
        <SummaryRow
          label="Übertrag Vormonat"
          value={formatSignedDuration(summary.carryInMinutes)}
          valueClass={deltaClass(summary.carryInMinutes)}
        />
        <SummaryRow
          label="Summe Soll"
          hint={
            isCurrentMonth
              ? `davon bis heute ${formatDuration(summary.targetMinutesToDate)}`
              : undefined
          }
          value={formatDuration(summary.targetMinutes)}
        />
        <SummaryRow
          label="Summe Ist"
          value={formatDuration(summary.actualMinutes)}
        />
        {creditedTotal > 0 && (
          <>
            {summary.creditedSickMinutes > 0 && (
              <SummaryRow
                label="Gutschrift Krankheit"
                hint={formatDays(summary.sickDays)}
                value={formatDuration(summary.creditedSickMinutes)}
              />
            )}
            {summary.creditedVacationMinutes > 0 && (
              <SummaryRow
                label="Gutschrift Urlaub"
                hint={formatDays(summary.vacationDays)}
                value={formatDuration(summary.creditedVacationMinutes)}
              />
            )}
            {summary.creditedOtherMinutes > 0 && (
              <SummaryRow
                label="Gutschrift Fortbildung/Sonstiges"
                value={formatDuration(summary.creditedOtherMinutes)}
              />
            )}
          </>
        )}
        <SummaryRow
          label="Dienstplan geplant"
          hint={planCoverage.hint}
          value={planCoverage.value}
          valueClass={
            summary.plannedShiftMinutes === null
              ? "text-gray-400"
              : "text-gray-900"
          }
        />
        <SummaryRow
          label={isCurrentMonth ? "Saldo Monat (bis heute)" : "Saldo Monat"}
          value={formatSignedDuration(summary.balanceMinutes)}
          valueClass={deltaClass(summary.balanceMinutes)}
        />
        <SummaryRow
          label={isCurrentMonth ? "Stundenkonto Stand" : "Übertrag Monatsende"}
          value={formatSignedDuration(summary.closingBalanceMinutes)}
          valueClass={deltaClass(summary.closingBalanceMinutes)}
          emphasis
        />
      </div>
    </div>
  );
}
