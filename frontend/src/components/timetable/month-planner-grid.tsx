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
  onPlanWeek: (dateISO: string) => void;
}

export function MonthPlannerGrid({
  days,
  monthDate,
  instances,
  todayISO,
  onDayClick,
  onPlanWeek,
}: MonthPlannerGridProps) {
  const grouped = groupInstancesByDate(instances);
  const currentMonth = monthDate.getMonth();
  const weekStarts = days.filter((_, index) => index % 7 === 0);

  return (
    <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
      <div className="grid grid-cols-7 border-b border-slate-200 bg-slate-50">
        {days.slice(0, 7).map((day) => (
          <div
            key={getGermanWeekdayShort(day)}
            className="px-3 py-2 text-center text-[11px] font-bold text-slate-500 uppercase"
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

          return (
            <button
              key={iso}
              type="button"
              onClick={() => onDayClick(iso)}
              className={`min-h-[112px] border-r border-b border-slate-100 p-2 text-left transition-colors hover:bg-[#EFF6FF] ${
                outsideMonth ? "bg-slate-50/70 text-slate-400" : "bg-white"
              } ${isToday ? "ring-2 ring-[#5080D8] ring-inset" : ""}`}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-bold">{day.getDate()}</span>
                {conflicts > 0 && (
                  <AlertTriangle className="h-3.5 w-3.5 text-[#A16207]" />
                )}
              </div>

              {dayInstances.length === 0 ? (
                <div className="mt-5 flex items-center gap-1 text-[11px] text-slate-400">
                  <CalendarDays className="h-3 w-3" />
                  Leer
                </div>
              ) : (
                <div className="mt-2 space-y-1">
                  <div className="text-xs font-semibold text-slate-800">
                    {dayInstances.length} Termin
                    {dayInstances.length === 1 ? "" : "e"}
                  </div>
                  <div className="flex flex-wrap gap-1">
                    {active > 0 && (
                      <span className="rounded-full bg-[#83CD2D] px-1.5 py-0.5 text-[9px] font-bold text-white">
                        {active} läuft
                      </span>
                    )}
                    {cancelled > 0 && (
                      <span className="rounded-full bg-[#FF3130] px-1.5 py-0.5 text-[9px] font-bold text-white">
                        {cancelled} abgesagt
                      </span>
                    )}
                  </div>
                  <div className="line-clamp-2 text-[11px] text-slate-500">
                    {dayInstances
                      .slice(0, 3)
                      .map((inst) => inst.title)
                      .join(", ")}
                  </div>
                </div>
              )}
            </button>
          );
        })}
      </div>

      <div className="flex flex-wrap gap-2 border-t border-slate-100 bg-slate-50 p-3">
        {weekStarts.map((weekStart) => {
          const iso = toISODate(weekStart);
          const weekHasInstances = days
            .slice(days.indexOf(weekStart), days.indexOf(weekStart) + 7)
            .some((day) => (grouped.get(toISODate(day)) ?? []).length > 0);
          if (weekHasInstances) return null;
          return (
            <button
              key={iso}
              type="button"
              onClick={() => onPlanWeek(iso)}
              className="rounded-md border border-dashed border-[#5080D8] bg-white px-3 py-1.5 text-xs font-semibold text-[#5080D8] hover:bg-[#EFF6FF]"
            >
              Woche ab {weekStart.toLocaleDateString("de-DE")} öffnen
            </button>
          );
        })}
      </div>
    </div>
  );
}
