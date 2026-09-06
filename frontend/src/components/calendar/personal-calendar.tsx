"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { Check, Pencil, Plus, Trash2, X } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { PlanLegend, type PlanLegendEntry } from "~/components/ui/plan-legend";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Skeleton } from "~/components/ui/skeleton";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type {
  CalendarAppointmentOverview,
  CalendarEvent,
  CalendarResponseStatus,
} from "~/lib/personal-calendar-api";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import {
  formatDayHeader,
  formatWeekLabel,
  getWeekRange,
  getWeekdays,
} from "~/lib/timetable-helpers";
import { LOCATION_COLORS, MOTO_COLOR_PALETTE } from "~/lib/location-helper";

export type CalendarViewMode = "day" | "week" | "month";

interface PersonalCalendarProps {
  /**
   * Wochenenden anzeigen. Den Schalter dazu trägt `PersonalCalendarChrome`
   * in der Kopfkarte der Seite, deshalb kommt der Wert von außen.
   */
  readonly showWeekend?: boolean;
  readonly events: readonly CalendarEvent[];
  readonly referenceDate?: Date;
  readonly weekStart?: Date;
  readonly viewMode?: CalendarViewMode;
  readonly loading?: boolean;
  readonly error?: string | null;
  readonly onShowOverview?: (appointmentId: string) => void;
  readonly onRespond?: (
    recipientId: string,
    status: "accepted" | "declined",
  ) => void;
  readonly respondingRecipientId?: string | null;
  // Organizer-only management actions. Passed by the staff calendar page and
  // omitted by the parents portal, so parents never see edit/cancel/delete.
  readonly onEdit?: (event: CalendarEvent) => void;
  readonly onCancel?: (event: CalendarEvent) => void;
  readonly onDelete?: (event: CalendarEvent) => void;
  readonly busyAppointmentId?: string | null;
  // Base path for the .ics download route, e.g. "/api/parent/calendar/appointments".
  // When set, appointment cards show a "Zum Kalender hinzufügen" download link.
  readonly icsHrefBase?: string;
}

interface CalendarEventActions {
  // Opens the event detail sheet. Every event surface (agenda row, all-day
  // pill, month pill, time-grid block) is a button that calls this instead of
  // rendering inline actions — Apple-Calendar style: tap the event, act in the
  // detail. Set internally by PersonalCalendar.
  readonly onSelect?: (event: CalendarEvent) => void;
  readonly onShowOverview?: (appointmentId: string) => void;
  readonly onRespond?: (
    recipientId: string,
    status: "accepted" | "declined",
  ) => void;
  readonly respondingRecipientId?: string | null;
  readonly onEdit?: (event: CalendarEvent) => void;
  readonly onCancel?: (event: CalendarEvent) => void;
  readonly onDelete?: (event: CalendarEvent) => void;
  readonly busyAppointmentId?: string | null;
  readonly icsHrefBase?: string;
}

const sourceTone = {
  appointment: {
    label: "Termin",
    bar: LOCATION_COLORS.GROUP_ROOM,
    bg: MOTO_COLOR_PALETTE.green.soft,
  },
  timetable: {
    label: "Betreuung",
    bar: LOCATION_COLORS.OTHER_ROOM,
    bg: MOTO_COLOR_PALETTE.blue.soft,
  },
  shift: {
    label: "Dienst",
    bar: LOCATION_COLORS.SCHOOLYARD,
    bg: MOTO_COLOR_PALETTE.orange.soft,
  },
} satisfies Record<
  CalendarEvent["source"],
  { label: string; bar: string; bg: string }
>;

/**
 * Der Kalender codiert die Herkunft eines Termins farbig. Bauart 3 Regel 3:
 * jede farbcodierte Fläche trägt eine Legende.
 */
const calendarLegendEntries: readonly PlanLegendEntry[] = [
  {
    key: "source-appointment",
    label: sourceTone.appointment.label,
    color: sourceTone.appointment.bar,
  },
  {
    key: "source-timetable",
    label: sourceTone.timetable.label,
    color: sourceTone.timetable.bar,
  },
  {
    key: "source-shift",
    label: sourceTone.shift.label,
    color: sourceTone.shift.bar,
  },
];

const responseLabel: Record<string, string> = {
  pending: "Offen",
  accepted: "Zugesagt",
  declined: "Abgelehnt",
  info: "Info",
};

const responseTone: Record<CalendarResponseStatus, string> = {
  pending: "bg-gray-100 text-gray-700",
  accepted: "bg-moto-green-soft text-gray-800",
  declined: "bg-moto-red/10 text-moto-red-strong",
  info: "bg-moto-blue-soft text-gray-800",
};

function eventTime(event: CalendarEvent): string {
  if (event.all_day) return "Ganztägig";
  return `${event.start_time}–${event.end_time}`;
}

function eventSortValue(event: CalendarEvent): string {
  return `${event.start_date} ${event.start_time} ${event.title}`;
}

function eventsForDay(
  events: readonly CalendarEvent[],
  day: Date,
): CalendarEvent[] {
  const iso = toISODate(day);
  return events
    .filter((event) => event.start_date <= iso && event.end_date >= iso)
    .sort((a, b) => eventSortValue(a).localeCompare(eventSortValue(b)));
}

const HOUR_PX = 64;
const DEFAULT_GRID_START_HOUR = 8;
const DEFAULT_GRID_END_HOUR = 17;

function clockToMinutes(clock: string): number {
  const [hours = 0, minutes = 0] = clock.split(":").map(Number);
  return hours * 60 + minutes;
}

function isWeekendDay(day: Date): boolean {
  const weekday = day.getDay();
  return weekday === 0 || weekday === 6;
}

// Ganztägige und mehrtägige Einträge haben keine sinnvolle Position auf der
// Zeitachse — sie wandern in die Ganztägig-Zeile über dem Raster.
function isAllDayLike(event: CalendarEvent): boolean {
  return event.all_day || event.start_date !== event.end_date;
}

// Ein Eintrag verschwindet mit ausgeblendetem Wochenende nur, wenn seine
// gesamte Laufzeit auf Sa/So liegt — ein Fr–Sa-Termin bleibt sichtbar.
function isWeekendOnlyEvent(event: CalendarEvent): boolean {
  const start = parseISODate(event.start_date);
  const end = parseISODate(event.end_date);
  const spanDays = Math.min(
    62,
    Math.round((end.getTime() - start.getTime()) / 86_400_000) + 1,
  );
  for (let offset = 0; offset < spanDays; offset += 1) {
    const day = new Date(start);
    day.setDate(start.getDate() + offset);
    if (!isWeekendDay(day)) return false;
  }
  return true;
}

/**
 * Einträge, die bei der gewählten Wochenend-Anzeige im Raster vorkommen.
 * Die Seitenkopfkarte nutzt dieselbe Regel für Zahl und Leerzustand, damit
 * ein nur am Wochenende liegender Zeitraum nicht als gefüllter Kalender
 * ohne sichtbaren Inhalt erscheint.
 */
export function visibleCalendarEvents(
  events: readonly CalendarEvent[],
  showWeekend: boolean,
  viewMode: CalendarViewMode,
): readonly CalendarEvent[] {
  return showWeekend || viewMode === "day"
    ? events
    : events.filter((event) => !isWeekendOnlyEvent(event));
}

function countWeekendEvents(
  events: readonly CalendarEvent[],
  weekendDays: readonly Date[],
): number {
  const seen = new Set<string>();
  for (const day of weekendDays) {
    for (const event of eventsForDay(events, day)) {
      seen.add(event.id);
    }
  }
  return seen.size;
}

function gridHours(dayEvents: readonly CalendarEvent[]): {
  startHour: number;
  endHour: number;
} {
  let startHour = DEFAULT_GRID_START_HOUR;
  let endHour = DEFAULT_GRID_END_HOUR;
  for (const event of dayEvents) {
    if (isAllDayLike(event)) continue;
    const startMinutes = clockToMinutes(event.start_time);
    startHour = Math.min(startHour, Math.floor(startMinutes / 60));
    endHour = Math.max(endHour, Math.ceil(effectiveEndMinutes(event) / 60));
  }
  startHour = Math.max(0, startHour);
  endHour = Math.min(24, Math.max(endHour, startHour + 1));
  return { startHour, endHour };
}

interface TimedPlacement {
  event: CalendarEvent;
  startMinutes: number;
  // Effektives Render-Ende: das spätere von tatsächlichem Ende und der
  // Mindesthöhe des Karteninhalts. Layout UND Kartenhöhe rechnen mit diesem
  // Wert, damit optisch verlängerte Karten nachfolgende nicht verdecken.
  endMinutes: number;
  column: number;
  columnCount: number;
}

// Mindesthöhe eines Zeitraster-Blocks in Pixeln, abhängig vom Inhalt. Blöcke
// zeigen nur noch Titel, Zeit und optional Ort/Zuordnung — Aktionen leben im
// Detail-Sheet (Klick). Der Wert muss zur tatsächlich gerenderten Box in
// TimeGridEventBlock passen: 12px vertikales Padding (py-1.5) + 2px Rahmen +
// eine Titelzeile (~16px) + eine Zeitzeile (~16px), plus je ~16px für Ort und
// die Zuordnungs-Unterzeile. Der Block ist overflow-hidden — eine zu kleine
// Höhe schneidet bei kurzen Terminen Zeit/Metadaten sichtbar ab.
function blockMinHeightPx(event: CalendarEvent): number {
  return (
    54 +
    (event.location ? 16 : 0) +
    ((event.student_name ?? event.school_name) ? 16 : 0)
  );
}

// Effektives Render-Ende eines Eintrags in Minuten: das spätere von
// tatsächlichem Ende und der Mindesthöhe des gerenderten Inhalts. Rasterfenster
// und Layout rechnen beide damit, sonst laufen späte Kurztermine unten heraus.
function effectiveEndMinutes(event: CalendarEvent): number {
  const startMinutes = clockToMinutes(event.start_time);
  const minRenderMinutes =
    event.source === "shift" ? 30 : (blockMinHeightPx(event) / HOUR_PX) * 60;
  return Math.max(
    clockToMinutes(event.end_time),
    startMinutes + minRenderMinutes,
  );
}

// Klassisches Zeitraster-Layout: überlappende Einträge bilden ein Cluster und
// teilen sich die Spaltenbreite; jeder Eintrag bekommt die erste freie Spalte.
function layoutTimedEvents(
  dayEvents: readonly CalendarEvent[],
): TimedPlacement[] {
  const items: TimedPlacement[] = dayEvents
    .map((event) => {
      const startMinutes = clockToMinutes(event.start_time);
      return {
        event,
        startMinutes,
        endMinutes: effectiveEndMinutes(event),
        column: 0,
        columnCount: 1,
      };
    })
    .sort(
      (a, b) => a.startMinutes - b.startMinutes || b.endMinutes - a.endMinutes,
    );

  const placed: TimedPlacement[] = [];
  let cluster: TimedPlacement[] = [];
  let columnEnds: number[] = [];
  let clusterEnd = -1;

  const flushCluster = () => {
    for (const item of cluster) {
      item.columnCount = columnEnds.length;
    }
    placed.push(...cluster);
    cluster = [];
    columnEnds = [];
    clusterEnd = -1;
  };

  for (const item of items) {
    if (clusterEnd >= 0 && item.startMinutes >= clusterEnd) flushCluster();
    let column = columnEnds.findIndex((end) => end <= item.startMinutes);
    if (column === -1) {
      column = columnEnds.length;
      columnEnds.push(item.endMinutes);
    } else {
      columnEnds[column] = item.endMinutes;
    }
    item.column = column;
    cluster.push(item);
    clusterEnd = Math.max(clusterEnd, item.endMinutes);
  }
  flushCluster();
  return placed;
}

function shiftDate(
  date: Date,
  viewMode: CalendarViewMode,
  offset: number,
): Date {
  const next = new Date(date);
  if (viewMode === "day") {
    next.setDate(next.getDate() + offset);
    return next;
  }
  if (viewMode === "month") {
    next.setMonth(next.getMonth() + offset, 1);
    return next;
  }
  next.setDate(next.getDate() + offset * 7);
  return getWeekRange(next).from;
}

function monthGridDays(referenceDate: Date): Date[] {
  const firstOfMonth = new Date(
    referenceDate.getFullYear(),
    referenceDate.getMonth(),
    1,
  );
  const start = getWeekRange(firstOfMonth).from;
  const days: Date[] = [];
  for (let index = 0; index < 42; index += 1) {
    const day = new Date(start);
    day.setDate(start.getDate() + index);
    days.push(day);
  }
  return days;
}

function periodLabel(
  referenceDate: Date,
  viewMode: CalendarViewMode,
  from: Date,
  to: Date,
): string {
  if (viewMode === "day") {
    return referenceDate.toLocaleDateString("de-DE", {
      timeZone: "Europe/Berlin",
      weekday: "long",
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    });
  }
  if (viewMode === "month") {
    return referenceDate.toLocaleDateString("de-DE", {
      timeZone: "Europe/Berlin",
      month: "long",
      year: "numeric",
    });
  }
  return formatWeekLabel(from, to);
}

const viewOptions = [
  { mode: "day", label: "Tag" },
  { mode: "week", label: "Woche" },
  { mode: "month", label: "Monat" },
] satisfies Array<{ mode: CalendarViewMode; label: string }>;

/**
 * Text des Leerzustands. Die Kalenderseite belegt damit `empty` der
 * `TenantPage`, das Raster selbst nutzt ihn als Rückfall.
 */
export function calendarEmptyLabel(viewMode: CalendarViewMode): string {
  if (viewMode === "day") return "Keine Einträge an diesem Tag.";
  if (viewMode === "month") return "Keine Einträge in diesem Monat.";
  return "Keine Einträge in dieser Woche.";
}

/** Zeigt die Fläche bereits den heutigen Tag? Dann bleibt "Heute" inaktiv. */
function showsToday(referenceDate: Date, viewMode: CalendarViewMode): boolean {
  const today = new Date();
  if (viewMode === "month") {
    return (
      referenceDate.getFullYear() === today.getFullYear() &&
      referenceDate.getMonth() === today.getMonth()
    );
  }
  if (viewMode === "day") return toISODate(referenceDate) === toISODate(today);
  const { from, to } = getWeekRange(referenceDate);
  const todayISO = toISODate(today);
  return todayISO >= toISODate(from) && todayISO <= toISODate(to);
}

function previousLabel(viewMode: CalendarViewMode): string {
  if (viewMode === "day") return "Vorheriger Tag";
  if (viewMode === "month") return "Vorheriger Monat";
  return "Vorherige Woche";
}

function nextLabel(viewMode: CalendarViewMode): string {
  if (viewMode === "day") return "Nächster Tag";
  if (viewMode === "month") return "Nächster Monat";
  return "Nächste Woche";
}

export function PersonalCalendar({
  showWeekend = false,
  events,
  referenceDate: rawReferenceDate,
  weekStart,
  viewMode = "week",
  loading,
  error,
  onShowOverview,
  onRespond,
  respondingRecipientId,
  onEdit,
  onCancel,
  onDelete,
  busyAppointmentId,
  icsHrefBase,
}: PersonalCalendarProps) {
  const [selectedEvent, setSelectedEvent] = useState<CalendarEvent | null>(
    null,
  );
  const actions: CalendarEventActions = {
    onSelect: setSelectedEvent,
    onShowOverview,
    onRespond,
    respondingRecipientId,
    onEdit,
    onCancel,
    onDelete,
    busyAppointmentId,
    icsHrefBase,
  };
  const referenceDate = rawReferenceDate ?? weekStart ?? new Date();
  const { from } = getWeekRange(referenceDate);
  const days = getWeekdays(from);
  const monthDays = monthGridDays(referenceDate);
  const sortedEvents = [...events].sort((a, b) =>
    eventSortValue(a).localeCompare(eventSortValue(b)),
  );
  const visibleWeekDays = showWeekend
    ? days
    : days.filter((day) => !isWeekendDay(day));
  const visibleMonthDays = showWeekend
    ? monthDays
    : monthDays.filter((day) => !isWeekendDay(day));
  const visibleSortedEvents = visibleCalendarEvents(
    sortedEvents,
    showWeekend,
    viewMode,
  );
  // Mobile lacks the space for the time grid, so it renders a per-day agenda.
  // The day set mirrors the desktop view (single day / visible week / visible
  // month) so the weekend toggle and range stay consistent across breakpoints.
  const mobileDays =
    viewMode === "day"
      ? [referenceDate]
      : viewMode === "month"
        ? visibleMonthDays
        : visibleWeekDays;

  // Die Legende sitzt als Fussband IN der jeweiligen Rasterflaeche. Frei
  // darunter stuende sie auf dem gemusterten Grund (BAUARTEN-SPEC Teil 3).
  const legendBand =
    visibleSortedEvents.length > 0 ? (
      <div className="border-t border-gray-200 bg-white px-3 py-2">
        <PlanLegend
          entries={calendarLegendEntries}
          aria-label="Legende Terminarten"
        />
      </div>
    ) : null;

  return (
    <div className="w-full space-y-6">
      {error ? <Alert type="error" message={error} /> : null}

      <div className="relative">
        {/* Der Kopf bleibt beim Laden stehen, nur die Datenfläche wird
            abgedeckt; ein Skelett darüber statt eines Eigenbau-Spinners. */}
        {loading ? (
          <div
            role="status"
            aria-busy="true"
            aria-label="Kalender wird geladen"
            className="absolute inset-0 z-10 space-y-2 bg-white/80 p-4"
          >
            <Skeleton className="h-10 w-full rounded-lg" />
            <Skeleton className="h-full min-h-40 w-full rounded-lg" />
          </div>
        ) : null}
        {viewMode === "day" ? (
          <div className="moto-content-surface hidden overflow-hidden rounded-2xl border shadow-sm lg:block">
            <CalendarTimeGrid
              days={[referenceDate]}
              events={events}
              actions={actions}
            />
            {legendBand}
          </div>
        ) : null}

        {viewMode === "week" ? (
          <div className="moto-content-surface hidden overflow-hidden rounded-2xl border shadow-sm lg:block">
            <CalendarTimeGrid
              days={visibleWeekDays}
              events={events}
              actions={actions}
            />
            {legendBand}
          </div>
        ) : null}

        {viewMode === "month" ? (
          <div className="moto-content-surface hidden overflow-hidden rounded-2xl border shadow-sm lg:block">
            <div
              className={`grid ${showWeekend ? "grid-cols-7" : "grid-cols-5"}`}
            >
              {visibleMonthDays.map((day) => {
                const inMonth = day.getMonth() === referenceDate.getMonth();
                return (
                  <CalendarDayColumn
                    key={toISODate(day)}
                    day={day}
                    events={eventsForDay(events, day)}
                    actions={actions}
                    compact
                    muted={!inMonth}
                    className="min-h-44 border-r border-b border-gray-200 last:border-r-0"
                  />
                );
              })}
            </div>
            {legendBand}
          </div>
        ) : null}

        <div className="lg:hidden">
          <MobileAgenda
            days={mobileDays}
            events={events}
            viewMode={viewMode}
            actions={actions}
            legend={legendBand}
          />
        </div>

        {!loading && visibleSortedEvents.length === 0 ? (
          <div className="hidden lg:block">
            <EmptyCalendarState viewMode={viewMode} />
          </div>
        ) : null}
      </div>

      <SlideOver
        open={selectedEvent !== null}
        onOpenChange={(open) => {
          if (!open) setSelectedEvent(null);
        }}
      >
        <SlideOverContent widthClass="sm:w-[520px]">
          <SlideOverHeader className="flex-row items-start justify-between gap-3">
            <div className="min-w-0">
              <SlideOverTitle>Termin</SlideOverTitle>
            </div>
            <SlideOverCloseButton />
          </SlideOverHeader>
          <div className="flex-1 overflow-y-auto px-5 py-4">
            {selectedEvent ? (
              <CalendarEventDetail
                event={selectedEvent}
                actions={actions}
                onClose={() => setSelectedEvent(null)}
              />
            ) : null}
          </div>
        </SlideOverContent>
      </SlideOver>
    </div>
  );
}

// Mobile replacement for the desktop time grid: a per-day agenda. Events are
// grouped under a sticky day header (weekday + date + count) so a full week no
// longer collapses into one undifferentiated list where Monday can't be told
// from Friday. Days without events are dropped entirely.
function MobileAgenda({
  days,
  events,
  viewMode,
  actions,
  legend,
}: Readonly<{
  days: readonly Date[];
  events: readonly CalendarEvent[];
  viewMode: CalendarViewMode;
  actions: CalendarEventActions;
  legend?: ReactNode;
}>) {
  const groups = days
    .map((day) => {
      const dayEvents = eventsForDay(events, day);
      return {
        day,
        allDay: dayEvents.filter(isAllDayLike),
        timed: dayEvents.filter((event) => !isAllDayLike(event)),
        count: dayEvents.length,
      };
    })
    .filter((group) => group.count > 0);

  if (groups.length === 0) {
    return <EmptyCalendarState viewMode={viewMode} />;
  }

  return (
    <div className="moto-content-surface divide-y divide-gray-100 overflow-hidden rounded-2xl border shadow-sm">
      {groups.map(({ day, allDay, timed, count }) => (
        <section key={toISODate(day)} className="divide-y divide-gray-100">
          <div className="flex items-baseline justify-between gap-2 bg-gray-50 px-4 py-2">
            <h2 className="text-sm font-semibold text-gray-900">
              {formatDayHeader(day)}
            </h2>
            <span className="shrink-0 text-xs text-gray-500">
              {count === 1 ? "1 Eintrag" : `${count} Einträge`}
            </span>
          </div>
          {[...allDay, ...timed].map((event) => (
            <AgendaRow
              key={`${event.id}-${toISODate(day)}`}
              event={event}
              actions={actions}
            />
          ))}
        </section>
      ))}
      {legend}
    </div>
  );
}

function CalendarDayColumn({
  day,
  events,
  actions,
  compact = false,
  muted = false,
  className = "",
}: Readonly<{
  day: Date;
  events: readonly CalendarEvent[];
  actions: CalendarEventActions;
  compact?: boolean;
  muted?: boolean;
  className?: string;
}>) {
  return (
    <section className={className}>
      <div
        className={`border-b border-gray-200 px-3 py-2 ${
          muted ? "bg-gray-50/60 text-gray-400" : "bg-gray-50"
        }`}
      >
        <div className="text-sm font-semibold text-gray-900">
          {compact
            ? day.toLocaleDateString("de-DE", {
                timeZone: "Europe/Berlin",
                weekday: "short",
                day: "2-digit",
              })
            : formatDayHeader(day)}
        </div>
        <div className="text-xs text-gray-500">
          {events.length === 1 ? "1 Eintrag" : `${events.length} Einträge`}
        </div>
      </div>
      <div className={`space-y-1 ${compact ? "p-1.5" : "p-2"}`}>
        {events.map((event) => (
          <EventPill
            key={`${event.id}-${toISODate(day)}`}
            event={event}
            actions={actions}
          />
        ))}
      </div>
    </section>
  );
}

function CalendarTimeGrid({
  days,
  events,
  actions,
}: Readonly<{
  days: readonly Date[];
  events: readonly CalendarEvent[];
  actions: CalendarEventActions;
}>) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const dayBuckets = days.map((day) => ({
    day,
    events: eventsForDay(events, day),
  }));
  const { startHour, endHour } = gridHours(
    dayBuckets.flatMap((bucket) => bucket.events),
  );
  const gridStartMinutes = startHour * 60;
  const bodyHeight = (endHour - startHour) * HOUR_PX;
  const hours: number[] = [];
  for (let hour = startHour; hour <= endHour; hour += 1) {
    hours.push(hour);
  }
  const hasAllDay = dayBuckets.some((bucket) =>
    bucket.events.some(isAllDayLike),
  );
  const columnTemplate = {
    gridTemplateColumns: `repeat(${days.length}, minmax(0, 1fr))`,
  };

  useEffect(() => {
    const node = scrollRef.current;
    if (!node) return;
    node.scrollTop = Math.max(
      0,
      (DEFAULT_GRID_START_HOUR - startHour) * HOUR_PX - 8,
    );
  }, [startHour]);

  return (
    <div>
      <div className="flex border-b border-gray-200">
        <div className="w-16 shrink-0 border-r border-gray-200 bg-gray-50" />
        <div className="grid flex-1" style={columnTemplate}>
          {dayBuckets.map(({ day, events: dayEvents }) => (
            <div
              key={toISODate(day)}
              className="border-r border-gray-200 bg-gray-50 px-3 py-2 last:border-r-0"
            >
              <div className="text-sm font-semibold text-gray-900">
                {formatDayHeader(day)}
              </div>
              <div className="text-xs text-gray-500">
                {dayEvents.length === 1
                  ? "1 Eintrag"
                  : `${dayEvents.length} Einträge`}
              </div>
            </div>
          ))}
        </div>
      </div>
      {hasAllDay ? (
        <div className="flex border-b border-gray-200">
          <div className="w-16 shrink-0 border-r border-gray-200 px-1 py-2 text-center text-[11px] text-gray-400">
            Ganztägig
          </div>
          <div className="grid flex-1" style={columnTemplate}>
            {dayBuckets.map(({ day, events: dayEvents }) => (
              <div
                key={toISODate(day)}
                className="space-y-1 border-r border-gray-200 p-1 last:border-r-0"
              >
                {dayEvents.filter(isAllDayLike).map((event) => (
                  <EventPill
                    key={`${event.id}-${toISODate(day)}`}
                    event={event}
                    actions={actions}
                  />
                ))}
              </div>
            ))}
          </div>
        </div>
      ) : null}
      <div ref={scrollRef} className="max-h-[70vh] overflow-y-auto">
        <div className="flex" style={{ height: bodyHeight }}>
          <div className="relative w-16 shrink-0 border-r border-gray-200">
            {hours.map((hour) =>
              hour === startHour ? null : (
                <div
                  key={hour}
                  className="absolute inset-x-0 -translate-y-1/2 text-center text-[11px] text-gray-400"
                  style={{ top: (hour - startHour) * HOUR_PX }}
                >
                  {`${String(hour).padStart(2, "0")}:00`}
                </div>
              ),
            )}
          </div>
          <div className="grid flex-1" style={columnTemplate}>
            {dayBuckets.map(({ day, events: dayEvents }) => (
              <TimeGridDayBody
                key={toISODate(day)}
                events={dayEvents}
                hours={hours}
                startHour={startHour}
                gridStartMinutes={gridStartMinutes}
                bodyHeight={bodyHeight}
                actions={actions}
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function TimeGridDayBody({
  events,
  hours,
  startHour,
  gridStartMinutes,
  bodyHeight,
  actions,
}: Readonly<{
  events: readonly CalendarEvent[];
  hours: readonly number[];
  startHour: number;
  gridStartMinutes: number;
  bodyHeight: number;
  actions: CalendarEventActions;
}>) {
  const shiftBands = events.filter(
    (event) => event.source === "shift" && !isAllDayLike(event),
  );
  const timed = layoutTimedEvents(
    events.filter((event) => event.source !== "shift" && !isAllDayLike(event)),
  );
  return (
    <div
      className="relative border-r border-gray-200 last:border-r-0"
      style={{ height: bodyHeight }}
    >
      {hours.map((hour) =>
        hour === startHour ? null : (
          <div
            key={hour}
            className="absolute inset-x-0 border-t border-gray-100"
            style={{ top: (hour - startHour) * HOUR_PX }}
            aria-hidden
          />
        ),
      )}
      {/* Das gesamte Dienst-Band ist der klickbare Bereich (z-0, unter den
          Terminblöcken): Termine (z-10) gewinnen immer in ihrer eigenen
          Fläche, jeder sichtbare Band-Pixel öffnet den Dienst — so bleiben
          beide auch bei Überlappung bedienbar. */}
      {shiftBands.map((event) => {
        const tone = sourceTone[event.source];
        const startMinutes = clockToMinutes(event.start_time);
        const endMinutes = Math.max(
          clockToMinutes(event.end_time),
          startMinutes + 30,
        );
        return (
          <button
            key={event.id}
            type="button"
            onClick={() => actions.onSelect?.(event)}
            className="absolute inset-x-0.5 z-0 flex flex-col overflow-hidden rounded-md border px-1.5 py-1 text-left transition-[filter] hover:brightness-95"
            style={{
              top: ((startMinutes - gridStartMinutes) / 60) * HOUR_PX + 1,
              height: ((endMinutes - startMinutes) / 60) * HOUR_PX - 2,
              backgroundColor: tone.bg,
              borderColor: `${tone.bar}55`,
            }}
          >
            <div className="flex flex-wrap items-center gap-1 text-[11px] text-gray-700">
              <span className="font-semibold text-gray-900">{event.title}</span>
              <span>{eventTime(event)}</span>
            </div>
          </button>
        );
      })}
      {timed.map((placement) => (
        <TimeGridEventBlock
          key={placement.event.id}
          placement={placement}
          gridStartMinutes={gridStartMinutes}
          actions={actions}
        />
      ))}
    </div>
  );
}

function TimeGridEventBlock({
  placement,
  gridStartMinutes,
  actions,
}: Readonly<{
  placement: TimedPlacement;
  gridStartMinutes: number;
  actions: CalendarEventActions;
}>) {
  const { event, startMinutes, endMinutes, column, columnCount } = placement;
  const tone = sourceTone[event.source];
  const cancelled = event.cancelled === true;
  // endMinutes ist bereits das effektive Render-Ende aus layoutTimedEvents —
  // die Mindesthöhe steckt in der Platzierung, damit nichts überdeckt wird.
  const height = ((endMinutes - startMinutes) / 60) * HOUR_PX - 2;
  const widthPercent = 100 / columnCount;
  const subtitle = [event.student_name, event.school_name]
    .filter(Boolean)
    .join(" · ");
  return (
    <button
      type="button"
      onClick={() => actions.onSelect?.(event)}
      className={`absolute z-10 overflow-hidden rounded-md border border-gray-200 p-1.5 text-left shadow-sm transition-[filter] hover:brightness-95 ${
        cancelled ? "opacity-70" : ""
      }`}
      style={{
        top: ((startMinutes - gridStartMinutes) / 60) * HOUR_PX + 1,
        height,
        left: `calc(${column * widthPercent}% + 2px)`,
        width: `calc(${widthPercent}% - 4px)`,
        borderLeft: `3px solid ${tone.bar}`,
        backgroundColor: tone.bg,
      }}
    >
      <div
        className={`truncate text-xs font-semibold text-gray-950 ${
          cancelled ? "line-through" : ""
        }`}
      >
        {event.title}
      </div>
      <div className="truncate text-[11px] text-gray-600">
        {eventTime(event)}
      </div>
      {event.location ? (
        <div className="flex items-center gap-1 text-[11px] text-gray-500">
          <MotoConceptIcon concept="rooms" size={12} className="shrink-0" />
          <span className="truncate">{event.location}</span>
        </div>
      ) : null}
      {subtitle ? (
        <div className="truncate text-[11px] text-gray-500">{subtitle}</div>
      ) : null}
    </button>
  );
}

// Apple-Kalender-artige Agenda-Zeile: linksbündige Zeitspalte, farbige Leiste,
// Titel + kompakte Unterzeile. Wird als Zeile INNERHALB des durchgehenden
// Tages-Panels gerendert (kein eigenes Karten-Chrome). Tap öffnet das Detail.
function AgendaRow({
  event,
  actions,
}: Readonly<{ event: CalendarEvent; actions: CalendarEventActions }>) {
  const tone = sourceTone[event.source];
  const cancelled = event.cancelled === true;
  // Präsentation hängt an event.all_day, nicht an der Datumsspanne: ein
  // mehrtägiger Termin MIT Uhrzeiten zeigt seine Zeiten, nur echte
  // Ganztägig-Einträge zeigen „ganztg.“ (isAllDayLike regelt nur die
  // Positionierung, nicht die Darstellung).
  const allDay = event.all_day;
  const badge = cancelled
    ? { label: "Abgesagt", cls: "bg-moto-red/10 text-moto-red-strong" }
    : event.response_status
      ? {
          label: responseLabel[event.response_status] ?? event.response_status,
          cls: responseTone[event.response_status],
        }
      : null;
  const subtitle = [
    tone.label,
    event.location,
    event.student_name,
    event.school_name,
  ]
    .filter(Boolean)
    .join(" · ");
  return (
    <button
      type="button"
      onClick={() => actions.onSelect?.(event)}
      className={`flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-gray-50 ${
        cancelled ? "opacity-70" : ""
      }`}
    >
      <div className="flex w-12 shrink-0 flex-col items-start leading-tight tabular-nums">
        {allDay ? (
          <span className="text-[11px] font-medium text-gray-400">ganztg.</span>
        ) : (
          <>
            <span className="text-sm font-semibold text-gray-900">
              {event.start_time}
            </span>
            <span className="text-[11px] text-gray-400">{event.end_time}</span>
          </>
        )}
      </div>
      <span
        className="h-9 w-1 shrink-0 rounded-full"
        style={{ backgroundColor: tone.bar }}
        aria-hidden
      />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span
            className={`truncate text-sm font-semibold text-gray-950 ${
              cancelled ? "line-through" : ""
            }`}
          >
            {event.title}
          </span>
          {badge ? (
            <span
              className={`shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold ${badge.cls}`}
            >
              {badge.label}
            </span>
          ) : null}
        </div>
        {subtitle ? (
          <div className="mt-0.5 truncate text-xs text-gray-500">
            {subtitle}
          </div>
        ) : null}
      </div>
    </button>
  );
}

// Schmale einzeilige Pille für die Ganztägig-Zeile im Wochen-/Tagesraster und
// für Monatszellen. Mehrtägige Einträge liegen aus Layout-Gründen ebenfalls in
// dieser Zeile; zeitgebundene Einträge behalten dort aber ihre Zeitangabe.
// Tap öffnet das Detail-Sheet.
function EventPill({
  event,
  actions,
}: Readonly<{ event: CalendarEvent; actions: CalendarEventActions }>) {
  const tone = sourceTone[event.source];
  const cancelled = event.cancelled === true;
  const timeLabel = event.all_day
    ? null
    : event.start_date === event.end_date
      ? event.start_time
      : `${event.start_time}–${event.end_time}`;
  return (
    <button
      type="button"
      onClick={() => actions.onSelect?.(event)}
      className={`flex w-full items-center gap-1.5 rounded-md px-2 py-1 text-left transition-[filter] hover:brightness-95 ${
        cancelled ? "opacity-70" : ""
      }`}
      style={{ backgroundColor: tone.bg }}
    >
      <span
        className="h-2 w-2 shrink-0 rounded-full"
        style={{ backgroundColor: tone.bar }}
        aria-hidden
      />
      <span
        className={`min-w-0 flex-1 truncate text-xs font-medium text-gray-900 ${
          cancelled ? "line-through" : ""
        }`}
      >
        {event.title}
      </span>
      {timeLabel ? (
        <span className="shrink-0 text-[11px] text-gray-500 tabular-nums">
          {timeLabel}
        </span>
      ) : null}
    </button>
  );
}

// Detail-Sheet, geöffnet beim Antippen eines Termins. Trägt alle Aktionen, die
// früher inline auf jeder Karte lagen (Antworten, Teilnehmer, Export, Verwalten)
// — so bleiben die Kalenderflächen selbst schlank und lesbar.
function CalendarEventDetail({
  event,
  actions,
  onClose,
}: Readonly<{
  event: CalendarEvent;
  actions: CalendarEventActions;
  onClose: () => void;
}>) {
  const {
    onShowOverview,
    onRespond,
    respondingRecipientId,
    onEdit,
    onCancel,
    onDelete,
    busyAppointmentId,
    icsHrefBase,
  } = actions;
  const tone = sourceTone[event.source];
  const recipientId = event.recipient_id;
  const cancelled = event.cancelled === true;
  const responding =
    Boolean(recipientId) && respondingRecipientId === recipientId;
  const managing =
    Boolean(event.appointment_id) && busyAppointmentId === event.appointment_id;
  const canManage =
    event.source === "appointment" &&
    event.can_edit &&
    Boolean(event.appointment_id) &&
    Boolean(onEdit ?? onCancel ?? onDelete);
  const showOverview = Boolean(
    event.can_view_overview && event.appointment_id && onShowOverview,
  );
  const showRespond = Boolean(
    !cancelled && event.can_respond && recipientId && onRespond,
  );
  const showIcs = Boolean(
    !cancelled &&
    event.source === "appointment" &&
    event.appointment_id &&
    icsHrefBase,
  );
  const startDate = parseISODate(event.start_date);
  const endDate = parseISODate(event.end_date);
  const dateLabel =
    event.start_date !== event.end_date
      ? `${startDate.toLocaleDateString("de-DE", {
          day: "2-digit",
          month: "long",
        })} – ${endDate.toLocaleDateString("de-DE", {
          day: "2-digit",
          month: "long",
          year: "numeric",
        })}`
      : startDate.toLocaleDateString("de-DE", {
          weekday: "long",
          day: "2-digit",
          month: "long",
          year: "numeric",
        });
  const subtitle = [event.student_name, event.school_name]
    .filter(Boolean)
    .join(" · ");
  const hasActions = showOverview || showRespond || showIcs || canManage;
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-1.5">
        <span
          className="rounded-md px-2 py-0.5 text-[11px] font-semibold text-gray-700"
          style={{ backgroundColor: tone.bg }}
        >
          {tone.label}
        </span>
        {cancelled ? (
          <span className="bg-moto-red/10 text-moto-red-strong rounded-md px-2 py-0.5 text-[11px] font-semibold">
            Abgesagt
          </span>
        ) : event.response_status ? (
          <span
            className={`rounded-md px-2 py-0.5 text-[11px] font-semibold ${
              responseTone[event.response_status]
            }`}
          >
            {responseLabel[event.response_status] ?? event.response_status}
          </span>
        ) : null}
      </div>

      <h3
        className={`text-lg font-semibold text-gray-950 ${
          cancelled ? "line-through" : ""
        }`}
      >
        {event.title}
      </h3>

      <div className="space-y-2 text-sm text-gray-700">
        <div className="flex items-center gap-2">
          <MotoConceptIcon concept="calendar" size={16} className="shrink-0" />
          <span>{dateLabel}</span>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
          <div className="flex items-center gap-2">
            <MotoConceptIcon
              concept="careTimes"
              size={16}
              className="shrink-0"
            />
            <span>{eventTime(event)}</span>
          </div>
          {event.location ? (
            <div className="flex items-center gap-2">
              <MotoConceptIcon concept="rooms" size={16} className="shrink-0" />
              <span>{event.location}</span>
            </div>
          ) : null}
        </div>
        {subtitle ? (
          <div className="flex items-center gap-2">
            <MotoConceptIcon
              concept="children"
              size={16}
              className="shrink-0"
            />
            <span>{subtitle}</span>
          </div>
        ) : null}
      </div>

      {event.description ? (
        <p className="text-sm leading-6 whitespace-pre-line text-gray-700">
          {event.description}
        </p>
      ) : null}

      {hasActions ? (
        <div className="flex flex-col gap-2 border-t border-gray-200 pt-4">
          {showRespond ? (
            <div className="grid grid-cols-2 gap-2">
              <Button
                type="button"
                variant="outline"
                size="md"
                className="gap-2"
                disabled={responding}
                onClick={() => {
                  onRespond!(recipientId!, "accepted");
                  onClose();
                }}
              >
                <Check className="h-4 w-4" aria-hidden />
                Zusagen
              </Button>
              <Button
                type="button"
                variant="outline_danger"
                size="md"
                className="gap-2"
                disabled={responding}
                onClick={() => {
                  onRespond!(recipientId!, "declined");
                  onClose();
                }}
              >
                <X className="h-4 w-4" aria-hidden />
                Absagen
              </Button>
            </div>
          ) : null}
          {showIcs ? (
            <a
              href={`${icsHrefBase}/${encodeURIComponent(
                event.appointment_id!,
              )}/ics`}
              download
              className="inline-flex h-10 w-full items-center justify-center gap-1.5 rounded-md bg-gray-100 px-4 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200"
            >
              <MotoConceptIcon concept="calendarPeriods" size={16} />
              Zum Kalender hinzufügen
            </a>
          ) : null}
          {showOverview ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              className="gap-2"
              onClick={() => {
                onShowOverview!(event.appointment_id!);
                onClose();
              }}
            >
              <MotoConceptIcon concept="people" size={16} />
              Teilnehmer
            </Button>
          ) : null}
          {canManage ? (
            <div className="flex flex-wrap gap-2">
              {onEdit && !cancelled ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="md"
                  className="gap-2"
                  disabled={managing}
                  onClick={() => {
                    onEdit(event);
                    onClose();
                  }}
                >
                  <Pencil className="h-4 w-4" aria-hidden />
                  Bearbeiten
                </Button>
              ) : null}
              {onCancel && !cancelled ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="md"
                  className="text-moto-red-strong gap-2"
                  disabled={managing}
                  onClick={() => {
                    onCancel(event);
                    onClose();
                  }}
                >
                  <MotoConceptIcon concept="closingDays" size={16} />
                  Absagen
                </Button>
              ) : null}
              {onDelete ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="md"
                  className="text-moto-red-strong gap-2"
                  disabled={managing}
                  onClick={() => {
                    onDelete(event);
                    onClose();
                  }}
                >
                  <Trash2 className="h-4 w-4" aria-hidden />
                  Löschen
                </Button>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

export function CalendarOverviewList({
  overview,
}: Readonly<{ overview: CalendarAppointmentOverview }>) {
  const accepted = overview.attendees.filter(
    (attendee) => attendee.status === "accepted",
  ).length;
  const declined = overview.attendees.filter(
    (attendee) => attendee.status === "declined",
  ).length;
  const pending = overview.attendees.filter(
    (attendee) => attendee.status === "pending",
  ).length;

  return (
    <div className="space-y-4">
      {overview.delivery_mode === "rsvp_required" ? (
        <div className="grid grid-cols-3 gap-2 text-center text-xs">
          <div className="bg-moto-green-soft rounded-lg px-2 py-2 text-gray-800">
            <div className="font-semibold">{accepted}</div>
            <div>Zugesagt</div>
          </div>
          <div className="bg-moto-red/10 text-moto-red-strong rounded-lg px-2 py-2">
            <div className="font-semibold">{declined}</div>
            <div>Abgesagt</div>
          </div>
          <div className="rounded-lg bg-gray-100 px-2 py-2 text-gray-700">
            <div className="font-semibold">{pending}</div>
            <div>Offen</div>
          </div>
        </div>
      ) : null}
      <div className="divide-y divide-gray-100 rounded-lg border border-gray-200 bg-white">
        {overview.attendees.map((attendee) => (
          <div
            key={attendee.recipient_id}
            className="flex items-center justify-between gap-3 px-3 py-2"
          >
            <div className="min-w-0">
              <div className="truncate text-sm font-medium text-gray-900">
                {attendee.name}
              </div>
              <div className="text-xs text-gray-500">
                {attendee.recipient_type === "staff"
                  ? "Mitarbeitende"
                  : "Eltern"}
              </div>
            </div>
            <span
              className={`shrink-0 rounded-md px-2 py-1 text-xs font-semibold ${
                responseTone[attendee.status]
              }`}
            >
              {responseLabel[attendee.status] ?? attendee.status}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

function EmptyCalendarState({
  viewMode,
}: Readonly<{ viewMode: CalendarViewMode }>) {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <EmptyState title={calendarEmptyLabel(viewMode)} />
    </div>
  );
}

/**
 * Das Bedienband des Kalenders: Zeitnavigation, Ansichtsumschalter,
 * Wochenend-Schalter und die Primäraktion. Es sitzt in der Kopfkarte der
 * Seite (`TenantPage` Slot `searchSlot`) und NICHT über dem Raster — sonst
 * hätte die Seite zwei Köpfe. Deshalb ist es eine eigene Komponente und kein
 * Teil von `PersonalCalendar`.
 */
export function PersonalCalendarChrome({
  events,
  referenceDate: rawReferenceDate,
  weekStart,
  viewMode = "week",
  showWeekend,
  onShowWeekendChange,
  onDateChange,
  onWeekChange,
  onViewModeChange,
  onCreate,
  subtitle,
}: Readonly<{
  events: readonly CalendarEvent[];
  referenceDate?: Date;
  weekStart?: Date;
  viewMode?: CalendarViewMode;
  showWeekend: boolean;
  onShowWeekendChange: (next: boolean) => void;
  onDateChange?: (date: Date) => void;
  onWeekChange?: (date: Date) => void;
  onViewModeChange?: (mode: CalendarViewMode) => void;
  onCreate?: () => void;
  subtitle?: string;
}>) {
  const referenceDate = rawReferenceDate ?? weekStart ?? new Date();
  const handleDateChange = onDateChange ?? onWeekChange ?? (() => undefined);
  const handleViewModeChange = onViewModeChange ?? (() => undefined);
  const { from, to } = getWeekRange(referenceDate);
  const days = getWeekdays(from);
  const monthDays = monthGridDays(referenceDate);
  const label = periodLabel(referenceDate, viewMode, from, to);
  const hiddenWeekendDays =
    viewMode === "month"
      ? monthDays.filter(isWeekendDay)
      : days.filter(isWeekendDay);
  const hiddenWeekendCount = showWeekend
    ? 0
    : countWeekendEvents([...events], hiddenWeekendDays);

  return (
    <PlanningContextBar
      onPrevious={() =>
        handleDateChange(shiftDate(referenceDate, viewMode, -1))
      }
      onNext={() => handleDateChange(shiftDate(referenceDate, viewMode, 1))}
      previousLabel={previousLabel(viewMode)}
      nextLabel={nextLabel(viewMode)}
      dateLabel={label}
      onToday={
        showsToday(referenceDate, viewMode)
          ? undefined
          : () => handleDateChange(new Date())
      }
      viewSwitcher={
        <SegmentedControl
          items={viewOptions.map((option) => ({
            value: option.mode,
            label: option.label,
          }))}
          value={viewMode}
          onChange={handleViewModeChange}
          ariaLabel="Ansicht wählen"
        />
      }
      actions={
        onCreate ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            className="gap-1.5"
            onClick={onCreate}
          >
            <Plus className="h-4 w-4" aria-hidden />
            Neuer Termin
          </Button>
        ) : undefined
      }
    >
      {viewMode !== "day" ? (
        <Button
          type="button"
          variant={showWeekend ? "primary" : "outline"}
          size="compact"
          className={showWeekend ? "" : "bg-white"}
          aria-pressed={showWeekend}
          onClick={() => onShowWeekendChange(!showWeekend)}
        >
          {hiddenWeekendCount > 0 ? `Sa/So (${hiddenWeekendCount})` : "Sa/So"}
        </Button>
      ) : null}
      {subtitle ? (
        <p className="truncate text-xs text-gray-500">{subtitle}</p>
      ) : null}
    </PlanningContextBar>
  );
}
