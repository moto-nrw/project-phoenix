"use client";

// Monatskarte (#1842): the consolidated month summary for the Zeiterfassung
// month view. Shows Übertrag, Soll, Ist, absence credits (Krank/Urlaub
// separately), the Dienstplan coverage (display-only), and the resulting
// Saldo. Everything is computed live by the backend — the Übertrag updates
// automatically when past months are corrected.

import { Lock } from "lucide-react";

import { Button } from "~/components/ui/button";
import { formatSignedDuration } from "~/components/staff/staff-time-views";
import {
  balanceAdjustmentTypeLabel,
  formatDuration,
} from "~/lib/time-tracking-helpers";
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
  /**
   * True when the viewed month lies entirely BEFORE the configured account
   * start. The backend then summarizes the month standalone — full month Soll,
   * zero carry — and deliberately keeps its closing value out of the account
   * chain, so the carry rows describe nothing and must not be shown.
   */
  readonly isPreAccountMonth?: boolean;
  /**
   * True when the configured account start lies in the FUTURE (a later day of
   * the current month, so the month is NOT wholly pre-account). The account
   * hasn't begun, so its closing value is not yet a real Stundenkonto — the
   * backend answers a clean 0 for the empty span before the start day, which
   * must not be printed as "Stundenkonto Stand: 0:00" before the account
   * exists (#1842). Distinct from `isPreAccountMonth`, which covers a month
   * lying entirely before the start.
   */
  readonly accountStartsInFuture?: boolean;
  /** Configured account start ("YYYY-MM-DD"), for the pre-account note. */
  readonly accountStartDate?: string;
  /**
   * Admin-only (#1417): renders the "Monat wieder öffnen" action on a closed
   * month. The MA-facing own view passes nothing and stays read-only.
   */
  readonly onReopen?: () => void;
}

function formatIsoDateGerman(iso: string): string {
  const [year, month, day] = iso.slice(0, 10).split("-");
  return year && month && day ? `${day}.${month}.${year}` : iso;
}

export function Monatskarte({
  summary,
  isLoading,
  error,
  isCurrentMonth = false,
  isPreAccountMonth = false,
  accountStartsInFuture = false,
  accountStartDate,
  onReopen,
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
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <h3 className="text-sm font-semibold text-gray-900">
          Monatskarte {monthLabel(summary.year, summary.month)}
        </h3>
        {summary.isClosed && (
          <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-semibold text-gray-700">
            <Lock className="h-3 w-3" />
            Abgeschlossen
            {summary.closedAt
              ? ` am ${formatIsoDateGerman(summary.closedAt)}`
              : ""}
          </span>
        )}
      </div>

      <div className="divide-y divide-gray-100">
        {!isPreAccountMonth && (
          <SummaryRow
            label="Übertrag Vormonat"
            value={formatSignedDuration(summary.carryInMinutes)}
            valueClass={deltaClass(summary.carryInMinutes)}
          />
        )}
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
        {/* Stundenkonto-Buchungen (#1420): Auszahlung, Freizeitausgleich,
            Reset — eine Zeile pro Buchung, damit die Kette Soll → Ist →
            Anpassungen → Saldo von oben nach unten nachrechenbar bleibt. */}
        {summary.adjustments.map((adjustment) => (
          <SummaryRow
            key={adjustment.id}
            label={balanceAdjustmentTypeLabel(adjustment.type)}
            hint={formatIsoDateGerman(adjustment.effectiveDate)}
            value={formatSignedDuration(adjustment.minutesDelta)}
            valueClass={deltaClass(adjustment.minutesDelta)}
          />
        ))}
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
          emphasis={isPreAccountMonth}
        />
        {/* For a pre-account month closingBalanceMinutes is just this month's
            own Saldo repeated — it carries nothing in and feeds nothing on.
            And before a same-month future start the account has not begun, so
            the value is a clean 0 for an empty span. Printing either as
            "Übertrag Monatsende" or "Stundenkonto Stand" would present a
            non-cumulative figure as a real balance. */}
        {!isPreAccountMonth &&
          !accountStartsInFuture &&
          (summary.isClosed && summary.frozenClosingBalanceMinutes !== null ? (
            // Abgeschlossener Monat: der Folgemonat rechnet mit dem
            // EINGEFRORENEN Wert weiter, also ist der die verbindliche Zahl.
            // Die live gerechneten Zeilen darüber bleiben stehen; weicht ihre
            // Summe ab, erklärt der Hinweis darunter die Differenz.
            <SummaryRow
              label="Übertrag Monatsende (eingefroren)"
              value={formatSignedDuration(summary.frozenClosingBalanceMinutes)}
              valueClass={deltaClass(summary.frozenClosingBalanceMinutes)}
              emphasis
            />
          ) : (
            <SummaryRow
              label={
                isCurrentMonth ? "Stundenkonto Stand" : "Übertrag Monatsende"
              }
              value={formatSignedDuration(summary.closingBalanceMinutes)}
              valueClass={deltaClass(summary.closingBalanceMinutes)}
              emphasis
            />
          ))}
      </div>

      {summary.isClosed && summary.driftMinutes !== 0 && (
        <div className="mt-3 rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
          <p className="font-semibold">
            Seit dem Abschluss wurden Zeiten in diesem Monat geändert.
          </p>
          <p className="mt-1">
            Neu gerechnet wäre der Übertrag{" "}
            <span className="font-medium tabular-nums">
              {formatSignedDuration(summary.closingBalanceMinutes)}
            </span>{" "}
            ({formatSignedDuration(summary.driftMinutes)} gegenüber dem
            abgeschlossenen Stand). Der Übertrag in den Folgemonat bleibt beim
            eingefrorenen Wert.
          </p>
          <p className="mt-1">
            Zwei Wege zur Korrektur: die Differenz im offenen Monat als Buchung
            im Stundenkonto erfassen (Übersicht-Tab, empfohlen), oder den Monat
            wieder öffnen, wenn der Abschluss selbst falsch war.
          </p>
        </div>
      )}

      {summary.isClosed && onReopen && (
        <div className="mt-3 flex justify-end">
          <Button
            type="button"
            size="compact"
            variant="ghost"
            onClick={onReopen}
          >
            Monat wieder öffnen…
          </Button>
        </div>
      )}

      {isPreAccountMonth && (
        <p className="mt-3 text-xs text-gray-500">
          Dieser Monat liegt vor dem Beginn des Stundenkontos
          {accountStartDate
            ? ` (${formatIsoDateGerman(accountStartDate)})`
            : ""}{" "}
          und wird eigenständig gerechnet. Der Saldo fließt nicht in das
          Stundenkonto ein.
        </p>
      )}

      {!isPreAccountMonth && accountStartsInFuture && (
        <p className="mt-3 text-xs text-gray-500">
          Das Stundenkonto beginnt
          {accountStartDate
            ? ` am ${formatIsoDateGerman(accountStartDate)}`
            : " später in diesem Monat"}
          . Bis dahin wird noch kein Stundenkonto-Stand geführt.
        </p>
      )}
    </div>
  );
}
