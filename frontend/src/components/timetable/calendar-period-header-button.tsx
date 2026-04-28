"use client";

/**
 * CalendarPeriodHeaderButton — compact period context selector for the
 * planner header. Periods are derived from the visible week, not "today", so
 * admins can plan future holidays and school years without stale context.
 *
 * A week may cross a period boundary (holidays often end mid-week), so the
 * dropdown renders the per-day assignment instead of pretending there is one
 * global week period.
 */

import { useMemo, useState } from "react";

import {
  type CalendarPeriod,
  formatPeriodRange,
  mapPeriodsForDates,
  uniqueAssignedPeriods,
} from "~/lib/calendar-period-helpers";
import { getGermanWeekdayShort, toISODate } from "~/lib/timetable-helpers";

interface CalendarPeriodHeaderButtonProps {
  periods: CalendarPeriod[];
  weekDays: Date[];
  isLoading?: boolean;
  onEdit: (period: CalendarPeriod) => void;
  onCreate: () => void;
}

export function CalendarPeriodHeaderButton({
  periods,
  weekDays,
  isLoading = false,
  onEdit,
  onCreate,
}: CalendarPeriodHeaderButtonProps) {
  const [open, setOpen] = useState(false);
  const isoDays = useMemo(() => weekDays.map(toISODate), [weekDays]);
  const assignments = useMemo(
    () => mapPeriodsForDates(periods, isoDays),
    [periods, isoDays],
  );
  const assignedPeriods = useMemo(
    () => uniqueAssignedPeriods(assignments),
    [assignments],
  );
  const hasMissingDays = assignments.some((a) => a.period === null);
  const singlePeriod =
    assignedPeriods.length === 1 && !hasMissingDays ? assignedPeriods[0] : null;
  const buttonLabel = singlePeriod
    ? singlePeriod.name
    : assignedPeriods.length > 0
      ? "Grenzwoche"
      : "Periode anlegen";

  if (isLoading) {
    return null;
  }

  if (assignedPeriods.length === 0) {
    return (
      <button
        type="button"
        onClick={onCreate}
        className="inline-flex items-center gap-2 rounded-md border border-[#FACC15] bg-[#FEF9C3] px-3 py-2 text-xs font-semibold text-[#854D0E] shadow-sm transition-colors hover:bg-[#FEF08A]"
        title="Ohne aktive Kalenderperiode kann der Plan nicht materialisiert werden."
      >
        Periode anlegen
      </button>
    );
  }

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="inline-flex items-center gap-1 rounded-md border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 shadow-sm transition-colors hover:border-slate-300 hover:bg-slate-50"
        title="Planungsperiode ansehen oder bearbeiten"
        aria-expanded={open}
      >
        <span>{buttonLabel}</span>
        <span aria-hidden className="text-slate-400">
          ▾
        </span>
      </button>

      {open && (
        <div className="absolute right-0 z-30 mt-2 w-80 overflow-hidden rounded-lg border border-slate-200 bg-white shadow-xl">
          <div className="border-b border-slate-100 px-3 py-2">
            <p className="text-xs font-bold text-slate-900">Planungsperiode</p>
            <p className="text-[11px] text-slate-500">
              Gilt tagesgenau für die angezeigte Woche.
            </p>
          </div>

          <div className="max-h-64 overflow-y-auto p-2">
            {assignments.map((assignment, index) => {
              const day = weekDays[index]!;
              return (
                <div
                  key={assignment.date}
                  className="flex items-center justify-between gap-3 rounded-md px-2 py-1.5 text-xs"
                >
                  <span className="font-semibold text-slate-500">
                    {getGermanWeekdayShort(day)}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-right text-slate-800">
                    {assignment.period?.name ?? "Keine aktive Periode"}
                  </span>
                </div>
              );
            })}
          </div>

          <div className="border-t border-slate-100 p-2">
            {assignedPeriods.map((period) => (
              <button
                key={period.id}
                type="button"
                onClick={() => {
                  setOpen(false);
                  onEdit(period);
                }}
                className="flex w-full flex-col rounded-md px-2 py-2 text-left text-xs hover:bg-slate-50"
              >
                <span className="font-semibold text-slate-900">
                  {period.name} bearbeiten
                </span>
                <span className="text-[11px] text-slate-500">
                  {formatPeriodRange(period)}
                </span>
              </button>
            ))}
            <button
              type="button"
              onClick={() => {
                setOpen(false);
                onCreate();
              }}
              className="mt-1 w-full rounded-md border border-dashed border-slate-300 px-2 py-2 text-left text-xs font-semibold text-[#5080D8] hover:bg-[#EFF6FF]"
            >
              Neue Periode anlegen
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
