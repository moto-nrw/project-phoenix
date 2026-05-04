"use client";

/**
 * WeeklyCalendarGrid — Apple-Calendar-style week view.
 *
 * Layout: a sticky day-header row above a vertically scrollable body.
 * Inside the body: a fixed time-gutter on the left and 5 day columns
 * (Mo-Fr). Each day column is a positioned container; events are placed
 * absolutely via `getEventBlockPosition`. Overlapping events split a
 * column horizontally via `assignBlockLanes`.
 *
 * Replaces the previous vertical-stack `WeeklyPlannerGrid`. The stack
 * approach worked but failed the "where is the 11am AG?" test —
 * positioning by time is the whole point of a week view.
 */

import { useEffect, useState } from "react";

import {
  assignBlockLanes,
  formatDayHeader,
  getCurrentTimeOffset,
  getEventBlockPosition,
  getGermanWeekdayShort,
  groupInstancesByDate,
  toISODate,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";

import { InstanceBlock } from "./instance-block";

const TIME_GUTTER_WIDTH_PX = 64;
const DAY_HEADER_HEIGHT_PX = 56;

interface WeeklyCalendarGridProps {
  weekDays: Date[]; // Mo-Fr (5 dates)
  instances: EnrichedInstance[];
  selectedId: string | null;
  onInstanceClick: (instance: EnrichedInstance) => void;
  todayISO: string;
  /** Visible day window start (HH parsed to integer). Default 9. */
  dayStartHour: number;
  /** Visible day window end (HH parsed to integer). Default 17. */
  dayEndHour: number;
  /** Pixel height per hour row. Driven by zoom controls. */
  hourHeightPx: number;
}

export function WeeklyCalendarGrid({
  weekDays,
  instances,
  selectedId,
  onInstanceClick,
  todayISO,
  dayStartHour,
  dayEndHour,
  hourHeightPx,
}: WeeklyCalendarGridProps) {
  const grouped = groupInstancesByDate(instances);
  const hours = Array.from(
    { length: dayEndHour - dayStartHour + 1 },
    (_, i) => dayStartHour + i,
  );
  const gridHeightPx = (dayEndHour - dayStartHour) * hourHeightPx;

  // Re-render every minute so the now-line moves smoothly.
  const [nowTick, setNowTick] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNowTick(Date.now()), 60_000);
    return () => clearInterval(id);
  }, []);
  void nowTick;

  return (
    <div className="overflow-hidden rounded-lg border border-slate-200 bg-white">
      {/* Sticky day header */}
      <div
        className="grid border-b border-slate-200 bg-white"
        style={{
          gridTemplateColumns: `${TIME_GUTTER_WIDTH_PX}px repeat(${weekDays.length}, minmax(0, 1fr))`,
          height: `${DAY_HEADER_HEIGHT_PX}px`,
        }}
      >
        <div aria-hidden />
        {weekDays.map((day) => {
          const iso = toISODate(day);
          const isToday = iso === todayISO;
          return (
            <div
              key={iso}
              className="flex items-center justify-center gap-2 border-l border-slate-200 px-2 py-2"
            >
              <span className="text-[11px] font-medium tracking-wide text-slate-500 uppercase">
                {getGermanWeekdayShort(day)}
              </span>
              {isToday ? (
                <span
                  className="inline-flex h-6 w-6 items-center justify-center rounded-full bg-slate-900 text-[12px] font-semibold text-white tabular-nums"
                  aria-label={formatDayHeader(day)}
                >
                  {day.getDate()}
                </span>
              ) : (
                <span
                  className="text-[14px] font-semibold text-slate-900 tabular-nums"
                  aria-label={formatDayHeader(day)}
                >
                  {String(day.getDate()).padStart(2, "0")}
                </span>
              )}
            </div>
          );
        })}
      </div>

      {/* Scrollable body */}
      <div className="max-h-[720px] overflow-y-auto">
        <div
          className="grid"
          style={{
            gridTemplateColumns: `${TIME_GUTTER_WIDTH_PX}px repeat(${weekDays.length}, minmax(0, 1fr))`,
          }}
        >
          {/* Time gutter */}
          <div
            className="border-r border-slate-200 bg-slate-50"
            style={{ height: `${gridHeightPx}px` }}
          >
            {hours.slice(0, -1).map((hour) => (
              <div
                key={hour}
                className="flex items-start justify-end px-2 pt-1 text-[11px] font-medium text-slate-400"
                style={{ height: `${hourHeightPx}px` }}
              >
                {String(hour).padStart(2, "0")}:00
              </div>
            ))}
          </div>

          {/* Day columns */}
          {weekDays.map((day) => {
            const iso = toISODate(day);
            const isToday = iso === todayISO;
            const dayInstances = grouped.get(iso) ?? [];
            const laned = assignBlockLanes(dayInstances);
            const nowOffset = isToday
              ? getCurrentTimeOffset(hourHeightPx, dayStartHour, dayEndHour)
              : null;

            return (
              <div
                key={iso}
                className={`relative border-l border-slate-200 ${isToday ? "bg-slate-50/60" : ""}`}
                style={{ height: `${gridHeightPx}px` }}
              >
                {/* Hour grid lines */}
                {hours.slice(1, -1).map((hour) => {
                  const top = (hour - dayStartHour) * hourHeightPx;
                  return (
                    <div
                      key={hour}
                      className="pointer-events-none absolute right-0 left-0 border-t border-slate-100"
                      style={{ top: `${top}px` }}
                    />
                  );
                })}

                {/* Half-hour grid lines (lighter) */}
                {hours.slice(0, -1).map((hour) => {
                  const top =
                    (hour - dayStartHour) * hourHeightPx + hourHeightPx / 2;
                  return (
                    <div
                      key={`half-${hour}`}
                      className="pointer-events-none absolute right-0 left-0 border-t border-slate-50"
                      style={{ top: `${top}px` }}
                    />
                  );
                })}

                {/* Event blocks */}
                {laned.map(({ instance, lane, laneCount }) => {
                  const { top, height } = getEventBlockPosition(
                    instance.startTime,
                    instance.endTime,
                    hourHeightPx,
                    dayStartHour,
                  );
                  const widthPct = 100 / laneCount;
                  return (
                    <InstanceBlock
                      key={instance.id}
                      instance={instance}
                      top={top}
                      height={height}
                      left={`${lane * widthPct}%`}
                      width={`calc(${widthPct}% - 4px)`}
                      isSelected={selectedId === instance.id}
                      onClick={() => onInstanceClick(instance)}
                    />
                  );
                })}

                {/* Now line (today only) */}
                {nowOffset !== null && (
                  <>
                    <div
                      className="pointer-events-none absolute -left-1 z-10 h-2 w-2 rounded-full bg-[#FF3130]"
                      style={{ top: `${nowOffset - 4}px` }}
                    />
                    <div
                      className="pointer-events-none absolute right-0 left-0 z-10 border-t-2 border-[#FF3130]"
                      style={{ top: `${nowOffset}px` }}
                    />
                  </>
                )}

                {/* Empty-state hint */}
                {dayInstances.length === 0 && (
                  <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
                    <span className="text-[11px] text-slate-400 italic">
                      Keine Aktivitäten
                    </span>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
