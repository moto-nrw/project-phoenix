"use client";

import { AlertTriangle, CalendarDays } from "lucide-react";

import {
  getGermanWeekdayShort,
  getMonthDays,
  groupInstancesByDate,
  isInstanceUnderstaffed,
  toISODate,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";
import {
  timetableSurface,
  timetableToneColors,
  timetableWarningText,
} from "./timetable-style";

interface YearPlannerGridProps {
  months: Date[];
  instances: EnrichedInstance[];
  todayISO?: string;
  onMonthClick: (month: Date) => void;
  onDayClick: (dateISO: string) => void;
}

function appointmentCountLabel(count: number): string {
  return `${count} ${count === 1 ? "Termin" : "Termine"}`;
}

function understaffedCountLabel(count: number): string {
  return count === 1
    ? "1 unterbesetzter Termin"
    : `${count} unterbesetzte Termine`;
}

function conflictCountLabel(count: number): string {
  return `${count} ${count === 1 ? "Konflikt" : "Konflikte"}`;
}

function dayStatusColor(
  hasUnderstaffed: boolean,
  hasConflicts: boolean,
): string {
  if (hasUnderstaffed) return timetableToneColors.danger;
  if (hasConflicts) return timetableToneColors.warning;
  return timetableToneColors.success;
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
            className={`${timetableSurface} overflow-hidden`}
          >
            <button
              type="button"
              onClick={() => onMonthClick(month)}
              className="flex w-full items-center justify-between gap-3 border-b border-gray-200 px-3 py-2 text-left transition-colors hover:bg-gray-50"
            >
              <div>
                <h2 className="text-sm font-semibold text-gray-900">
                  {month.toLocaleDateString("de-DE", { month: "long" })}
                </h2>
                <p className="text-[11px] text-gray-500">
                  {monthInstances.length} Termin
                  {monthInstances.length === 1 ? "" : "e"}
                </p>
              </div>
              {conflictCount > 0 && (
                <span
                  className={`inline-flex items-center gap-1 rounded-full border border-[#EAB308]/20 bg-[#EAB308]/10 px-2 py-1 text-[10px] font-semibold ${timetableWarningText}`}
                >
                  <AlertTriangle className="h-3 w-3" aria-hidden />
                  {conflictCount}
                </span>
              )}
            </button>

            <div className="grid grid-cols-7 border-b border-gray-100 px-2 pt-2">
              {monthDays.slice(0, 7).map((day) => (
                <div
                  key={getGermanWeekdayShort(day)}
                  className="pb-1 text-center text-[10px] font-medium text-gray-400"
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
                const dayConflictCount = dayInstances.reduce(
                  (sum, instance) => sum + instance.conflictWarnings.length,
                  0,
                );
                const understaffedCount = dayInstances.filter(
                  isInstanceUnderstaffed,
                ).length;
                const hasConflicts = dayConflictCount > 0;
                // Priority chain danger > warning > success: a day with an
                // understaffed block outranks a mere conflict warning for
                // the single dot this cell has room for (issue #1838).
                const hasUnderstaffed = understaffedCount > 0;
                const dayDotColor = dayStatusColor(
                  hasUnderstaffed,
                  hasConflicts,
                );

                return (
                  <button
                    key={iso}
                    type="button"
                    onClick={() => onDayClick(iso)}
                    className={`relative flex aspect-square min-h-8 items-center justify-center rounded-md text-[11px] font-medium tabular-nums transition-colors hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                      outsideMonth ? "text-gray-300" : "text-gray-700"
                    } ${dayInstances.length > 0 ? "border border-gray-200 bg-white shadow-sm" : ""}`}
                    aria-label={[
                      `${iso}: ${appointmentCountLabel(dayInstances.length)}`,
                      understaffedCount > 0
                        ? understaffedCountLabel(understaffedCount)
                        : null,
                      dayConflictCount > 0
                        ? conflictCountLabel(dayConflictCount)
                        : null,
                    ]
                      .filter(Boolean)
                      .join(", ")}
                  >
                    <span
                      className={
                        isToday
                          ? "flex h-5 w-5 items-center justify-center rounded-full bg-gray-900 text-white"
                          : ""
                      }
                    >
                      {day.getDate()}
                    </span>
                    {dayInstances.length > 0 && (
                      <span
                        className="absolute bottom-1 h-1.5 w-1.5 rounded-full"
                        style={{ backgroundColor: dayDotColor }}
                        aria-hidden
                      />
                    )}
                  </button>
                );
              })}
            </div>

            {monthInstances.length === 0 && (
              <div className="flex items-center gap-1 border-t border-gray-100 px-3 py-2 text-[11px] text-gray-400">
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
