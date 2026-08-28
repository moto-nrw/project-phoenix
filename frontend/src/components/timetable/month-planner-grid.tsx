"use client";

import { AlertTriangle } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

import { ClosingDayChip } from "~/components/planning/closing-day-marker";
import {
  comparePlanningInstances,
  getGermanWeekdayShort,
  groupInstancesByDate,
  toISODate,
} from "~/lib/timetable-helpers";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import type { EnrichedInstance } from "~/lib/timetable-types";
import { capacityTone, TimetableRatioPill } from "./timetable-ratio-pill";
import {
  TIMETABLE_UNTYPED_EDGE_COLOR,
  timetableSurface,
} from "./timetable-style";

interface MonthPlannerGridProps {
  days: Date[];
  monthDate: Date;
  instances: EnrichedInstance[];
  todayISO?: string;
  /**
   * OGS-Schließtage im sichtbaren Monat, keyed YYYY-MM-DD → Grund (#2032).
   * Betroffene Tageszellen werden neutral eingefärbt und mit dem Grund
   * beschriftet.
   */
  closingDays?: ReadonlyMap<string, string>;
  onDayClick: (dateISO: string) => void;
  onInstanceClick?: (instance: EnrichedInstance) => void;
}

export function MonthPlannerGrid({
  days,
  monthDate,
  instances,
  todayISO,
  closingDays,
  onDayClick,
  onInstanceClick,
}: MonthPlannerGridProps) {
  const grouped = groupInstancesByDate(instances);
  const currentMonth = monthDate.getMonth();

  return (
    <div className={`${timetableSurface} overflow-hidden`}>
      <div className="grid grid-cols-7 border-b border-gray-200">
        {days.slice(0, 7).map((day) => (
          <div
            key={getGermanWeekdayShort(day)}
            className="px-3 py-2 text-center text-[11px] font-medium tracking-wide text-gray-500 uppercase"
          >
            {getGermanWeekdayShort(day)}
          </div>
        ))}
      </div>

      <div className="grid grid-cols-7">
        {days.map((day) => {
          const iso = toISODate(day);
          const dayInstances = [...(grouped.get(iso) ?? [])].sort(
            comparePlanningInstances,
          );
          const isToday = iso === todayISO;
          const outsideMonth = day.getMonth() !== currentMonth;
          const closingReason = closingDays?.get(iso);
          const conflicts = dayInstances.reduce(
            (sum, inst) => sum + inst.conflictWarnings.length,
            0,
          );

          const visibleInstances = dayInstances.slice(0, 4);
          const moreCount = dayInstances.length - visibleInstances.length;
          let backgroundClass = "bg-white";
          if (closingReason !== undefined) {
            backgroundClass = "bg-gray-100/70";
          } else if (outsideMonth) {
            backgroundClass = "bg-gray-50/40";
          }
          const Cell = onInstanceClick ? "div" : "button";
          const InstanceCard = onInstanceClick ? "button" : "div";

          return (
            <Cell
              key={iso}
              {...(!onInstanceClick
                ? { type: "button" as const, onClick: () => onDayClick(iso) }
                : {})}
              className={`relative min-h-[112px] border-r border-b border-gray-100 p-2 text-left transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset ${backgroundClass} ${outsideMonth ? "text-gray-400" : ""}`}
            >
              {onInstanceClick && (
                <button
                  type="button"
                  onClick={() => onDayClick(iso)}
                  aria-label={`${getGermanWeekdayShort(day)} ${day.getDate()}.${day.getMonth() + 1}.${day.getFullYear()}`}
                  className="absolute inset-0 z-0 rounded focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset"
                />
              )}

              <div className="pointer-events-none relative z-10">
                <div className="flex items-center justify-between gap-2">
                  {isToday ? (
                    <span className="inline-flex h-6 w-6 items-center justify-center rounded-full bg-gray-900 text-xs font-semibold text-white tabular-nums">
                      {day.getDate()}
                    </span>
                  ) : (
                    <span className="text-sm font-semibold tabular-nums">
                      {day.getDate()}
                    </span>
                  )}
                  {conflicts > 0 && (
                    <AlertTriangle className="text-moto-amber h-3.5 w-3.5" />
                  )}
                </div>

                {closingReason !== undefined && (
                  <ClosingDayChip
                    reason={closingReason}
                    className="mt-1 w-full"
                  />
                )}

                {dayInstances.length === 0 ? (
                  closingReason === undefined && (
                    <div className="mt-5 flex items-center gap-1 text-[11px] text-gray-400">
                      <MotoConceptIcon concept="calendar" size={14} />
                      Leer
                    </div>
                  )
                ) : (
                  <div className="mt-2 space-y-1">
                    {visibleInstances.map((inst) => {
                      const isCancelled = inst.status === "cancelled";
                      const isActive = inst.status === "active";
                      const hasConflict = inst.conflictWarnings.length > 0;

                      return (
                        <InstanceCard
                          key={inst.id}
                          {...(onInstanceClick
                            ? {
                                type: "button" as const,
                                onClick: () => onInstanceClick(inst),
                              }
                            : {})}
                          className={`pointer-events-auto flex min-w-0 items-center gap-1.5 rounded-lg border border-l-[3px] bg-white px-1.5 py-1 text-[11px] shadow-sm ${
                            isCancelled
                              ? "border-moto-red border-dashed text-gray-400 line-through"
                              : "border-gray-200 text-gray-700"
                          }`}
                          style={{
                            borderLeftColor: isCancelled
                              ? MOTO_COLOR_PALETTE.red.base
                              : (inst.planningTrackColor ??
                                TIMETABLE_UNTYPED_EDGE_COLOR),
                          }}
                        >
                          <span
                            className="h-1.5 w-1.5 shrink-0 rounded-full"
                            style={{
                              backgroundColor: isCancelled
                                ? MOTO_COLOR_PALETTE.red.base
                                : (inst.planningTrackColor ??
                                  TIMETABLE_UNTYPED_EDGE_COLOR),
                            }}
                            aria-hidden
                          />
                          <span className="min-w-0 flex-1 truncate font-medium">
                            {inst.title}
                          </span>
                          {inst.planningTrackName && (
                            <span className="sr-only">
                              Planungsspur {inst.planningTrackName}
                            </span>
                          )}
                          {inst.isSpontaneous && !isCancelled && (
                            <span
                              className="shrink-0 rounded-full bg-gray-100 px-1 text-[9px] font-bold tracking-wide text-gray-600 uppercase"
                              title="Dieser Termin wurde spontan gestartet und war nicht geplant."
                            >
                              Spontan
                            </span>
                          )}
                          {isActive && !isCancelled && (
                            <span
                              className="bg-moto-green h-1.5 w-1.5 shrink-0 rounded-full"
                              aria-label="läuft"
                            />
                          )}
                          {hasConflict && (
                            <AlertTriangle
                              className="text-moto-amber h-3 w-3 shrink-0"
                              aria-label={`${inst.conflictWarnings.length} Konflikte`}
                            />
                          )}
                          {!isCancelled && inst.requiredStaffCount > 0 && (
                            <TimetableRatioPill
                              variant="dot"
                              icon={null}
                              label="Besetzung"
                              value={`${inst.assignedStaffCount}/${inst.requiredStaffCount}`}
                              tone={capacityTone(
                                inst.assignedStaffCount,
                                inst.requiredStaffCount,
                              )}
                            />
                          )}
                        </InstanceCard>
                      );
                    })}
                    {moreCount > 0 && (
                      <div className="text-[10px] font-medium text-gray-500">
                        + {moreCount} weitere
                      </div>
                    )}
                  </div>
                )}
              </div>
            </Cell>
          );
        })}
      </div>
    </div>
  );
}
