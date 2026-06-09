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

import { useMemo, useRef, useState } from "react";
import { Check, ChevronDown, Plus } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { useClickOutside } from "~/lib/hooks/use-click-outside";

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
  view?: "week" | "month" | "year" | "series";
  selectedPeriodId?: string | null;
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
  view = "week",
  selectedPeriodId = null,
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
  const showContextAssignments = view !== "year" && view !== "series";
  const contextLabel = view === "month" ? "Dieser Monat" : "Diese Woche";
  const selectedPeriod = selectedPeriodId
    ? (periods.find((period) => period.id === selectedPeriodId) ?? null)
    : null;

  // Headline label on the trigger pill.
  const triggerLabel =
    view === "year"
      ? "Planungsperioden"
      : view === "series" && selectedPeriod
        ? selectedPeriod.name
        : view === "series"
          ? "Planungsperioden"
          : assignedPeriods.length === 0
            ? "Periode anlegen"
            : assignedPeriods.length === 1 && !hasMissingDays
              ? assignedPeriods[0]!.name
              : view === "week"
                ? "Grenzwoche"
                : "Mehrere Perioden";

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

  // Click-outside / Escape to close.
  useClickOutside(containerRef, () => setOpen(false), open);

  // While loading, render a skeleton sized like the trigger pill so the
  // header keeps its place instead of popping in when periods resolve.
  if (isLoading) {
    return <Skeleton className="h-8 w-44 rounded-md" />;
  }

  // Empty state — no periods exist at all.
  if (periods.length === 0) {
    return (
      <Button
        type="button"
        variant="primary"
        size="compact"
        onClick={onCreate}
        title="Ohne aktive Kalenderperiode kann der Plan nicht materialisiert werden."
      >
        Periode anlegen
      </Button>
    );
  }

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="dialog"
        className="inline-flex h-8 max-w-[240px] items-center gap-1.5 rounded-md border border-gray-200 bg-white px-2.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        title="Planungsperiode wechseln"
      >
        <span
          className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#83CD2D]"
          aria-hidden
        />
        <span className="truncate">{triggerLabel}</span>
        <ChevronDown className="h-3.5 w-3.5 text-gray-400" aria-hidden />
      </button>

      {open && (
        <div className="absolute left-0 z-30 mt-2 w-[calc(100vw-3rem)] max-w-96 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg sm:right-0 sm:left-auto sm:w-96">
          <div className="border-b border-gray-100 px-4 py-3">
            <p className="text-sm font-semibold text-gray-900">
              Planungsperiode
            </p>
            <p className="text-[11px] text-gray-500">
              Wähle eine Periode oder erstelle eine neue.
            </p>
          </div>

          {showContextAssignments && view === "month" && (
            <MonthPeriodSummary
              assignedPeriods={assignedPeriods}
              hasMissingDays={hasMissingDays}
            />
          )}

          {showContextAssignments && view !== "month" && (
            <div className="border-b border-gray-100 bg-gray-50/60 px-4 py-2.5">
              <p className="mb-1 text-[10px] font-semibold tracking-wider text-gray-500 uppercase">
                {contextLabel}
              </p>
              <div className="space-y-0.5">
                {assignments.map((a, index) => {
                  const day = weekDays[index]!;
                  return (
                    <div
                      key={a.date}
                      className="flex items-center justify-between gap-3 text-[11px]"
                    >
                      <span className="font-medium text-gray-500">
                        {getGermanWeekdayShort(day)}
                      </span>
                      <span className="min-w-0 flex-1 truncate text-right text-gray-700">
                        {a.period?.name ?? (
                          <span className="text-[#EAB308]">
                            Keine aktive Periode
                          </span>
                        )}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* All periods, grouped */}
          <div className="max-h-72 overflow-y-auto py-1">
            {grouped.map((group) => (
              <div key={group.label} className="py-1">
                <p className="px-4 py-1 text-[10px] font-semibold tracking-wider text-gray-400 uppercase">
                  {group.label}
                </p>
                {group.periods.map((p) => {
                  const isAssigned = selectedPeriodId
                    ? p.id === selectedPeriodId
                    : view !== "year" &&
                      assignedPeriods.some((ap) => ap.id === p.id);
                  return (
                    <div key={p.id} className="flex items-center gap-1 px-2">
                      <button
                        type="button"
                        onClick={() => {
                          setOpen(false);
                          onSelect(p);
                        }}
                        className={`group min-w-0 flex-1 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-gray-100 ${
                          isAssigned ? "bg-gray-100" : ""
                        }`}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="truncate text-xs font-medium text-gray-900">
                            {p.name}
                          </span>
                          {isAssigned && (
                            <Check
                              className="h-3.5 w-3.5 shrink-0 text-gray-900"
                              aria-hidden
                            />
                          )}
                        </div>
                        <p className="text-[10px] text-gray-500 tabular-nums">
                          {formatPeriodRange(p)}
                        </p>
                      </button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="compact"
                        onClick={() => {
                          setOpen(false);
                          onEdit(p);
                        }}
                        aria-label={`${p.name} bearbeiten`}
                      >
                        Bearbeiten
                      </Button>
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
            className="flex w-full items-center gap-1.5 border-t border-gray-100 px-4 py-2.5 text-left text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            <Plus className="h-4 w-4" aria-hidden /> Neue Periode anlegen
          </button>
        </div>
      )}
    </div>
  );
}

function MonthPeriodSummary({
  assignedPeriods,
  hasMissingDays,
}: {
  assignedPeriods: CalendarPeriod[];
  hasMissingDays: boolean;
}) {
  if (assignedPeriods.length === 1 && !hasMissingDays) {
    const period = assignedPeriods[0]!;
    return (
      <div className="border-b border-gray-100 bg-gray-50/60 px-4 py-2.5">
        <p className="mb-1 text-[10px] font-semibold tracking-wider text-gray-500 uppercase">
          Dieser Monat
        </p>
        <p className="text-xs text-gray-700">
          Liegt komplett in{" "}
          <span className="font-semibold text-gray-900">{period.name}</span>.
        </p>
      </div>
    );
  }

  if (assignedPeriods.length === 0) {
    return (
      <div className="border-b border-gray-100 bg-gray-50/60 px-4 py-2.5">
        <p className="mb-1 text-[10px] font-semibold tracking-wider text-gray-500 uppercase">
          Dieser Monat
        </p>
        <p className="text-xs text-[#EAB308]">
          Für diesen Monat ist keine aktive Periode hinterlegt.
        </p>
      </div>
    );
  }

  return (
    <div className="border-b border-gray-100 bg-gray-50/60 px-4 py-2.5">
      <p className="mb-1 text-[10px] font-semibold tracking-wider text-gray-500 uppercase">
        Dieser Monat
      </p>
      <p className="mb-1.5 text-xs text-gray-700">Umfasst mehrere Perioden.</p>
      <div className="space-y-0.5">
        {assignedPeriods.map((period) => (
          <p
            key={period.id}
            className="truncate text-[11px] text-gray-600 tabular-nums"
          >
            <span className="font-medium text-gray-800">{period.name}</span>{" "}
            <span className="text-gray-400">{formatPeriodRange(period)}</span>
          </p>
        ))}
        {hasMissingDays && (
          <p className="text-[11px] text-[#EAB308]">
            Einige Tage haben keine aktive Periode.
          </p>
        )}
      </div>
    </div>
  );
}
