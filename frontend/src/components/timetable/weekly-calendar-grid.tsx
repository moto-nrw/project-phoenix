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

import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";

import { ClosingDayChip } from "~/components/planning/closing-day-marker";
import { Alert } from "~/components/ui/alert";
import { CapacityStrip } from "~/components/ui/capacity-strip";
import { EmptyState } from "~/components/ui/empty-state";
import { PlanAddAffordance } from "~/components/ui/plan-add-affordance";
import {
  assignBlockLanes,
  buildDayColumnTemplate,
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
// single day column via the day strip; ab sm bekommt JEDER übergebene Tag
// eine Spalte — eine feste Sieben würde die 1-Tages-Ansicht der Vertretung
// (#1840) und ihre 5-Tage-Woche (#2030) auf einen Bruchteil der Breite
// stauchen und den Rest der Fläche leer lassen.
//
// Die Spaltenbreite folgt den parallelen Blöcken eines Tages
// (`buildDayColumnTemplate`): früher bekam jeder Tag ein gleiches Siebtel,
// und fünf parallele AGs teilten sich dieses Siebtel zu unlesbaren Streifen.
// Reicht die Fläche nicht, scrollt das Raster waagrecht; Zeitleiste, Kopf-
// und Kapazitätszeile bleiben dabei stehen.
//
// Tailwind kann nur statische Klassen erzeugen, deshalb kommen Spalten und
// Mindestbreite aus CSS-Variablen auf dem Wurzelelement, vererbt an
// Kopfzeile und Raster. Die Tageskopfzeile (CapacityStrip) nutzt dieselbe
// Vorlage — es gibt genau EINE Quelle für die Spaltenbreiten.
const GRID_COLS_CLASS =
  "grid-cols-(--day-grid-cols-mobile) sm:grid-cols-(--day-grid-cols)";
const MOBILE_TIME_GUTTER_PX = 40;
const SM_TIME_GUTTER_PX = 64;

interface WeeklyCalendarGridProps {
  // The caller chooses the visible dates (e.g. Mo-Fr in planning, a single
  // day or Mo-Fr in substitutions). The grid deliberately has no fixed week
  // length.
  weekDays: Date[];
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
   * Mindestbreite einer Spur paralleler Blöcke, aus derselben Zoomstufe wie
   * `hourHeightPx`. Ohne Wert die lesbare Normalbreite.
   */
  laneMinWidthPx?: number;
  /**
   * When provided, every full hour of every day column becomes a click
   * target for creating a new instance at that slot. Without it the grid
   * renders exactly as before (no extra elements).
   */
  onSlotClick?: (dateISO: string, hour: number) => void;
  /**
   * Calendar dates that stay visible but do not accept new timetable blocks.
   * Existing blocks remain visible so historical data is never hidden.
   */
  planningDisabledDateISOs?: ReadonlySet<string>;
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
   * Betreuungsplan-Tageskopfzeile: zeigt
   * pro Wochentagsspalte die eingeplante Personenzahl als CapacityStrip-Zeile
   * unter dem Sticky-Tagesheader. Default aus (false) — die
   * Vertretungs-Einzeltagesnutzung dieses Grids bleibt unverändert ohne
   * Kopfzeile.
   */
  showDayHeader?: boolean;
  /**
   * Legende der Farbcodierung dieses Rasters (Bauart 3, Regel 3). Sie sitzt
   * als Fußband INNERHALB der Rasterfläche: auf dem gemusterten Grund der
   * Planungsseiten steht kein Text (BAUARTEN-SPEC Teil 3).
   */
  legend?: ReactNode;
  /**
   * Personal-ID → voller Name: die Blöcke zeigen damit, wer sie betreut
   * (#3817). Ohne Map zeigen sie nur die Zahlen.
   */
  staffNames?: ReadonlyMap<string, string>;
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
  laneMinWidthPx,
  onSlotClick,
  planningDisabledDateISOs,
  gapInstanceIds,
  closingDays,
  emptyState,
  showDayHeader = false,
  legend,
  staffNames,
}: WeeklyCalendarGridProps) {
  const gridColsClass = GRID_COLS_CLASS;
  const grouped = useMemo(() => groupInstancesByDate(instances), [instances]);
  // Spuren je Tag einmal pro Datenstand: sie bestimmen sowohl die Lage der
  // Blöcke als auch die Spaltenbreite.
  const lanedByDay = useMemo(
    () =>
      weekDays.map((day) =>
        assignBlockLanes(grouped.get(toISODate(day)) ?? []),
      ),
    [weekDays, grouped],
  );
  const dayLaneCounts = lanedByDay.map((laned) =>
    laned.reduce((max, { laneCount }) => Math.max(max, laneCount), 0),
  );
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
  const smColumns = buildDayColumnTemplate(
    dayLaneCounts,
    SM_TIME_GUTTER_PX,
    laneMinWidthPx,
  );
  const mobileColumns = buildDayColumnTemplate(
    [dayLaneCounts[safeSelectedIndex] ?? 0],
    MOBILE_TIME_GUTTER_PX,
    laneMinWidthPx,
  );

  // Liegt heute außerhalb des sichtbaren Ausschnitts (breite Tage davor),
  // rückt das Raster beim Öffnen einer Woche die heutige Spalte ins Bild;
  // eine andere Woche beginnt wieder am Montag. Eine Spalte, die schon ganz
  // zu sehen ist, verschiebt nichts — das Raster springt dem Lesenden nicht
  // unter der Hand weg.
  const scrollRef = useRef<HTMLDivElement>(null);
  const weekKey = weekDays.map(toISODate).join(",");
  const smTemplateKey = smColumns.template;
  useEffect(() => {
    const scroller = scrollRef.current;
    if (!scroller) return;
    const todayColumn = scroller.querySelector<HTMLElement>(
      "[data-today-column]",
    );
    const gutter = scroller.querySelector<HTMLElement>("[data-time-gutter]");
    if (!todayColumn || todayColumn.offsetParent === null) {
      scroller.scrollLeft = 0;
      return;
    }
    const left = todayColumn.offsetLeft - (gutter?.offsetWidth ?? 0);
    const right = todayColumn.offsetLeft + todayColumn.offsetWidth;
    if (
      left < scroller.scrollLeft ||
      right > scroller.scrollLeft + scroller.clientWidth
    ) {
      scroller.scrollLeft = Math.max(left, 0);
    }
  }, [weekKey, todayISO, smTemplateKey]);

  const planningDisabledDayCount = weekDays.reduce(
    (count, day) =>
      count + (planningDisabledDateISOs?.has(toISODate(day)) ? 1 : 0),
    0,
  );

  return (
    <div
      className={`${timetableSurface} overflow-hidden`}
      style={
        {
          "--day-grid-cols": smColumns.template,
          "--day-grid-cols-mobile": mobileColumns.template,
          "--day-grid-min-w": `${smColumns.minWidthPx}px`,
          "--day-grid-min-w-mobile": `${mobileColumns.minWidthPx}px`,
        } as CSSProperties
      }
    >
      {planningDisabledDayCount > 0 && (
        <div className="border-b border-gray-200 bg-white p-2">
          <Alert
            type="info"
            title="Nicht planbar"
            message={
              weekDays.length === 1
                ? "Dieser Tag liegt außerhalb des Planungszeitraums."
                : "Die markierten Tage liegen außerhalb des Planungszeitraums."
            }
          />
        </div>
      )}

      {/* Mobile day strip — tap a day to switch (single-day view < sm).
          Bei genau einem übergebenen Tag entfällt er: ein Umschalter mit einer
          einzigen Option schaltet nichts, kostet aber eine fette Zeile direkt
          unter der Tagesauswahl der Kopfzeile (Vertretung, Tagesansicht). */}
      <div
        className={`gap-1 border-b border-gray-200 bg-white p-2 sm:hidden ${
          weekDays.length > 1 ? "flex" : "hidden"
        }`}
      >
        {weekDays.map((day, index) => {
          const iso = toISODate(day);
          const isToday = iso === todayISO;
          const isSelected = index === safeSelectedIndex;
          const closingReason = closingDays?.get(iso);
          const planningDisabled = planningDisabledDateISOs?.has(iso) ?? false;
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
                // Im Tagesstreifen ist kein Platz für Text: nur das Symbol,
                // Beschriftung und Grund bleiben im Tooltip.
                <ClosingDayChip reason={closingReason} text="" />
              )}
              {planningDisabled && (
                <span
                  className={`text-[9px] leading-none font-medium ${
                    isSelected ? "text-white/80" : "text-gray-500"
                  }`}
                >
                  Nicht planbar
                </span>
              )}
              {isToday && !isSelected && (
                <span
                  className="bg-moto-red h-1 w-1 rounded-full"
                  aria-hidden
                />
              )}
            </button>
          );
        })}
      </div>

      {/* Scrollable body. Tageskopfzeile und Kapazitätszeile liegen MIT im
          Scroll-Container (sticky), nicht darüber: sonst verliert nur der
          Körper die Breite der Scrollleiste und die Kopfspalten stehen um
          diese Differenz versetzt über den Tagesspalten — über fünf, sieben
          Spalten summiert sich das sichtbar auf. Die Maximalhöhe umfasst den
          Rasterkörper (720px) und die Sticky-Zeilen. Die optionale
          Kapazitätszeile bekommt ihr eigenes Höhenbudget, damit sie im
          Betreuungsplan nicht sichtbare Rasterhöhe wegnimmt. Waagrecht
          scrollt derselbe Container, wenn die Tage ihre Mindestbreite
          brauchen; der innere Rahmen trägt diese Breite für alle drei
          Zeilen, damit Kopf und Raster gemeinsam scrollen. */}
      <div
        ref={scrollRef}
        className={`relative overflow-auto overscroll-x-contain ${
          showDayHeader ? "max-h-[776px] sm:max-h-[808px]" : "max-h-[776px]"
        }`}
      >
        <div className="min-w-(--day-grid-min-w-mobile) sm:min-w-(--day-grid-min-w)">
          {/* Day header (desktop — mobile uses the day strip above). Die
            Kopfzeile behält ihre feste Höhe, weil die Kapazitätszeile mit
            `top-14` darunter klebt: Wochentag und Datum stehen deshalb an
            einem Schließtag in einer eigenen Zeile, der Chip einzeilig
            darunter. Der vollständige Grund steht im Tooltip. */}
          <div
            className={`sticky top-0 z-30 hidden h-[52px] border-b border-gray-200 bg-white sm:grid sm:h-14 ${gridColsClass}`}
          >
            <div aria-hidden className="sticky left-0 z-10 bg-white" />
            {weekDays.map((day) => {
              const iso = toISODate(day);
              const isToday = iso === todayISO;
              const closingReason = closingDays?.get(iso);
              const planningDisabled =
                planningDisabledDateISOs?.has(iso) ?? false;
              return (
                <div
                  key={iso}
                  className={`flex min-w-0 flex-col items-center justify-center gap-0.5 border-l border-gray-200 px-1 py-1 sm:px-2 ${
                    closingReason === undefined && !planningDisabled
                      ? ""
                      : "bg-gray-50"
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
                  {/* Der Schließtag-Grund („Ferien") schlägt das generische
                    „Nicht planbar": ein Schließtag ist ohnehin nicht planbar,
                    und der konkrete Grund darf nicht verschwinden. */}
                  {closingReason !== undefined ? (
                    <ClosingDayChip
                      reason={closingReason}
                      className="w-full justify-center text-center"
                    />
                  ) : planningDisabled ? (
                    <span className="text-[10px] font-medium text-gray-500">
                      Nicht planbar
                    </span>
                  ) : null}
                </div>
              );
            })}
          </div>

          {/* Day-header capacity strip (Betreuungsplan opt-in) — sm+ only, matches the desktop day-header columns;
            on mobile the day strip above already shows one day at a time.
            Klebt unter der Tageskopfzeile (sm:h-14 = 56px). */}
          {showDayHeader && (
            <div className="sticky top-14 z-20 hidden bg-white sm:block">
              <CapacityStrip
                as="div"
                position="header"
                labelWidthClassName="w-[64px]"
                rowLabel=""
                gridTemplateColumns={smColumns.template}
                cells={dayHeaderCells}
              />
            </div>
          )}

          <div className={`grid ${gridColsClass}`}>
            {/* Time gutter — klebt links, wenn das Raster waagrecht scrollt. */}
            <div
              data-time-gutter
              className="sticky left-0 z-20 border-r border-gray-200 bg-gray-50"
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
              const planningDisabled =
                planningDisabledDateISOs?.has(iso) ?? false;
              const laned = lanedByDay[dayIndex] ?? [];
              const nowOffset = isToday
                ? getCurrentTimeOffset(
                    hourHeightPx,
                    renderStartHour,
                    renderEndHour,
                  )
                : null;
              let backgroundClass = "";
              if (closingReason !== undefined || planningDisabled) {
                backgroundClass = "bg-gray-100/70";
              } else if (isToday) {
                backgroundClass = "bg-gray-50/60";
              }

              return (
                <div
                  key={iso}
                  data-today-column={isToday ? "" : undefined}
                  className={`relative min-w-0 border-l border-gray-200 ${backgroundClass} ${dayIndex === safeSelectedIndex ? "" : "hidden sm:block"}`}
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
                      (hour - renderStartHour) * hourHeightPx +
                      hourHeightPx / 2;
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
                    !planningDisabled &&
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
                        className="group absolute inset-x-0 flex items-center justify-center p-0.5 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset"
                        style={{
                          top: `${(hour - renderStartHour) * hourHeightPx}px`,
                          height: `${hourHeightPx}px`,
                        }}
                      >
                        {/* Dieselbe Anlege-Geste wie in den Zellen von Dienstplan
                          und ResourceGrid (#2031) statt der früheren Textpille
                          "+ Termin". */}
                        <PlanAddAffordance />
                      </button>
                    ))}

                  {/* Event blocks */}
                  {laned.map(({ instance, lane, laneCount, laneSpan }) => {
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
                        // Volle Spaltenbreite: der frühere 4px-Abzug ließ rechts
                        // einen Streifen der Spalte frei und der Block stand
                        // sichtbar neben seiner eigenen Spaltenkante. Freie
                        // Spuren rechts davon nimmt der Block mit.
                        width={`${laneSpan * widthPct}%`}
                        isSelected={selectedId === instance.id}
                        isGap={gapInstanceIds?.has(instance.id) ?? false}
                        staffNames={staffNames}
                        onClick={() => onInstanceClick(instance)}
                      />
                    );
                  })}

                  {/* Now line (today only) */}
                  {nowOffset !== null && (
                    <>
                      <div
                        className="bg-moto-red pointer-events-none absolute -left-1 z-10 h-2 w-2 rounded-full"
                        style={{ top: `${nowOffset - 4}px` }}
                      />
                      <div
                        className="border-moto-red pointer-events-none absolute right-0 left-0 z-10 border-t-2"
                        style={{ top: `${nowOffset}px` }}
                      />
                    </>
                  )}

                  {/* Kein "Keine Aktivitäten" pro leerer Tagesspalte mehr
                    (#2031): die leere Spalte sagt es selbst, die Spaltenkopfzeile
                    zeigt zusätzlich die Personenzahl, und beim Hovern erscheint
                    die Anlege-Fläche. Der Leerzustand für die KOMPLETT leere
                    Woche (`emptyState`) bleibt.

                    Der Schließtag-Hinweis (#2032) bleibt dagegen stehen: er
                    trägt einen Grund, den die leere Spalte nicht selbst
                    zeigt. */}
                  {closingReason !== undefined &&
                    dayInstances.length === 0 &&
                    !emptyState && (
                      <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-1">
                        <ClosingDayChip reason={closingReason} />
                      </div>
                    )}
                </div>
              );
            })}
          </div>
        </div>

        {emptyState && (
          <div className="pointer-events-none absolute inset-x-0 top-1/2 z-20 flex -translate-y-1/2 justify-center px-4">
            <div className="moto-content-surface max-w-sm rounded-2xl border px-4 py-1 shadow-sm">
              <EmptyState
                variant="compact"
                title={emptyState.title}
                description={emptyState.description}
              />
            </div>
          </div>
        )}
      </div>

      {legend ? (
        <div className="border-t border-gray-200 bg-white px-3 py-2">
          {legend}
        </div>
      ) : null}
    </div>
  );
}
