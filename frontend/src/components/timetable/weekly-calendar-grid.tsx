"use client";

/**
 * WeeklyCalendarGrid — Apple-Calendar-style week view.
 *
 * Layout: a sticky day-header row above a vertically scrollable body.
 * Inside the body: a fixed time-gutter on the left and 7 day columns
 * (Mo-So). Each day column is a positioned container; events are placed
 * absolutely via `getEventBlockPosition`. Overlapping events split a
 * column horizontally via `assignBlockLanes`.
 *
 * Replaces the previous vertical-stack `WeeklyPlannerGrid`. The stack
 * approach worked but failed the "where is the 11am AG?" test —
 * positioning by time is the whole point of a week view.
 */

import { useEffect, useMemo, useState } from "react";

import { ClosingDayChip } from "~/components/planning/closing-day-marker";
import { CapacityStrip } from "~/components/ui/capacity-strip";
import {
  assignBlockLanes,
  countPlannedStaff,
  formatDayHeader,
  getCurrentTimeOffset,
  getEventBlockPosition,
  getGermanWeekdayShort,
  groupInstancesByDate,
  parseTimeToMinutes,
  toISODate,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";

import { InstanceBlock } from "./instance-block";
import { timetableSurface } from "./timetable-style";

// Grid template columns: narrow time gutter + day columns. Mobile shows a
// single day column (gutter + 1fr) via the day strip; sm+ shows all seven.
// The two variants are static strings so Tailwind can generate both; the
// single-day variant powers the Vertretungsplan's "Tag" view (#1840) — the
// week planner always passes 7 days and keeps the seven-column layout.
const GRID_COLS_CLASS_WEEK =
  "grid-cols-[40px_minmax(0,1fr)] sm:grid-cols-[64px_repeat(7,minmax(0,1fr))]";
const GRID_COLS_CLASS_DAY =
  "grid-cols-[40px_minmax(0,1fr)] sm:grid-cols-[64px_minmax(0,1fr)]";
// Inline-Style-Pendant zu den sm+-Klassen oben (die Tageskopfzeile rendert
// nur ab sm). Muss mit Gutter-Breite und Spaltenbasis der Klassen-Strings
// übereinstimmen — bei Änderungen BEIDE Stellen anpassen.
const SM_TIME_GUTTER_PX = 64;
const smGridTemplate = (dayCount: number) =>
  `${SM_TIME_GUTTER_PX}px repeat(${dayCount}, minmax(0,1fr))`;

interface WeeklyCalendarGridProps {
  weekDays: Date[]; // Mo-So (7 dates)
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
  /**
   * When provided, every full hour of every day column becomes a click
   * target for creating a new instance at that slot. Without it the grid
   * renders exactly as before (no extra elements).
   */
  onSlotClick?: (dateISO: string, hour: number) => void;
  /**
   * Instanz-IDs offener (nicht quittierter) Personal-Lücken. Ein Block, dessen
   * ID hier liegt, zeigt das eine Lücken-Status-Icon (Betreuungsplan Abschnitt
   * 5.1/5.2). Optional und per Default leer: ohne Set zeigt kein Block ein
   * Lücken-Icon, beide bestehenden Call-Sites bleiben verhaltensgleich.
   */
  gapInstanceIds?: ReadonlySet<string>;
  /**
   * OGS-Schließtage im sichtbaren Fenster, keyed YYYY-MM-DD → Grund (#2032).
   * Betroffene Tagesspalten werden neutral eingefärbt und mit dem Grund
   * beschriftet; geplant werden darf trotzdem (Ferien-/Notbetreuung).
   */
  closingDays?: ReadonlyMap<string, string>;
  emptyState?: {
    title: string;
    description: string;
  };
  /**
   * Betreuungsplan-Tageskopfzeile (06-betreuungsplan.md Abschnitt 3.1): zeigt
   * pro Wochentagsspalte die eingeplante Personenzahl als CapacityStrip-Zeile
   * unter dem Sticky-Tagesheader. Default aus (false) — die
   * Vertretungs-Einzeltagesnutzung dieses Grids bleibt unverändert ohne
   * Kopfzeile.
   */
  showDayHeader?: boolean;
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
  onSlotClick,
  gapInstanceIds,
  closingDays,
  emptyState,
  showDayHeader = false,
}: WeeklyCalendarGridProps) {
  const gridColsClass =
    weekDays.length === 1 ? GRID_COLS_CLASS_DAY : GRID_COLS_CLASS_WEEK;
  const grouped = useMemo(() => groupInstancesByDate(instances), [instances]);
  // Zellen der Tageskopfzeile nur bei Daten-/Wochenwechsel neu ableiten —
  // nicht bei jedem unabhängigen Re-Render (Popover, Selektion, Modals).
  const dayHeaderCells = useMemo(
    () =>
      showDayHeader
        ? weekDays.map((day) => {
            const iso = toISODate(day);
            const count = countPlannedStaff(grouped.get(iso) ?? []);
            return { key: iso, content: `${count} P.` };
          })
        : [],
    [showDayHeader, weekDays, grouped],
  );
  const eventStarts = instances
    .map((instance) => parseTimeToMinutes(instance.startTime))
    .filter((minutes) => Number.isFinite(minutes));
  const eventEnds = instances
    .map((instance) => parseTimeToMinutes(instance.endTime))
    .filter((minutes) => Number.isFinite(minutes));
  const renderStartHour =
    eventStarts.length > 0
      ? Math.max(
          0,
          Math.min(dayStartHour, Math.floor(Math.min(...eventStarts) / 60)),
        )
      : dayStartHour;
  const renderEndHour =
    eventEnds.length > 0
      ? Math.min(
          24,
          Math.max(dayEndHour, Math.ceil(Math.max(...eventEnds) / 60)),
        )
      : dayEndHour;
  const hours = Array.from(
    { length: renderEndHour - renderStartHour + 1 },
    (_, i) => renderStartHour + i,
  );
  const gridHeightPx = (renderEndHour - renderStartHour) * hourHeightPx;

  // Re-render every minute so the now-line moves smoothly.
  const [nowTick, setNowTick] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNowTick(Date.now()), 60_000);
    return () => clearInterval(id);
  }, []);
  void nowTick;

  // Mobile (< sm) shows one day at a time, picked via the day strip. Default
  // to today when it falls in the visible week, otherwise Monday.
  const todayIndex = weekDays.findIndex((day) => toISODate(day) === todayISO);
  const [selectedDayIndex, setSelectedDayIndex] = useState(
    todayIndex >= 0 ? todayIndex : 0,
  );
  const safeSelectedIndex = Math.min(
    Math.max(selectedDayIndex, 0),
    weekDays.length - 1,
  );

  return (
    <div className={`${timetableSurface} overflow-hidden`}>
      {/* Mobile day strip — tap a day to switch (single-day view < sm) */}
      <div className="flex gap-1 border-b border-gray-200 bg-white p-2 sm:hidden">
        {weekDays.map((day, index) => {
          const iso = toISODate(day);
          const isToday = iso === todayISO;
          const isSelected = index === safeSelectedIndex;
          const closingReason = closingDays?.get(iso);
          return (
            <button
              key={iso}
              type="button"
              onClick={() => setSelectedDayIndex(index)}
              aria-pressed={isSelected}
              className={`flex min-w-0 flex-1 flex-col items-center gap-0.5 rounded-lg px-1 py-1.5 transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                isSelected
                  ? "bg-gray-900 text-white"
                  : "text-gray-600 hover:bg-gray-100"
              }`}
            >
              <span
                className={`text-[10px] font-medium tracking-wide uppercase ${
                  isSelected ? "text-white/80" : "text-gray-500"
                }`}
              >
                {getGermanWeekdayShort(day)}
              </span>
              <span className="text-sm font-semibold tabular-nums">
                {String(day.getDate()).padStart(2, "0")}
              </span>
              {closingReason !== undefined && (
                <ClosingDayChip reason={closingReason} variant="compact" />
              )}
              {isToday && !isSelected && (
                <span
                  className="h-1 w-1 rounded-full bg-[#FF3130]"
                  aria-hidden
                />
              )}
            </button>
          );
        })}
      </div>

      {/* Sticky day header (desktop — mobile uses the day strip above) */}
      <div
        className={`hidden h-[52px] border-b border-gray-200 bg-white sm:grid sm:h-14 ${gridColsClass}`}
      >
        <div aria-hidden />
        {weekDays.map((day) => {
          const iso = toISODate(day);
          const isToday = iso === todayISO;
          const closingReason = closingDays?.get(iso);
          return (
            <div
              key={iso}
              className={`flex min-w-0 flex-col items-center justify-center gap-0.5 border-l border-gray-200 px-1 py-1 sm:px-2 ${
                closingReason === undefined ? "" : "bg-gray-50"
              }`}
            >
              <div className="flex min-w-0 items-center gap-1 sm:gap-2">
                <span className="text-[10px] font-medium tracking-wide text-gray-500 uppercase sm:text-[11px]">
                  {getGermanWeekdayShort(day)}
                </span>
                {isToday ? (
                  <span
                    className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-gray-900 text-[11px] font-semibold text-white tabular-nums sm:h-6 sm:w-6 sm:text-xs"
                    aria-label={formatDayHeader(day)}
                  >
                    {day.getDate()}
                  </span>
                ) : (
                  <span
                    className="text-xs font-semibold text-gray-900 tabular-nums sm:text-sm"
                    aria-label={formatDayHeader(day)}
                  >
                    {String(day.getDate()).padStart(2, "0")}
                  </span>
                )}
              </div>
              {closingReason !== undefined && (
                <ClosingDayChip
                  reason={closingReason}
                  className="w-full justify-center"
                />
              )}
            </div>
          );
        })}
      </div>

      {/* Day-header capacity strip (Betreuungsplan opt-in, 06-betreuungsplan.md
          Abschnitt 3.1) — sm+ only, matches the desktop day-header columns;
          on mobile the day strip above already shows one day at a time. */}
      {showDayHeader && (
        <div className="hidden sm:block">
          <CapacityStrip
            as="div"
            position="header"
            stickyLabel={false}
            labelWidthClassName="w-[64px]"
            rowLabel=""
            gridTemplateColumns={smGridTemplate(weekDays.length)}
            cells={dayHeaderCells}
          />
        </div>
      )}

      {/* Scrollable body */}
      <div className="relative max-h-[720px] overflow-y-auto">
        <div className={`grid ${gridColsClass}`}>
          {/* Time gutter */}
          <div
            className="border-r border-gray-200 bg-gray-50"
            style={{ height: `${gridHeightPx}px` }}
          >
            {hours.slice(0, -1).map((hour) => (
              <div
                key={hour}
                className="flex items-start justify-end px-1 pt-1 text-[10px] font-medium text-gray-400 sm:px-2 sm:text-[11px]"
                style={{ height: `${hourHeightPx}px` }}
              >
                {String(hour).padStart(2, "0")}:00
              </div>
            ))}
          </div>

          {/* Day columns */}
          {weekDays.map((day, dayIndex) => {
            const iso = toISODate(day);
            const isToday = iso === todayISO;
            const dayInstances = grouped.get(iso) ?? [];
            const closingReason = closingDays?.get(iso);
            const laned = assignBlockLanes(dayInstances);
            const nowOffset = isToday
              ? getCurrentTimeOffset(
                  hourHeightPx,
                  renderStartHour,
                  renderEndHour,
                )
              : null;

            return (
              <div
                key={iso}
                className={`relative min-w-0 border-l border-gray-200 ${closingReason !== undefined ? "bg-gray-100/70" : isToday ? "bg-gray-50/60" : ""} ${dayIndex === safeSelectedIndex ? "" : "hidden sm:block"}`}
                style={{ height: `${gridHeightPx}px` }}
              >
                {/* Hour grid lines */}
                {hours.slice(1, -1).map((hour) => {
                  const top = (hour - renderStartHour) * hourHeightPx;
                  return (
                    <div
                      key={hour}
                      className="pointer-events-none absolute right-0 left-0 border-t border-gray-100"
                      style={{ top: `${top}px` }}
                    />
                  );
                })}

                {/* Half-hour grid lines (lighter) */}
                {hours.slice(0, -1).map((hour) => {
                  const top =
                    (hour - renderStartHour) * hourHeightPx + hourHeightPx / 2;
                  return (
                    <div
                      key={`half-${hour}`}
                      className="pointer-events-none absolute right-0 left-0 border-t border-gray-50"
                      style={{ top: `${top}px` }}
                    />
                  );
                })}

                {/* Empty-slot click targets — before the event blocks in DOM
                    order, so blocks paint on top and keep their own click
                    handlers without z-index tricks. */}
                {onSlotClick &&
                  hours.slice(0, -1).map((hour) => (
                    <button
                      key={`slot-${hour}`}
                      type="button"
                      // Out of the tab order: ~100 hourly slots would bury
                      // the first event behind dozens of tab stops; keyboard
                      // users create events via the "Neu" menu instead.
                      tabIndex={-1}
                      onClick={() => onSlotClick(iso, hour)}
                      aria-label={`Neuen Termin anlegen: ${formatDayHeader(day)}, ${String(hour).padStart(2, "0")}:00 Uhr`}
                      className="group absolute inset-x-0 flex items-center justify-center focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset"
                      style={{
                        top: `${(hour - renderStartHour) * hourHeightPx}px`,
                        height: `${hourHeightPx}px`,
                      }}
                    >
                      <span className="pointer-events-none hidden rounded-lg border border-gray-200 bg-white px-1.5 py-0.5 text-[11px] font-medium text-gray-500 shadow-sm group-hover:inline group-focus-visible:inline">
                        + Termin
                      </span>
                    </button>
                  ))}

                {/* Event blocks */}
                {laned.map(({ instance, lane, laneCount }) => {
                  const { top, height } = getEventBlockPosition(
                    instance.startTime,
                    instance.endTime,
                    hourHeightPx,
                    renderStartHour,
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
                      isGap={gapInstanceIds?.has(instance.id) ?? false}
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

                {/* Per-column hint — only on a genuinely empty day, not when
                    the whole-week empty overlay is showing. */}
                {dayInstances.length === 0 && !emptyState && (
                  <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-1">
                    {closingReason === undefined ? (
                      <span className="hidden text-[11px] text-gray-400 italic sm:inline">
                        Keine Aktivitäten
                      </span>
                    ) : (
                      <ClosingDayChip reason={closingReason} />
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>

        {emptyState && (
          <div className="pointer-events-none absolute inset-x-0 top-1/2 z-20 flex -translate-y-1/2 justify-center px-4">
            <div className="moto-content-surface max-w-sm rounded-2xl border border-gray-200 bg-white/95 px-4 py-3 text-center shadow-sm backdrop-blur">
              <h3 className="text-sm font-semibold text-gray-900">
                {emptyState.title}
              </h3>
              <p className="mt-1 text-xs leading-relaxed text-gray-500">
                {emptyState.description}
              </p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
