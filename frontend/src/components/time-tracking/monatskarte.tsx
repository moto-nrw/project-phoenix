"use client";

// Monatskarte (#1842): the consolidated month summary for the Zeiterfassung
// month view. Shows Übertrag, Soll, Ist, absence credits (Krank/Urlaub
// separately), the Dienstplan coverage (display-only), and the resulting
// Saldo. Everything is computed live by the backend — the Übertrag updates
// automatically when past months are corrected.

import type { ReactNode } from "react";
import { Lock } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusBadge } from "~/components/ui/status-badge";
import { MISSING_TARGET_OWNER } from "~/components/staff/missing-target-notice";
import { formatSignedDuration } from "~/components/staff/staff-time-views";
import { formatLocalizedDate } from "~/lib/localized-date-format";
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

// Plain text on the white card: brand green for a positive balance, the app's
// red for a negative one — same tone map as the table rows and KpiCard, which
// is why this uses the -strong step: green-hover (#74B825) sits at 2.43:1 on
// white and misses AA, green-strong (#3F6F12) reaches 6.01:1.
function deltaClass(minutes: number): string {
  if (minutes > 0) return "text-moto-green-strong";
  if (minutes < 0) return "text-moto-red-strong";
  return "text-gray-700";
}

function SummaryRow({
  label,
  value,
  hint,
  badge,
  valueClass,
  emphasis,
}: {
  readonly label: string;
  readonly value: string;
  readonly hint?: string;
  /** Status next to the label, e.g. the missing Soll (#6889). */
  readonly badge?: ReactNode;
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
        {badge && (
          <span className="ml-2 inline-flex align-middle">{badge}</span>
        )}
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

interface AbsenceRow {
  readonly label: string;
  readonly minutes: number;
  /** Credited days; Sonstiges has no credited day counter. */
  readonly days?: number;
  /** Days of a month without Soll, credited with 0 hours (#6889). */
  readonly daysWithoutTarget: number;
}

function absenceRows(summary: MonthSummary): AbsenceRow[] {
  return [
    {
      label: "Gutschrift Krankheit",
      minutes: summary.creditedSickMinutes,
      days: summary.sickDays,
      daysWithoutTarget: summary.sickDaysWithoutTarget,
    },
    {
      label: "Gutschrift Urlaub",
      minutes: summary.creditedVacationMinutes,
      days: summary.vacationDays,
      daysWithoutTarget: summary.vacationDaysWithoutTarget,
    },
    {
      label: "Gutschrift Fortbildung",
      minutes: summary.creditedTrainingMinutes,
      days: summary.trainingDays,
      daysWithoutTarget: summary.trainingDaysWithoutTarget,
    },
    {
      label: "Gutschrift Sonstiges",
      minutes: summary.creditedOtherMinutes,
      daysWithoutTarget: summary.otherDaysWithoutTarget,
    },
  ].filter((row) => row.minutes > 0 || row.daysWithoutTarget > 0);
}

/** "1 Tag", or "2 Tage ohne Soll" for days that credit nothing. */
function absenceRowHint(row: AbsenceRow): string | undefined {
  const parts: string[] = [];
  if (row.minutes > 0 && row.days !== undefined)
    parts.push(formatDays(row.days));
  if (row.daysWithoutTarget > 0) {
    parts.push(`${formatDays(row.daysWithoutTarget)} ohne Soll`);
  }
  return parts.length > 0 ? parts.join(" · ") : undefined;
}

export interface MonatskarteProps {
  readonly summary: MonthSummary | null;
  readonly isLoading: boolean;
  /** The failed load from `useApiLoadError` (with retry and request ID) or
   *  a plain sentence. */
  readonly error?: FormErrorInput;
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
  /**
   * Opens the Arbeitszeitmodell of this person; only for those who may set
   * the Soll. Without it the missing-Soll note names who sets it (#6889).
   */
  readonly onSetTarget?: () => void;
  /** Same as `onSetTarget`, as a link (own time-tracking page). */
  readonly setTargetHref?: string;
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
  onSetTarget,
  setTargetHref,
}: MonatskarteProps) {
  if (isLoading) {
    // Mirrors the loaded card below: title row, then SummaryRow-shaped
    // label/value lines in the same divide-y raster.
    return (
      <SectionCard padding="none">
        <output
          aria-label="Monatskarte wird geladen"
          aria-live="polite"
          className="block p-4 sm:p-6"
        >
          <div className="mb-3 flex items-center">
            <Skeleton className="h-5 w-44" />
          </div>
          <div className="divide-y divide-gray-100" aria-hidden="true">
            {Array.from({ length: 4 }, (_, i) => (
              <div
                key={i}
                className="flex items-baseline justify-between gap-4 py-1.5"
              >
                <Skeleton className="h-4 w-32" />
                <Skeleton className="h-4 w-16" />
              </div>
            ))}
          </div>
          <span className="sr-only">Monatskarte wird geladen</span>
        </output>
      </SectionCard>
    );
  }
  if (error) {
    return (
      <SectionCard>
        <LoadErrorAlert error={error} />
      </SectionCard>
    );
  }
  if (!summary) return null;

  const absences = absenceRows(summary);
  const hasDaysWithoutTarget = absences.some(
    (row) => row.daysWithoutTarget > 0,
  );

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
    <SectionCard
      title={`Monatskarte ${monthLabel(summary.year, summary.month)}`}
      titleSize="sm"
      headingLevel={3}
      actions={
        <>
          {summary.isClosed && (
            <span className="inline-flex items-center gap-1.5">
              <Lock className="h-3 w-3 text-gray-500" aria-hidden />
              <StatusBadge
                tone="gray"
                label={`Abgeschlossen${
                  summary.closedAt
                    ? ` am ${formatLocalizedDate(summary.closedAt, "de")}`
                    : ""
                }`}
              />
            </span>
          )}
          {summary.isClosed && onReopen && (
            <Button type="button" size="md" variant="ghost" onClick={onReopen}>
              Monat wieder öffnen…
            </Button>
          )}
        </>
      }
      bodyClassName="mt-3"
    >
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
          badge={
            summary.hasTarget ? undefined : (
              <StatusBadge
                tone="warning"
                compact
                label="Kein Soll hinterlegt"
              />
            )
          }
          hint={
            isCurrentMonth && summary.hasTarget
              ? `davon bis heute ${formatDuration(summary.targetMinutesToDate)}`
              : undefined
          }
          value={formatDuration(summary.targetMinutes)}
        />
        <SummaryRow
          label="Summe Ist"
          value={formatDuration(summary.actualMinutes)}
        />
        {/* Gutschriften; ohne Soll zählt ein Abwesenheitstag 0 Stunden, die
            Zeile bleibt trotzdem stehen, damit keine Krankmeldung aus dem
            Monat verschwindet (#6889). */}
        {absences.map((row) => (
          <SummaryRow
            key={row.label}
            label={row.label}
            hint={absenceRowHint(row)}
            value={formatDuration(row.minutes)}
            valueClass={row.minutes > 0 ? undefined : "text-gray-500"}
          />
        ))}
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

      {hasDaysWithoutTarget && (
        <Alert
          type="warning"
          size="compact"
          announce="off"
          className="mt-3"
          message={`Ohne Soll zählen Abwesenheiten 0 Stunden. Gilt später ein Soll für diese Tage, rechnet moto die Stunden nach.${
            onSetTarget || setTargetHref ? "" : ` ${MISSING_TARGET_OWNER}`
          }`}
          action={
            setTargetHref ? (
              <ButtonLink
                href={setTargetHref}
                variant="outline"
                size="compact"
                tooltip="Öffnet das Arbeitszeitmodell dieser Person."
              >
                Soll festlegen
              </ButtonLink>
            ) : onSetTarget ? (
              <Button
                type="button"
                variant="outline"
                size="compact"
                onClick={onSetTarget}
                tooltip="Öffnet das Arbeitszeitmodell dieser Person."
              >
                Soll festlegen
              </Button>
            ) : undefined
          }
        />
      )}

      {summary.isClosed && summary.driftMinutes !== 0 && (
        <Alert
          type="warning"
          size="compact"
          announce="off"
          className="mt-3"
          title="Seit dem Abschluss wurden Zeiten in diesem Monat geändert."
          message={
            <p>
              Neu gerechnet wäre der Übertrag{" "}
              <span className="font-medium tabular-nums">
                {formatSignedDuration(summary.closingBalanceMinutes)}
              </span>{" "}
              ({formatSignedDuration(summary.driftMinutes)} gegenüber dem
              abgeschlossenen Stand). Der Übertrag in den Folgemonat bleibt beim
              eingefrorenen Wert.
            </p>
          }
          details={
            <p className="mt-1">
              Zwei Wege zur Korrektur: die Differenz im offenen Monat als
              Buchung im Stundenkonto erfassen (Übersicht-Tab, empfohlen), oder
              den Monat wieder öffnen, wenn der Abschluss selbst falsch war.
            </p>
          }
        />
      )}

      {isPreAccountMonth && (
        <div className="mt-3">
          <Alert
            type="info"
            message={`Dieser Monat liegt vor dem Beginn des Stundenkontos${
              accountStartDate
                ? ` (${formatIsoDateGerman(accountStartDate)})`
                : ""
            } und wird eigenständig gerechnet. Der Saldo fließt nicht in das Stundenkonto ein.`}
          />
        </div>
      )}

      {!isPreAccountMonth && accountStartsInFuture && (
        <div className="mt-3">
          <Alert
            type="info"
            message={`Das Stundenkonto beginnt${
              accountStartDate
                ? ` am ${formatIsoDateGerman(accountStartDate)}`
                : " später in diesem Monat"
            }. Bis dahin wird noch kein Stundenkonto-Stand geführt.`}
          />
        </div>
      )}
    </SectionCard>
  );
}
