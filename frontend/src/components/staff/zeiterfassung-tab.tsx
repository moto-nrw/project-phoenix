"use client";

import { useMemo, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { staffShiftService } from "~/lib/shift-api";
import type { StaffShift } from "~/lib/shift-helpers";
import {
  staffAbsenceService,
  staffHistoryService,
  staffMonthCloseService,
  staffMonthSummaryService,
  staffScheduleService,
} from "~/lib/staff-api";
import { MonthCloseReasonModal } from "~/components/staff/month-close-modal";
import { useSWRConfig } from "swr";
import type { StaffAbsenceRow, StaffHistorySession } from "~/lib/staff-api";
import { Monatskarte } from "~/components/time-tracking/monatskarte";
import type { MonthSummary } from "~/lib/time-tracking-helpers";
import {
  endOfMonth,
  endOfWeek,
  startOfMonth,
  startOfWeek,
  startOfYear,
  toDateKey,
} from "~/lib/staff-metrics-helpers";
import {
  getWeekNumber,
  OPEN_MONTH_REFRESH_MS,
} from "~/lib/time-tracking-helpers";
import { berlinTodayISO, parseISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { usePeriodMetrics } from "~/lib/hooks/use-period-metrics";
import { timeTrackingService } from "~/lib/time-tracking-api";
import { useSWRAuth } from "~/lib/swr";

import { StaffExportButton } from "./staff-export-button";
import { StaffSessionTable } from "./staff-session-table";
import { KpiCards, ViewToggle, type ViewMode } from "./staff-time-views";

// Reopening changes both the staff-detail chain and, when the selected person
// is the signed-in manager, the self-service month chain. The remaining
// school-wide caches expose the lock state and frozen carry-over.
export function isStaleAfterMonthReopen(
  key: unknown,
  staffId: string,
): boolean {
  return (
    typeof key === "string" &&
    (key.includes(`staff-month-summary-${staffId}-`) ||
      key.includes("time-tracking-month-summary-") ||
      key.includes("staff-time-accounts") ||
      key.includes("staff-month-close"))
  );
}

// Zeiterfassung tab. Day-row table comparing Soll vs Ist for each day in
// the visible window (week or month). A row click expands the read-only
// audit history; the pencil action opens admin-session-edit-modal, where
// corrections and backfills go through the time_tracking:manage endpoints
// with a mandatory audit reason.
export function ZeiterfassungTab({ staffId }: { readonly staffId: string }) {
  // The Berlin day, not the browser's, and re-rendered on the rollover: this
  // tab stays mounted for hours, and `new Date()` frozen at mount would keep
  // pointing "Dieser Monat" and the open-month poll at yesterday's month after
  // midnight — and at the wrong month entirely from a non-Berlin browser
  // (#1842). The backend derives its month from timezone.TodayDate().
  const todayISO = useBerlinToday();
  const today = useMemo(() => parseISODate(todayISO), [todayISO]);

  const [viewMode, setViewMode] = useState<ViewMode>("week");
  const [monthAnchor, setMonthAnchor] = useState(() =>
    startOfMonth(parseISODate(berlinTodayISO())),
  );
  const [weekAnchor, setWeekAnchor] = useState(() =>
    startOfWeek(parseISODate(berlinTodayISO())),
  );

  const { data: schedule, isLoading: scheduleLoading } = useSWRAuth(
    `staff-schedule-${staffId}`,
    () => staffScheduleService.getSchedule(staffId),
  );

  const {
    data: timeTrackingConfig,
    isLoading: timeTrackingConfigLoading,
    error: timeTrackingConfigError,
  } = useSWRAuth("time-tracking-config", () => timeTrackingService.getConfig());

  // Week, month and Stundenkonto all come from the server-computed model: only
  // the backend resolves each day against the schedule that was valid on that
  // day, so these cards can never contradict the Monatskarte and the daily
  // rows below them after a contract change (#1842).
  const metrics = usePeriodMetrics(staffId);

  const visibleFrom = useMemo(
    () =>
      viewMode === "month"
        ? startOfMonth(monthAnchor)
        : startOfWeek(weekAnchor),
    [viewMode, monthAnchor, weekAnchor],
  );
  const visibleTo = useMemo(
    () =>
      viewMode === "month" ? endOfMonth(monthAnchor) : endOfWeek(weekAnchor),
    [viewMode, monthAnchor, weekAnchor],
  );

  const visibleFromKey = toDateKey(visibleFrom);
  const visibleToKey = toDateKey(visibleTo);
  // Keyed by range only (no "visible" qualifier): usePeriodMetrics asks for the
  // current week under the same scheme, so while the table shows that week SWR
  // dedupes both into one request instead of fetching it twice.
  const { data: visibleSessions, isLoading: visibleLoading } = useSWRAuth<
    readonly StaffHistorySession[]
  >(`staff-history-${staffId}-${visibleFromKey}-${visibleToKey}`, () =>
    staffHistoryService.getHistory(staffId, visibleFromKey, visibleToKey),
  );
  // Absences are loaded in parallel with sessions so the table can show Krank/
  // Urlaub badges next to "Vor Ort"/"Homeoffice" (matches the MA-Sicht).
  const { data: visibleAbsences } = useSWRAuth<readonly StaffAbsenceRow[]>(
    `staff-absences-${staffId}-${visibleFromKey}-${visibleToKey}`,
    () =>
      staffAbsenceService.getAbsences(staffId, visibleFromKey, visibleToKey),
  );
  // Planned Dienstplan shifts for the visible range feed the table's Plan
  // column (#1844) — plan next to Ist, with the deviation reason in the audit
  // expand. Does not touch the Soll/Saldo math (Arbeitszeitmodell, #1842).
  // Loading and error state are surfaced explicitly: rendering an unresolved
  // or failed request as [] would show "–" in every Plan cell, presenting a
  // fetch failure to admins as proof that no shifts were planned.
  const {
    data: visibleShifts,
    isLoading: shiftsLoading,
    error: shiftsError,
  } = useSWRAuth<StaffShift[]>(
    `staff-shifts-visible-${staffId}-${visibleFromKey}-${visibleToKey}`,
    () =>
      staffShiftService.getShiftsForStaff(
        staffId,
        visibleFromKey,
        visibleToKey,
      ),
  );

  // Date-valid Soll for the visible range (#1842) — the same source the
  // Monatskarte is computed from. Without it the table applies the CURRENT
  // schedule to historical dates, so card and rows disagree the moment a
  // staff member's contracted hours change.
  // `isLoading` is the staleness signal, not a spinner: with keepPreviousData
  // SWR serves the PREVIOUS range's map while the new one is in flight, and the
  // table must not fall back to today's plan for the days it doesn't cover.
  const {
    data: dailyTargets,
    error: dailyTargetsError,
    isLoading: dailyTargetsLoading,
  } = useSWRAuth<ReadonlyMap<string, number>>(
    `staff-schedule-targets-${staffId}-${visibleFromKey}-${visibleToKey}`,
    () =>
      staffMonthSummaryService.getScheduleTargets(
        staffId,
        visibleFromKey,
        visibleToKey,
      ),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // Gesetzliche Feiertage im sichtbaren Zeitraum (#1418 3a). Tenant-global
  // (Bundesland-Setting), daher derselbe Endpunkt wie die MA-Sicht; die
  // Route akzeptiert time_tracking:own ODER :manage.
  const { data: tableHolidays } = useSWRAuth(
    `time-tracking-holidays-${visibleFromKey}-${visibleToKey}`,
    () => timeTrackingService.getHolidays(visibleFromKey, visibleToKey),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // OGS-Schließtage im sichtbaren Zeitraum (#1418 3b), analog zu den
  // Feiertagen tenant-global über denselben Endpunkt-Pfad.
  const { data: tableClosingDays } = useSWRAuth(
    `time-tracking-closing-days-${visibleFromKey}-${visibleToKey}`,
    () => timeTrackingService.getClosingDays(visibleFromKey, visibleToKey),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // Monatskarte (#1842): server-computed month aggregate, only fetched in
  // month mode. Everything is live — the Übertrag recomputes automatically
  // when past months are corrected. Edits and backfills invalidate this key
  // (staff-session-table handleSaved); the poll on top covers the current
  // month, whose Ist grows while a session is still running.
  const monthYear = monthAnchor.getFullYear();
  const monthNumber = monthAnchor.getMonth() + 1;
  const isCurrentMonth =
    monthYear === today.getFullYear() && monthNumber === today.getMonth() + 1;
  // A month wholly before the configured account start is summarized standalone
  // by the backend (full month, zero carry) and its closing value never enters
  // the account chain — so the Monatskarte must not sell it as a carry or as
  // the Stundenkonto (#1842).
  const accountStartDate = timeTrackingConfig?.accountStartDate ?? "";
  const isPreAccountMonth =
    accountStartDate !== "" &&
    `${monthYear}-${String(monthNumber).padStart(2, "0")}` <
      accountStartDate.slice(0, 7);
  // A start later THIS month (same month, future day) is not "pre-account" by
  // the month comparison above, but the account still hasn't begun — the card
  // must not print a "Stundenkonto Stand" for it (#1842). ISO dates compare
  // lexicographically; both are "YYYY-MM-DD".
  const accountStartsInFuture =
    accountStartDate !== "" && accountStartDate > todayISO;
  const [showReopenModal, setShowReopenModal] = useState(false);
  const { mutate: globalMutate } = useSWRConfig();
  const {
    data: monthSummary,
    isLoading: monthSummaryLoading,
    error: monthSummaryError,
  } = useSWRAuth<MonthSummary>(
    viewMode === "month"
      ? `staff-month-summary-${staffId}-${monthYear}-${monthNumber}`
      : null,
    () =>
      staffMonthSummaryService.getMonthSummary(staffId, monthYear, monthNumber),
    { refreshInterval: isCurrentMonth ? OPEN_MONTH_REFRESH_MS : 0 },
  );

  if (scheduleLoading) {
    return <Loading fullPage={false} />;
  }

  const handlePrev = () => {
    if (viewMode === "month") {
      setMonthAnchor(
        (prev) => new Date(prev.getFullYear(), prev.getMonth() - 1, 1),
      );
    } else {
      setWeekAnchor((prev) => {
        const next = new Date(prev);
        next.setDate(next.getDate() - 7);
        return next;
      });
    }
  };
  const handleNext = () => {
    if (viewMode === "month") {
      setMonthAnchor(
        (prev) => new Date(prev.getFullYear(), prev.getMonth() + 1, 1),
      );
    } else {
      setWeekAnchor((prev) => {
        const next = new Date(prev);
        next.setDate(next.getDate() + 7);
        return next;
      });
    }
  };
  const handleGoToday = () => {
    if (viewMode === "month") {
      setMonthAnchor(startOfMonth(today));
    } else {
      setWeekAnchor(startOfWeek(today));
    }
  };

  return (
    <div className="space-y-5">
      <KpiCards metrics={metrics} />

      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
        <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
          <h3 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
            Zeiterfassung
          </h3>
          <div className="flex flex-wrap items-center gap-2">
            <ViewToggle value={viewMode} onChange={setViewMode} />
            <StaffExportButton
              staffId={staffId}
              yearStart={startOfYear(today)}
            />
          </div>
        </div>

        <RangeNav
          label={formatRangeLabel(viewMode, monthAnchor, weekAnchor)}
          onPrev={handlePrev}
          onNext={handleNext}
          onToday={handleGoToday}
          todayLabel={viewMode === "month" ? "Diesen Monat" : "Diese Woche"}
        />

        {viewMode === "month" && (
          <div className="mt-4">
            <Monatskarte
              summary={monthSummary ?? null}
              isLoading={monthSummaryLoading}
              error={
                monthSummaryError
                  ? "Die Monatskarte konnte nicht geladen werden."
                  : null
              }
              isCurrentMonth={isCurrentMonth}
              isPreAccountMonth={isPreAccountMonth}
              accountStartsInFuture={accountStartsInFuture}
              accountStartDate={accountStartDate}
              onReopen={() => setShowReopenModal(true)}
            />
          </div>
        )}

        {showReopenModal && (
          <MonthCloseReasonModal
            title={`Monat wieder öffnen — ${String(monthNumber).padStart(2, "0")}/${monthYear}`}
            description={
              <>
                <p>
                  Hebt den Abschluss dieses Monats{" "}
                  <strong>nur für diese Person</strong> auf. Der Übertrag wird
                  wieder live aus den erfassten Zeiten gerechnet; nachträgliche
                  Änderungen wirken dann wieder auf alle Folgemonate.
                </p>
                <p>
                  Nur nötig, wenn der Abschluss selbst falsch war. Für eine
                  einzelne Nachkorrektur ist eine Buchung im offenen Monat
                  (Übersicht-Tab, Stundenkonto) meist der bessere Weg; der
                  Abschluss bleibt dann bestehen.
                </p>
              </>
            }
            submitLabel="Monat wieder öffnen"
            successMessage="Monat wieder geöffnet."
            destructive
            onSubmit={async (reason) => {
              await staffMonthCloseService.reopenMonth(staffId, {
                year: monthYear,
                month: monthNumber,
                reason,
              });
              // Reopen verschiebt den Kettenstart: alle Monatskarten und
              // Zeitkonten-Antworten dieses Tenants sind potenziell veraltet.
              await globalMutate((key) =>
                isStaleAfterMonthReopen(key, staffId),
              );
            }}
            onClose={() => setShowReopenModal(false)}
          />
        )}

        {visibleLoading || shiftsLoading ? (
          <div className="py-10">
            <Loading fullPage={false} />
          </div>
        ) : (
          <div className="mt-4">
            {shiftsError ? (
              <div className="mb-4">
                <Alert
                  type="error"
                  message="Der Dienstplan konnte nicht geladen werden. Die Plan-Spalte ist deshalb unvollständig; bitte die Seite neu laden."
                />
              </div>
            ) : null}
            <StaffSessionTable
              staffId={staffId}
              from={visibleFrom}
              to={visibleTo}
              sessions={visibleSessions ?? []}
              absences={visibleAbsences ?? []}
              schedule={schedule ?? null}
              dailyTargets={dailyTargets}
              dailyTargetsError={dailyTargetsError != null}
              dailyTargetsPending={dailyTargetsLoading}
              holidays={tableHolidays}
              closingDays={tableClosingDays}
              accountStartDate={timeTrackingConfig?.accountStartDate ?? null}
              accountStartDatePending={timeTrackingConfigLoading}
              accountStartDateError={
                timeTrackingConfig === undefined &&
                timeTrackingConfigError != null
              }
              today={today}
              isAdminView
              plannedShifts={visibleShifts ?? []}
            />
          </div>
        )}
      </div>
    </div>
  );
}

function RangeNav({
  label,
  onPrev,
  onNext,
  onToday,
  todayLabel,
}: {
  readonly label: string;
  readonly onPrev: () => void;
  readonly onNext: () => void;
  readonly onToday: () => void;
  readonly todayLabel: string;
}) {
  return (
    <div className="flex flex-col gap-3 sm:grid sm:grid-cols-3 sm:items-center">
      <div className="hidden sm:block" />
      <div className="flex min-w-0 items-center justify-center gap-2">
        <button
          type="button"
          onClick={onPrev}
          aria-label="Zurück"
          className="rounded-full p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900"
        >
          <svg
            className="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M15 19l-7-7 7-7"
            />
          </svg>
        </button>
        <h3 className="min-w-0 flex-1 text-center text-sm font-semibold text-gray-800 sm:min-w-[14rem]">
          {label}
        </h3>
        <button
          type="button"
          onClick={onNext}
          aria-label="Vor"
          className="rounded-full p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900"
        >
          <svg
            className="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M9 5l7 7-7 7"
            />
          </svg>
        </button>
      </div>
      <div className="flex justify-center sm:justify-end">
        <button
          type="button"
          onClick={onToday}
          className="rounded-full border border-gray-200 bg-white px-3 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50"
        >
          {todayLabel}
        </button>
      </div>
    </div>
  );
}

function formatRangeLabel(
  mode: ViewMode,
  monthAnchor: Date,
  weekAnchor: Date,
): string {
  if (mode === "month") {
    return monthAnchor.toLocaleDateString("de-DE", {
      timeZone: "Europe/Berlin",
      month: "long",
      year: "numeric",
    });
  }
  const monday = startOfWeek(weekAnchor);
  const sunday = endOfWeek(weekAnchor);
  const startDay = monday.getDate();
  const endDay = sunday.getDate();
  const startMonth = monday.toLocaleString("de-DE", {
    timeZone: "Europe/Berlin",
    month: "short",
  });
  const endMonth = sunday.toLocaleString("de-DE", {
    timeZone: "Europe/Berlin",
    month: "short",
  });
  // ISO-Wochennummer voranstellen, damit auf den ersten Blick klar ist,
  // welche KW gezeigt wird. Ohne diese Information bleibt nur das Datum,
  // was bei Schichtplänen / Wochen-Soll eher umständlich zu interpretieren ist.
  const weekNumber = getWeekNumber(monday);
  const dateRange =
    monday.getMonth() === sunday.getMonth()
      ? `${startDay}. – ${endDay}. ${endMonth} ${sunday.getFullYear()}`
      : `${startDay}. ${startMonth} – ${endDay}. ${endMonth} ${sunday.getFullYear()}`;
  return `KW ${weekNumber} · ${dateRange}`;
}
