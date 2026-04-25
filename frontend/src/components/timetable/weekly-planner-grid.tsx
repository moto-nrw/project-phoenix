"use client";

/**
 * WeeklyPlannerGrid — Mo–Fr columns × hourly time rows. Renders the full
 * week as a calendar-style grid with InstanceCards stacked vertically per
 * day in chronological order.
 *
 * Layout decision: instead of pixel-positioning each card by its start
 * time (which mockup V1 did), we stack cards in order. Reason: real-world
 * data has parallel activities and overflowing durations that pixel maths
 * doesn't handle gracefully. The chronological stack stays readable even
 * with 8+ instances per day; the time string on each card carries the
 * "when" information already.
 *
 * The grid header always shows Mo–Fr columns; days with no activities
 * show an empty placeholder so the visual scaffold stays consistent.
 */

import {
  formatDayHeader,
  getGermanWeekdayLong,
  groupInstancesByDate,
  toISODate,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";

import { InstanceCard } from "./instance-card";

interface WeeklyPlannerGridProps {
  weekDays: Date[]; // five Date objects Mo–Fr
  instances: EnrichedInstance[];
  selectedId?: string | null;
  onInstanceClick: (instance: EnrichedInstance) => void;
  todayISO?: string; // YYYY-MM-DD — column gets a subtle "heute" highlight
}

export function WeeklyPlannerGrid({
  weekDays,
  instances,
  selectedId,
  onInstanceClick,
  todayISO,
}: WeeklyPlannerGridProps) {
  const grouped = groupInstancesByDate(instances);

  return (
    <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
      {/* Day header row */}
      <div className="grid grid-cols-5 border-b border-slate-200 bg-slate-50">
        {weekDays.map((day) => {
          const iso = toISODate(day);
          const isToday = iso === todayISO;
          return (
            <div
              key={iso}
              className={`flex flex-col items-center justify-center px-3 py-3 ${
                isToday ? "bg-[#EFF6FF]" : ""
              }`}
            >
              <span
                className={`text-[11px] font-semibold ${
                  isToday ? "text-[#1E3A8A]" : "text-slate-500"
                }`}
              >
                {getGermanWeekdayLong(day)}
              </span>
              <span
                className={`text-sm font-bold ${
                  isToday ? "text-[#1E3A8A]" : "text-slate-900"
                }`}
              >
                {formatDayHeader(day).replace(/^.. /, "")}
              </span>
            </div>
          );
        })}
      </div>

      {/* Day body columns — vertical stack of cards per day */}
      <div className="grid min-h-[420px] grid-cols-5 divide-x divide-slate-100">
        {weekDays.map((day) => {
          const iso = toISODate(day);
          const dayInstances = grouped.get(iso) ?? [];
          const isToday = iso === todayISO;
          return (
            <div
              key={iso}
              className={`flex flex-col gap-2 p-2 ${isToday ? "bg-[#F8FAFD]" : ""}`}
            >
              {dayInstances.length === 0 ? (
                <div className="flex flex-1 items-center justify-center">
                  <span className="text-center text-[11px] text-slate-400 italic">
                    Keine Aktivitäten
                  </span>
                </div>
              ) : (
                dayInstances.map((inst) => (
                  <InstanceCard
                    key={inst.id}
                    instance={inst}
                    onClick={() => onInstanceClick(inst)}
                    isSelected={selectedId === inst.id}
                  />
                ))
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
