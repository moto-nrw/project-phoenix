"use client";

import { useMemo, useState } from "react";

import { Loading } from "~/components/ui/loading";
import { staffShiftService } from "~/lib/shift-api";
import type { StaffShift } from "~/lib/shift-helpers";
import {
  staffAbsenceService,
  staffHistoryService,
  staffScheduleService,
} from "~/lib/staff-api";
import type { StaffAbsenceRow, StaffHistorySession } from "~/lib/staff-api";
import {
  computeStaffMetrics,
  endOfMonth,
  endOfWeek,
  resolveAccountStartDate,
  startOfMonth,
  startOfWeek,
  startOfYear,
  toDateKey,
} from "~/lib/staff-metrics-helpers";
import { getWeekNumber } from "~/lib/time-tracking-helpers";
import { timeTrackingService } from "~/lib/time-tracking-api";
import { useSWRAuth } from "~/lib/swr";

import { StaffExportButton } from "./staff-export-button";
import { StaffSessionTable } from "./staff-session-table";
import { KpiCards, ViewToggle, type ViewMode } from "./staff-time-views";

// Zeiterfassung tab. Day-row table comparing Soll vs Ist for each day in
// the visible window (week or month). A row click expands the read-only
// audit history; the pencil action opens admin-session-edit-modal, where
// corrections and backfills go through the time_tracking:manage endpoints
// with a mandatory audit reason.
export function ZeiterfassungTab({ staffId }: { readonly staffId: string }) {
  const today = useMemo(() => new Date(), []);

  const [viewMode, setViewMode] = useState<ViewMode>("week");
  const [monthAnchor, setMonthAnchor] = useState(() =>
    startOfMonth(new Date()),
  );
  const [weekAnchor, setWeekAnchor] = useState(() => startOfWeek(new Date()));

  const { data: schedule, isLoading: scheduleLoading } = useSWRAuth(
    `staff-schedule-${staffId}`,
    () => staffScheduleService.getSchedule(staffId),
  );

  const { data: timeTrackingConfig } = useSWRAuth("time-tracking-config", () =>
    timeTrackingService.getConfig(),
  );

  const accountAnchor = useMemo(() => {
    return resolveAccountStartDate(today, timeTrackingConfig?.accountStartDate);
  }, [timeTrackingConfig?.accountStartDate, today]);
  const accountFromKey = toDateKey(accountAnchor);
  const accountToKey = toDateKey(today);
  const { data: accountSessions } = useSWRAuth<StaffHistorySession[]>(
    `staff-history-account-${staffId}-${accountFromKey}-${accountToKey}`,
    () => staffHistoryService.getHistory(staffId, accountFromKey, accountToKey),
  );
  const { data: accountAbsences } = useSWRAuth<StaffAbsenceRow[]>(
    `staff-absences-account-${staffId}-${accountFromKey}-${accountToKey}`,
    () =>
      staffAbsenceService.getAbsences(staffId, accountFromKey, accountToKey),
  );
  const metrics = useMemo(
    () =>
      schedule
        ? computeStaffMetrics(
            schedule,
            accountSessions ?? [],
            accountAbsences ?? [],
            today,
            timeTrackingConfig?.accountStartDate,
          )
        : null,
    [
      schedule,
      accountSessions,
      accountAbsences,
      today,
      timeTrackingConfig?.accountStartDate,
    ],
  );

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
  const { data: visibleSessions, isLoading: visibleLoading } = useSWRAuth<
    StaffHistorySession[]
  >(`staff-history-visible-${staffId}-${visibleFromKey}-${visibleToKey}`, () =>
    staffHistoryService.getHistory(staffId, visibleFromKey, visibleToKey),
  );
  // Absences are loaded in parallel with sessions so the table can show Krank/
  // Urlaub badges next to "Vor Ort"/"Homeoffice" (matches the MA-Sicht).
  const { data: visibleAbsences } = useSWRAuth<StaffAbsenceRow[]>(
    `staff-absences-visible-${staffId}-${visibleFromKey}-${visibleToKey}`,
    () =>
      staffAbsenceService.getAbsences(staffId, visibleFromKey, visibleToKey),
  );
  // Planned Dienstplan shifts for the visible range feed the table's Plan
  // column (#1844) — plan next to Ist, with the deviation reason in the audit
  // expand. Does not touch the Soll/Saldo math (Arbeitszeitmodell, #1842).
  const { data: visibleShifts } = useSWRAuth<StaffShift[]>(
    `staff-shifts-visible-${staffId}-${visibleFromKey}-${visibleToKey}`,
    () =>
      staffShiftService.getShiftsForStaff(
        staffId,
        visibleFromKey,
        visibleToKey,
      ),
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
      setMonthAnchor(startOfMonth(new Date()));
    } else {
      setWeekAnchor(startOfWeek(new Date()));
    }
  };

  return (
    <div className="space-y-5">
      {metrics && <KpiCards metrics={metrics} />}

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

        {visibleLoading ? (
          <div className="py-10">
            <Loading fullPage={false} />
          </div>
        ) : (
          <div className="mt-4">
            <StaffSessionTable
              staffId={staffId}
              from={visibleFrom}
              to={visibleTo}
              sessions={visibleSessions ?? []}
              absences={visibleAbsences ?? []}
              schedule={schedule ?? null}
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
      month: "long",
      year: "numeric",
    });
  }
  const monday = startOfWeek(weekAnchor);
  const sunday = endOfWeek(weekAnchor);
  const startDay = monday.getDate();
  const endDay = sunday.getDate();
  const startMonth = monday.toLocaleString("de-DE", { month: "short" });
  const endMonth = sunday.toLocaleString("de-DE", { month: "short" });
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
