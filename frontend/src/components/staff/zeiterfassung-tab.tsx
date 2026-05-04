"use client";

import { useMemo, useState } from "react";

import { Loading } from "~/components/ui/loading";
import { staffHistoryService, staffScheduleService } from "~/lib/staff-api";
import type { StaffHistorySession } from "~/lib/staff-api";
import {
  buildMonthGrid,
  endOfMonth,
  endOfWeek,
  groupSessionsByDay,
  startOfMonth,
  startOfWeek,
  toDateKey,
} from "~/lib/staff-metrics-helpers";
import { useSWRAuth } from "~/lib/swr";

import {
  MonthCalendar,
  ViewToggle,
  WeekView,
  type ViewMode,
} from "./staff-time-views";

// Zeiterfassung tab. Shows the actual sessions per day vs the schedule's
// Soll. The Dienstplan tab edits Soll separately. Default view is the
// month calendar; the toggle switches to a week-detail view. Future work
// (Tranche 1.5+ in #1375): replace this read-only calendar with a row-
// per-day table that allows admin corrections inline.
export function ZeiterfassungTab({ staffId }: { readonly staffId: string }) {
  const today = useMemo(() => new Date(), []);

  const [viewMode, setViewMode] = useState<ViewMode>("month");
  const [monthAnchor, setMonthAnchor] = useState(() =>
    startOfMonth(new Date()),
  );
  const [weekAnchor, setWeekAnchor] = useState(() => startOfWeek(new Date()));

  const { data: schedule, isLoading: scheduleLoading } = useSWRAuth(
    `staff-schedule-${staffId}`,
    () => staffScheduleService.getSchedule(staffId),
  );

  const visibleFrom = useMemo(() => {
    if (viewMode === "month") {
      const grid = buildMonthGrid(monthAnchor);
      const firstRow = grid[0];
      return firstRow?.[0]?.date ?? startOfMonth(monthAnchor);
    }
    return startOfWeek(weekAnchor);
  }, [viewMode, monthAnchor, weekAnchor]);

  const visibleTo = useMemo(() => {
    if (viewMode === "month") {
      const grid = buildMonthGrid(monthAnchor);
      const lastRow = grid[grid.length - 1];
      return lastRow?.[6]?.date ?? endOfMonth(monthAnchor);
    }
    return endOfWeek(weekAnchor);
  }, [viewMode, monthAnchor, weekAnchor]);

  const visibleFromKey = toDateKey(visibleFrom);
  const visibleToKey = toDateKey(visibleTo);
  const { data: visibleSessions, isLoading: visibleLoading } = useSWRAuth<
    StaffHistorySession[]
  >(`staff-history-visible-${staffId}-${visibleFromKey}-${visibleToKey}`, () =>
    staffHistoryService.getHistory(staffId, visibleFromKey, visibleToKey),
  );

  const scheduleEntries = useMemo(() => schedule?.entries ?? [], [schedule]);

  const targetByDow = useMemo(() => {
    const map = new Map<number, number>();
    for (const entry of scheduleEntries) {
      map.set(entry.dayOfWeek, entry.targetMinutes);
    }
    return map;
  }, [scheduleEntries]);

  const sessionMinutesByDay = useMemo(
    () => groupSessionsByDay(visibleSessions ?? []),
    [visibleSessions],
  );

  if (scheduleLoading) {
    return <Loading fullPage={false} />;
  }

  const handlePrevMonth = () => {
    setMonthAnchor(
      (prev) => new Date(prev.getFullYear(), prev.getMonth() - 1, 1),
    );
  };
  const handleNextMonth = () => {
    setMonthAnchor(
      (prev) => new Date(prev.getFullYear(), prev.getMonth() + 1, 1),
    );
  };
  const handleGoToday = () => {
    if (viewMode === "month") {
      setMonthAnchor(startOfMonth(new Date()));
    } else {
      setWeekAnchor(startOfWeek(new Date()));
    }
  };
  const handlePrevWeek = () => {
    setWeekAnchor((prev) => {
      const next = new Date(prev);
      next.setDate(next.getDate() - 7);
      return next;
    });
  };
  const handleNextWeek = () => {
    setWeekAnchor((prev) => {
      const next = new Date(prev);
      next.setDate(next.getDate() + 7);
      return next;
    });
  };

  return (
    <div className="space-y-5">
      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
        <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
          <h3 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
            Zeiterfassung
          </h3>
          <ViewToggle value={viewMode} onChange={setViewMode} />
        </div>

        {visibleLoading ? (
          <Loading fullPage={false} />
        ) : viewMode === "month" ? (
          <MonthCalendar
            monthAnchor={monthAnchor}
            sessionMinutesByDay={sessionMinutesByDay}
            targetByDow={targetByDow}
            onPrev={handlePrevMonth}
            onNext={handleNextMonth}
            onGoToday={handleGoToday}
            today={today}
          />
        ) : (
          <WeekView
            weekAnchor={weekAnchor}
            sessionMinutesByDay={sessionMinutesByDay}
            targetByDow={targetByDow}
            onPrev={handlePrevWeek}
            onNext={handleNextWeek}
            onGoToday={handleGoToday}
            today={today}
          />
        )}
      </div>
    </div>
  );
}
