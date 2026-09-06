"use client";

/**
 * PeriodSwitcherDropdown: replaces CalendarPeriodHeaderButton.
 *
 * Fixes the user-reported "du kannst nirgendwo diese Periode auswählen"
 * friction: the previous component only listed periods that already
 * intersect the visible week, so jumping to a totally separate period
 * (next school year, last summer holidays) required editing dates
 * manually.
 *
 * The new dropdown shows three things:
 * 1. Per-day assignment for the visible week, preserves the
 *    "Übergangswoche" insight when a week crosses period boundaries.
 * 2. All registered periods, grouped by status (active / upcoming /
 *    archived). Clicking jumps the calendar to that period's start.
 * 3. A "Neuen Zeitraum anlegen" footer.
 */

import { useCallback, useMemo, useRef, useState } from "react";
import Link from "~/components/ui/navigation-link";
import { Check, ChevronDown, Plus, SlidersHorizontal } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { useClickOutside } from "~/lib/hooks/use-click-outside";
import {
  timetablePopoverSurface,
  timetableWarningText,
} from "./timetable-style";

import {
  type CalendarPeriod,
  formatPeriodRange,
  mapPeriodsForDates,
  uniqueAssignedPeriods,
  weekCycleLabel,
  weekCycleSlotForDate,
  weekCycleSlotLetter,
} from "~/lib/calendar-period-helpers";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { getGermanWeekdayShort, toISODate } from "~/lib/timetable-helpers";

interface PeriodSwitcherDropdownProps {
  periods: CalendarPeriod[];
  weekDays: Date[];
  view?: "day" | "week" | "month" | "series";
  selectedPeriodId?: string | null;
  isLoading?: boolean;
  /** Open the create modal. */
  onCreate: () => void;
  /** Open the edit modal for an existing period. */
  onEdit: (period: CalendarPeriod) => void;
  /** Jump the calendar view to the given period's start. */
  onSelect: (period: CalendarPeriod) => void;
  /**
   * Leseansicht (#2283): false blendet Anlegen/Bearbeiten und den
   * Verwaltungslink aus — der Umschalter bleibt reine Navigation.
   */
  canManage?: boolean;
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
  canManage = true,
}: PeriodSwitcherDropdownProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const tenantPath = useTenantAwarePath();

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
  const showContextAssignments = view !== "series";
  const contextLabel =
    view === "month"
      ? "Dieser Monat"
      : view === "day"
        ? "Dieser Tag"
        : "Diese Woche";
  const selectedPeriod = selectedPeriodId
    ? (periods.find((period) => period.id === selectedPeriodId) ?? null)
    : null;

  // Headline label on the trigger pill.
  const triggerLabel =
    view === "series" && selectedPeriod
      ? selectedPeriod.name
      : view === "series"
        ? "Zeiträume"
        : assignedPeriods.length === 0
          ? // Ohne Planungsrecht ist "Zeitraum anlegen" eine Aufforderung ins
            // Leere: der Umschalter ist dort reine Auskunft (#2621).
            canManage
            ? "Zeitraum anlegen"
            : "Kein Planungszeitraum"
          : assignedPeriods.length === 1 && !hasMissingDays
            ? assignedPeriods[0]!.name
            : view === "week"
              ? "Übergangswoche"
              : "Mehrere Zeiträume";

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
      {
        label: "Abgelaufen oder deaktiviert",
        periods: archived.sort(byStart).reverse(),
      },
    ].filter((g) => g.periods.length > 0);
  }, [periods]);

  // Click-outside / Escape to close.
  const closeMenu = useCallback(() => setOpen(false), []);
  useClickOutside(containerRef, closeMenu, open);

  // While loading, render a skeleton sized like the trigger pill so the
  // header keeps its place instead of popping in when periods resolve.
  if (isLoading) {
    return <Skeleton className="h-8 w-44 rounded-lg" />;
  }

  // Empty state: no periods exist at all.
  if (periods.length === 0) {
    if (!canManage) return null;
    return (
      <Button
        type="button"
        variant="primary"
        size="compact"
        onClick={onCreate}
        title="Ohne aktiven Zeitraum können keine regelmäßigen Termine eingetragen werden."
      >
        Zeitraum anlegen
      </Button>
    );
  }

  return (
    <div className="relative" ref={containerRef}>
      <Button
        type="button"
        variant="ghost"
        size="compact"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="dialog"
        className="-ml-2.5 max-w-[240px]"
        title="Planungszeitraum wechseln"
      >
        <span
          className="bg-moto-green h-1.5 w-1.5 shrink-0 rounded-full"
          aria-hidden
        />
        <span className="truncate">{triggerLabel}</span>
        <ChevronDown className="h-3.5 w-3.5 text-gray-400" aria-hidden />
      </Button>

      {open && (
        <div
          className={`absolute left-0 z-30 mt-2 w-[calc(100vw-3rem)] max-w-96 sm:right-0 sm:left-auto sm:w-96 ${timetablePopoverSurface}`}
        >
          <div className="border-b border-gray-100 px-4 py-3">
            <p className="text-sm font-semibold text-gray-900">
              Planungszeitraum
            </p>
            <p className="text-[11px] text-gray-500">
              Zeiträume legen fest, in welchen Wochen regelmäßige Termine
              gelten. So kannst du Schuljahr, Ferien oder besondere Phasen
              getrennt planen.
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
                        {a.period ? (
                          <>
                            {a.period.name}
                            {/* Wochen-Rhythmus sichtbar machen (#1946):
                                weekCycleSlotForDate liefert den 1-basierten
                                Slot auch für 3-/4-Wochen-Zyklen (C, D, …). */}
                            {(() => {
                              const slot = weekCycleSlotForDate(
                                a.period,
                                a.date,
                              );
                              return slot ? (
                                <span className="text-gray-400">
                                  {" "}
                                  · Woche {weekCycleSlotLetter(slot)}
                                </span>
                              ) : null;
                            })()}
                          </>
                        ) : (
                          <span className={timetableWarningText}>
                            Kein aktiver Zeitraum
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
                  const isSelected = selectedPeriodId
                    ? p.id === selectedPeriodId
                    : false;
                  return (
                    <div key={p.id} className="flex items-center gap-1 px-2">
                      <button
                        type="button"
                        onClick={() => {
                          setOpen(false);
                          onSelect(p);
                        }}
                        className={`group relative min-w-0 flex-1 rounded-lg px-2 py-1.5 pr-8 text-left transition-colors hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                          isSelected ? "bg-gray-100" : ""
                        }`}
                      >
                        <div className="min-w-0">
                          <span className="truncate text-xs font-medium text-gray-900">
                            {p.name}
                          </span>
                        </div>
                        <p className="text-[10px] text-gray-500 tabular-nums">
                          {formatPeriodRange(p)}
                          {(() => {
                            const label = weekCycleLabel(p.weekCycleLength);
                            return label ? ` · ${label}` : null;
                          })()}
                        </p>
                        {isSelected && (
                          <Check
                            data-testid="selected-period-check"
                            className="absolute top-1/2 right-2 h-3.5 w-3.5 -translate-y-1/2 text-gray-900"
                            aria-hidden
                          />
                        )}
                      </button>
                      {canManage && (
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
                      )}
                    </div>
                  );
                })}
              </div>
            ))}
          </div>

          {/* Footer: create new */}
          {canManage && (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => {
                setOpen(false);
                onCreate();
              }}
              className="w-full justify-start rounded-none border-t border-gray-100 px-4"
            >
              <Plus className="h-4 w-4" aria-hidden /> Neuen Zeitraum anlegen
            </Button>
          )}
          {/* Verwaltungslink: /calendar-periods ist auch als Unterpunkt im
              Planung-Bereich der Sidebar erreichbar (#1946); der Chip bleibt
              als direkter Weg aus dem Planungskontext. tenantPath hält den
              Link im Path-Routing-Modus innerhalb des Tenant-Segments. */}
          {canManage && (
            <Link
              href={tenantPath("/calendar-periods")}
              onClick={() => setOpen(false)}
              className="flex h-8 w-full items-center gap-1 border-t border-gray-100 px-4 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-100"
            >
              <SlidersHorizontal className="h-4 w-4" aria-hidden /> Zeiträume
              verwalten
            </Link>
          )}
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
        <p className={`text-xs ${timetableWarningText}`}>
          Für diesen Monat ist kein aktiver Zeitraum hinterlegt.
        </p>
      </div>
    );
  }

  return (
    <div className="border-b border-gray-100 bg-gray-50/60 px-4 py-2.5">
      <p className="mb-1 text-[10px] font-semibold tracking-wider text-gray-500 uppercase">
        Dieser Monat
      </p>
      <p className="mb-1.5 text-xs text-gray-700">Umfasst mehrere Zeiträume.</p>
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
          <p className={`text-[11px] ${timetableWarningText}`}>
            Einige Tage haben keinen aktiven Zeitraum.
          </p>
        )}
      </div>
    </div>
  );
}
