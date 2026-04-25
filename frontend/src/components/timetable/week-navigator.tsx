"use client";

/**
 * WeekNavigator — header row with prev/next/today controls and the
 * formatted KW label. Stays compact so it can sit alongside actions in the
 * page header. Adds a "Heute"-Pill when offset !== 0.
 */

import { ChevronLeft, ChevronRight, RotateCcw } from "lucide-react";

import { formatWeekLabel } from "~/lib/timetable-helpers";

interface WeekNavigatorProps {
  weekOffset: number;
  weekRange: { from: Date; to: Date };
  onChange: (newOffset: number) => void;
}

export function WeekNavigator({
  weekOffset,
  weekRange,
  onChange,
}: WeekNavigatorProps) {
  const isCurrent = weekOffset === 0;
  return (
    <div className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white p-1.5">
      <button
        type="button"
        onClick={() => onChange(weekOffset - 1)}
        className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-600 hover:bg-slate-100"
        aria-label="Vorherige Woche"
      >
        <ChevronLeft className="h-4 w-4" />
      </button>
      <span className="px-2 text-sm font-semibold text-slate-900">
        {formatWeekLabel(weekRange.from, weekRange.to)}
      </span>
      <button
        type="button"
        onClick={() => onChange(weekOffset + 1)}
        className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-600 hover:bg-slate-100"
        aria-label="Nächste Woche"
      >
        <ChevronRight className="h-4 w-4" />
      </button>
      {!isCurrent && (
        <button
          type="button"
          onClick={() => onChange(0)}
          className="ml-1 inline-flex items-center gap-1 rounded-md bg-slate-100 px-2.5 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-200"
        >
          <RotateCcw className="h-3 w-3" />
          Heute
        </button>
      )}
    </div>
  );
}
