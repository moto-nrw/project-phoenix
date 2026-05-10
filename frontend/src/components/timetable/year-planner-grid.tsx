"use client";

import { AlertTriangle, CalendarDays } from "lucide-react";

import {
  getGermanWeekdayShort,
  getMonthDays,
  groupInstancesByDate,
  toISODate,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";

interface YearPlannerGridProps {
  months: Date[];
  instances: EnrichedInstance[];
  todayISO?: string;
  onMonthClick: (month: Date) => void;
  onDayClick: (dateISO: string) => void;
}

export function YearPlannerGrid({
  months,
  instances,
  todayISO,
  onMonthClick,
  onDayClick,
}: YearPlannerGridProps) {
  const grouped = groupInstancesByDate(instances);

  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      {months.map((month) => {
        const monthDays = getMonthDays(month);
        const currentMonth = month.getMonth();
        const monthInstances = instances.filter((instance) => {
          const date = new Date(`${instance.date}T00:00:00`);
          return (
            date.getFullYear() === month.getFullYear() &&
            date.getMonth() === currentMonth
          );
        });
        const conflictCount = monthInstances.reduce(
          (sum, instance) => sum + instance.conflictWarnings.length,
          0,
        );

        return (
          <section
            key={`${month.getFullYear()}-${month.getMonth()}`}
            className="overflow-hidden rounded-lg border border-slate-200 bg-white"
          >
            <button
              type="button"
              onClick={() => onMonthClick(month)}
              className="flex w-full items-center justify-between gap-3 border-b border-slate-200 px-3 py-2 text-left transition-colors hover:bg-slate-50"
            >
              <div>
                <h2 className="text-sm font-semibold text-slate-900">
                  {month.toLocaleDateString("de-DE", { month: "long" })}
                </h2>
                <p className="text-[11px] text-slate-500">
                  {monthInstances.length} Termin
                  {monthInstances.length === 1 ? "" : "e"}
                </p>
              </div>
              {conflictCount > 0 && (
                <span className="inline-flex items-center gap-1 rounded-full bg-[#FCEFD9] px-2 py-1 text-[10px] font-semibold text-[#F78C10]">
                  <AlertTriangle className="h-3 w-3" aria-hidden />
                  {conflictCount}
                </span>
              )}
            </button>

            <div className="grid grid-cols-7 border-b border-slate-100 px-2 pt-2">
              {monthDays.slice(0, 7).map((day) => (
                <div
                  key={getGermanWeekdayShort(day)}
                  className="pb-1 text-center text-[10px] font-medium text-slate-400"
                >
                  {getGermanWeekdayShort(day)}
                </div>
              ))}
            </div>

            <div className="grid grid-cols-7 gap-1 p-2">
              {monthDays.map((day) => {
                const iso = toISODate(day);
                const dayInstances = grouped.get(iso) ?? [];
                const outsideMonth = day.getMonth() !== currentMonth;
                const isToday = iso === todayISO;
                const hasConflicts = dayInstances.some(
                  (instance) => instance.conflictWarnings.length > 0,
                );

                return (
                  <button
                    key={iso}
                    type="button"
                    onClick={() => onDayClick(iso)}
                    className={`relative flex aspect-square min-h-8 items-center justify-center rounded text-[11px] font-medium tabular-nums transition-colors hover:bg-slate-100 ${
                      outsideMonth ? "text-slate-300" : "text-slate-700"
                    } ${dayInstances.length > 0 ? "bg-slate-50" : ""}`}
                    aria-label={`${iso}: ${dayInstances.length} Termine`}
                  >
                    <span
                      className={
                        isToday
                          ? "flex h-5 w-5 items-center justify-center rounded-full bg-slate-900 text-white"
                          : ""
                      }
                    >
                      {day.getDate()}
                    </span>
                    {dayInstances.length > 0 && (
                      <span
                        className={`absolute bottom-1 h-1.5 w-1.5 rounded-full ${
                          hasConflicts ? "bg-[#F78C10]" : "bg-[#83CD2D]"
                        }`}
                        aria-hidden
                      />
                    )}
                  </button>
                );
              })}
            </div>

            {monthInstances.length === 0 && (
              <div className="flex items-center gap-1 border-t border-slate-100 px-3 py-2 text-[11px] text-slate-400">
                <CalendarDays className="h-3 w-3" aria-hidden />
                Leer
              </div>
            )}
          </section>
        );
      })}
    </div>
  );
}
