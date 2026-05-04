"use client";

/**
 * PeriodSwitcherDropdown — replaces CalendarPeriodHeaderButton.
 *
 * Fixes the user-reported "du kannst nirgendwo diese Periode auswählen"
 * friction: the previous component only listed periods that already
 * intersect the visible week, so jumping to a totally separate period
 * (next school year, last summer holidays) required editing dates
 * manually.
 *
 * The new dropdown shows three things:
 * 1. Per-day assignment for the visible week — preserves the
 *    "Grenzwoche" insight when a week crosses period boundaries.
 * 2. All registered periods, grouped by status (active / upcoming /
 *    archived). Clicking jumps the calendar to that period's start.
 * 3. A "Neue Periode anlegen" footer.
 */

import { useEffect, useMemo, useRef, useState } from "react";

import {
  type CalendarPeriod,
  formatPeriodRange,
  mapPeriodsForDates,
  uniqueAssignedPeriods,
} from "~/lib/calendar-period-helpers";
import { getGermanWeekdayShort, toISODate } from "~/lib/timetable-helpers";

interface PeriodSwitcherDropdownProps {
  periods: CalendarPeriod[];
  weekDays: Date[];
  isLoading?: boolean;
  /** Open the create modal. */
  onCreate: () => void;
  /** Open the edit modal for an existing period. */
  onEdit: (period: CalendarPeriod) => void;
  /** Jump the calendar view to the given period's start. */
  onSelect: (period: CalendarPeriod) => void;
}

interface PeriodGroup {
  label: string;
  periods: CalendarPeriod[];
}

export function PeriodSwitcherDropdown({
  periods,
  weekDays,
  isLoading = false,
  onCreate,
  onEdit,
  onSelect,
}: PeriodSwitcherDropdownProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);

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

  // Headline label on the trigger pill.
  const triggerLabel =
    assignedPeriods.length === 0
      ? "Periode anlegen"
      : assignedPeriods.length === 1 && !hasMissingDays
        ? assignedPeriods[0]!.name
        : "Grenzwoche";

  // Group all periods for the list section.
  const grouped = useMemo<PeriodGroup[]>(() => {
    const today = toISODate(new Date());
    const active: CalendarPeriod[] = [];
    const upcoming: CalendarPeriod[] = [];
    const archived: CalendarPeriod[] = [];
    for (const p of periods) {
      if (!p.isActive) {
        archived.push(p);
        continue;
      }
      if (p.endDate < today) archived.push(p);
      else if (p.startDate > today) upcoming.push(p);
      else active.push(p);
    }
    const byStart = (a: CalendarPeriod, b: CalendarPeriod) =>
      a.startDate.localeCompare(b.startDate);
    return [
      { label: "Aktiv", periods: active.sort(byStart) },
      { label: "Geplant", periods: upcoming.sort(byStart) },
      { label: "Archiv", periods: archived.sort(byStart).reverse() },
    ].filter((g) => g.periods.length > 0);
  }, [periods]);

  // Click-outside to close.
  useEffect(() => {
    if (!open) return;
    function onDocClick(event: MouseEvent) {
      if (
        containerRef.current &&
        event.target instanceof Node &&
        !containerRef.current.contains(event.target)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [open]);

  if (isLoading) return null;

  // Empty state — no periods exist at all.
  if (periods.length === 0) {
    return (
      <button
        type="button"
        onClick={onCreate}
        className="inline-flex h-8 items-center gap-1.5 rounded-md border border-amber-300 bg-amber-50 px-3 text-[12px] font-medium text-amber-900 transition-colors hover:bg-amber-100"
        title="Ohne aktive Kalenderperiode kann der Plan nicht materialisiert werden."
      >
        Periode anlegen
      </button>
    );
  }

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="dialog"
        className="inline-flex h-8 max-w-[240px] items-center gap-1.5 rounded-md border border-slate-200 bg-white px-2.5 text-[12px] font-medium text-slate-700 transition-colors hover:bg-slate-50"
        title="Planungsperiode wechseln"
      >
        <span
          className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#83CD2D]"
          aria-hidden
        />
        <span className="truncate">{triggerLabel}</span>
        <span aria-hidden className="text-slate-400">
          ▾
        </span>
      </button>

      {open && (
        <div className="absolute right-0 z-30 mt-2 w-96 overflow-hidden rounded-lg border border-slate-200 bg-white shadow-md">
          <div className="border-b border-slate-100 px-4 py-3">
            <p className="text-sm font-semibold text-slate-900">
              Planungsperiode
            </p>
            <p className="text-[11px] text-slate-500">
              Wähle eine Periode oder erstelle eine neue.
            </p>
          </div>

          {/* Per-day assignment (preserves Grenzwoche insight) */}
          <div className="border-b border-slate-100 bg-slate-50/60 px-4 py-2.5">
            <p className="mb-1 text-[10px] font-semibold tracking-wider text-slate-500 uppercase">
              Diese Woche
            </p>
            <div className="space-y-0.5">
              {assignments.map((a, index) => {
                const day = weekDays[index]!;
                return (
                  <div
                    key={a.date}
                    className="flex items-center justify-between gap-3 text-[11px]"
                  >
                    <span className="font-medium text-slate-500">
                      {getGermanWeekdayShort(day)}
                    </span>
                    <span className="min-w-0 flex-1 truncate text-right text-slate-700">
                      {a.period?.name ?? (
                        <span className="text-amber-700">
                          Keine aktive Periode
                        </span>
                      )}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>

          {/* All periods, grouped */}
          <div className="max-h-72 overflow-y-auto py-1">
            {grouped.map((group) => (
              <div key={group.label} className="py-1">
                <p className="px-4 py-1 text-[10px] font-semibold tracking-wider text-slate-400 uppercase">
                  {group.label}
                </p>
                {group.periods.map((p) => {
                  const isAssigned = assignedPeriods.some(
                    (ap) => ap.id === p.id,
                  );
                  return (
                    <div key={p.id} className="flex items-center gap-1 px-2">
                      <button
                        type="button"
                        onClick={() => {
                          setOpen(false);
                          onSelect(p);
                        }}
                        className={`group min-w-0 flex-1 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-slate-100 ${
                          isAssigned ? "bg-slate-100" : ""
                        }`}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="truncate text-[12px] font-medium text-slate-900">
                            {p.name}
                          </span>
                          {isAssigned && (
                            <span className="shrink-0 text-[10px] font-semibold text-slate-900">
                              ✓
                            </span>
                          )}
                        </div>
                        <p className="text-[10px] text-slate-500 tabular-nums">
                          {formatPeriodRange(p)}
                        </p>
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setOpen(false);
                          onEdit(p);
                        }}
                        className="rounded-md px-2 py-1 text-[10px] font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900"
                        aria-label={`${p.name} bearbeiten`}
                      >
                        Bearbeiten
                      </button>
                    </div>
                  );
                })}
              </div>
            ))}
          </div>

          {/* Footer: create new */}
          <button
            type="button"
            onClick={() => {
              setOpen(false);
              onCreate();
            }}
            className="flex w-full items-center gap-1.5 border-t border-slate-100 px-4 py-2.5 text-left text-[12px] font-medium text-slate-700 transition-colors hover:bg-slate-50"
          >
            <span className="text-base leading-none">+</span> Neue Periode
            anlegen
          </button>
        </div>
      )}
    </div>
  );
}
