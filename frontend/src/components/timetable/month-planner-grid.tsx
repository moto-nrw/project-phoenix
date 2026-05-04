"use client";

import { AlertTriangle, CalendarDays } from "lucide-react";

import {
  getGermanWeekdayShort,
  groupInstancesByDate,
  toISODate,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";

interface MonthPlannerGridProps {
  days: Date[];
  monthDate: Date;
  instances: EnrichedInstance[];
  todayISO?: string;
  onDayClick: (dateISO: string) => void;
}

export function MonthPlannerGrid({
  days,
  monthDate,
  instances,
  todayISO,
  onDayClick,
}: MonthPlannerGridProps) {
  const grouped = groupInstancesByDate(instances);
  const currentMonth = monthDate.getMonth();

  return (
    <div className="overflow-hidden rounded-lg border border-slate-200 bg-white">
      <div className="grid grid-cols-7 border-b border-slate-200">
        {days.slice(0, 7).map((day) => (
          <div
            key={getGermanWeekdayShort(day)}
            className="px-3 py-2 text-center text-[11px] font-medium tracking-wide text-slate-500 uppercase"
          >
            {getGermanWeekdayShort(day)}
          </div>
        ))}
      </div>

      <div className="grid grid-cols-7">
        {days.map((day) => {
          const iso = toISODate(day);
          const dayInstances = grouped.get(iso) ?? [];
          const isToday = iso === todayISO;
          const outsideMonth = day.getMonth() !== currentMonth;
          const cancelled = dayInstances.filter(
            (inst) => inst.status === "cancelled",
          ).length;
          const active = dayInstances.filter(
            (inst) => inst.status === "active",
          ).length;
          const conflicts = dayInstances.reduce(
            (sum, inst) => sum + inst.conflictWarnings.length,
            0,
          );

          const visibleTitles = dayInstances.slice(0, 3);
          const moreCount = dayInstances.length - visibleTitles.length;

          return (
            <button
              key={iso}
              type="button"
              onClick={() => onDayClick(iso)}
              className={`min-h-[112px] border-r border-b border-slate-100 p-2 text-left transition-colors hover:bg-slate-50 ${
                outsideMonth ? "bg-slate-50/40 text-slate-400" : "bg-white"
              }`}
            >
              <div className="flex items-center justify-between gap-2">
                {isToday ? (
                  <span className="inline-flex h-6 w-6 items-center justify-center rounded-full bg-slate-900 text-[12px] font-semibold text-white tabular-nums">
                    {day.getDate()}
                  </span>
                ) : (
                  <span className="text-[13px] font-semibold tabular-nums">
                    {day.getDate()}
                  </span>
                )}
                {conflicts > 0 && (
                  <AlertTriangle className="h-3.5 w-3.5 text-amber-600" />
                )}
              </div>

              {dayInstances.length === 0 ? (
                <div className="mt-5 flex items-center gap-1 text-[11px] text-slate-400">
                  <CalendarDays className="h-3 w-3" />
                  Leer
                </div>
              ) : (
                <div className="mt-2 space-y-1">
                  <div className="text-[12px] font-semibold text-slate-900">
                    {dayInstances.length} Termin
                    {dayInstances.length === 1 ? "" : "e"}
                  </div>
                  <div className="flex flex-wrap gap-1">
                    {active > 0 && (
                      <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-1.5 py-0.5 text-[9px] font-medium text-slate-700">
                        <span
                          className="h-1 w-1 rounded-full bg-[#83CD2D]"
                          aria-hidden
                        />
                        {active} läuft
                      </span>
                    )}
                    {cancelled > 0 && (
                      <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-1.5 py-0.5 text-[9px] font-medium text-slate-700">
                        <span
                          className="h-1 w-1 rounded-full bg-[#FF3130]"
                          aria-hidden
                        />
                        {cancelled} abgesagt
                      </span>
                    )}
                  </div>
                  <div className="line-clamp-2 text-[11px] text-slate-500">
                    {visibleTitles.map((inst) => inst.title).join(", ")}
                  </div>
                  {moreCount > 0 && (
                    <div className="text-[10px] font-medium text-slate-500">
                      + {moreCount} weitere
                    </div>
                  )}
                </div>
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}
