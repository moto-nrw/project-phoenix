"use client";

import { useEffect, useRef, useState } from "react";
import {
  Ban,
  CalendarDays,
  CalendarPlus,
  Check,
  ChevronLeft,
  ChevronRight,
  Clock,
  Loader2,
  MapPin,
  Pencil,
  Plus,
  Trash2,
  Users,
  X,
} from "lucide-react";
import { Button } from "~/components/ui/button";
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
import { LOCATION_COLORS } from "~/lib/location-helper";

export type CalendarViewMode = "day" | "week" | "month";

interface PersonalCalendarProps {
  readonly title: string;
  readonly subtitle?: string;
  readonly events: readonly CalendarEvent[];
  readonly referenceDate?: Date;
  readonly weekStart?: Date;
  readonly viewMode?: CalendarViewMode;
  readonly loading?: boolean;
  readonly error?: string | null;
  readonly onDateChange?: (nextDate: Date) => void;
  readonly onWeekChange?: (nextWeekStart: Date) => void;
  readonly onViewModeChange?: (mode: CalendarViewMode) => void;
  readonly onCreate?: () => void;
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
    bg: "#ECF7DA",
  },
  timetable: {
    label: "Betreuung",
    bar: LOCATION_COLORS.OTHER_ROOM,
    bg: "#EBF0FB",
  },
  shift: {
    label: "Dienst",
    bar: LOCATION_COLORS.SCHOOLYARD,
    bg: "#FEF3E7",
  },
} satisfies Record<
  CalendarEvent["source"],
  { label: string; bar: string; bg: string }
>;

const responseLabel: Record<string, string> = {
  pending: "Offen",
  accepted: "Zugesagt",
  declined: "Abgelehnt",
  info: "Info",
};

const responseTone: Record<CalendarResponseStatus, string> = {
  pending: "bg-gray-100 text-gray-700",
  accepted: "bg-[#ECF7DA] text-gray-800",
  declined: "bg-[#FF3130]/10 text-[#CC2626]",
  info: "bg-[#EBF0FB] text-gray-800",
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

function gridHours(
  dayEvents: readonly CalendarEvent[],
  options: TimedLayoutOptions,
): {
  startHour: number;
  endHour: number;
} {
  let startHour = DEFAULT_GRID_START_HOUR;
  let endHour = DEFAULT_GRID_END_HOUR;
  for (const event of dayEvents) {
    if (isAllDayLike(event)) continue;
    const startMinutes = clockToMinutes(event.start_time);
    startHour = Math.min(startHour, Math.floor(startMinutes / 60));
    endHour = Math.max(
      endHour,
      Math.ceil(effectiveEndMinutes(event, options) / 60),
    );
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

interface TimedLayoutOptions {
  hasRespond: boolean;
  hasOverview: boolean;
  hasManage: boolean;
  hasIcs: boolean;
}

// Mindesthöhe eines Zeitraster-Blocks in Pixeln, abhängig vom Inhalt. Muss zum
// Markup in TimeGridEventBlock passen — Aktionen und Textzeilen brauchen Platz,
// sonst wären sie bei kurzen Terminen abgeschnitten.
function blockMinHeightPx(
  event: CalendarEvent,
  options: TimedLayoutOptions,
): number {
  const cancelled = event.cancelled === true;
  const isAppointment =
    event.source === "appointment" && Boolean(event.appointment_id);
  const showRespond = Boolean(
    !cancelled && event.can_respond && event.recipient_id && options.hasRespond,
  );
  const showOverview = Boolean(
    event.can_view_overview && event.appointment_id && options.hasOverview,
  );
  const showIcs = Boolean(!cancelled && isAppointment && options.hasIcs);
  const showManage = Boolean(isAppointment && event.can_edit && options.hasManage);
  return (
    56 +
    ((event.student_name ?? event.school_name) ? 16 : 0) +
    (event.location ? 18 : 0) +
    (event.description ? 36 : 0) +
    (showRespond ? 40 : 0) +
    (showOverview ? 36 : 0) +
    (showIcs ? 36 : 0) +
    (showManage ? 44 : 0)
  );
}

// Effektives Render-Ende eines Eintrags in Minuten: das spätere von
// tatsächlichem Ende und der Mindesthöhe des gerenderten Inhalts. Rasterfenster
// und Layout rechnen beide damit, sonst laufen späte Kurztermine unten heraus.
function effectiveEndMinutes(
  event: CalendarEvent,
  options: TimedLayoutOptions,
): number {
  const startMinutes = clockToMinutes(event.start_time);
  const minRenderMinutes =
    event.source === "shift"
      ? 30
      : (blockMinHeightPx(event, options) / HOUR_PX) * 60;
  return Math.max(
    clockToMinutes(event.end_time),
    startMinutes + minRenderMinutes,
  );
}

// Klassisches Zeitraster-Layout: überlappende Einträge bilden ein Cluster und
// teilen sich die Spaltenbreite; jeder Eintrag bekommt die erste freie Spalte.
function layoutTimedEvents(
  dayEvents: readonly CalendarEvent[],
  options: TimedLayoutOptions,
): TimedPlacement[] {
  const items: TimedPlacement[] = dayEvents
    .map((event) => {
      const startMinutes = clockToMinutes(event.start_time);
      return {
        event,
        startMinutes,
        endMinutes: effectiveEndMinutes(event, options),
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

function emptyLabel(viewMode: CalendarViewMode): string {
  if (viewMode === "day") return "Keine Einträge an diesem Tag.";
  if (viewMode === "month") return "Keine Einträge in diesem Monat.";
  return "Keine Einträge in dieser Woche.";
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
  title,
  subtitle,
  events,
  referenceDate: rawReferenceDate,
  weekStart,
  viewMode = "week",
  loading,
  error,
  onDateChange,
  onWeekChange,
  onViewModeChange,
  onCreate,
  onShowOverview,
  onRespond,
  respondingRecipientId,
  onEdit,
  onCancel,
  onDelete,
  busyAppointmentId,
  icsHrefBase,
}: PersonalCalendarProps) {
  const actions: CalendarEventActions = {
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
  const handleDateChange = onDateChange ?? onWeekChange ?? (() => undefined);
  const handleViewModeChange = onViewModeChange ?? (() => undefined);
  const { from, to } = getWeekRange(referenceDate);
  const days = getWeekdays(from);
  const monthDays = monthGridDays(referenceDate);
  const sortedEvents = [...events].sort((a, b) =>
    eventSortValue(a).localeCompare(eventSortValue(b)),
  );
  const label = periodLabel(referenceDate, viewMode, from, to);
  const [showWeekend, setShowWeekend] = useState(false);
  const visibleWeekDays = showWeekend
    ? days
    : days.filter((day) => !isWeekendDay(day));
  const visibleMonthDays = showWeekend
    ? monthDays
    : monthDays.filter((day) => !isWeekendDay(day));
  const hiddenWeekendDays =
    viewMode === "month"
      ? monthDays.filter(isWeekendDay)
      : days.filter(isWeekendDay);
  const hiddenWeekendCount = showWeekend
    ? 0
    : countWeekendEvents(events, hiddenWeekendDays);
  const visibleSortedEvents =
    showWeekend || viewMode === "day"
      ? sortedEvents
      : sortedEvents.filter((event) => !isWeekendOnlyEvent(event));

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 border-b border-gray-200 pb-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm font-semibold text-gray-500">
            <CalendarDays className="h-4 w-4" aria-hidden />
            Kalender
          </div>
          <h1 className="mt-1 text-2xl font-semibold text-gray-900">{title}</h1>
          {subtitle ? (
            <p className="mt-1 text-sm leading-6 text-gray-600">{subtitle}</p>
          ) : null}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex items-center gap-1 rounded-lg border border-gray-200 bg-white p-1 shadow-sm">
            {viewOptions.map((option) => {
              const selected = option.mode === viewMode;
              return (
                <Button
                  key={option.mode}
                  type="button"
                  variant={selected ? "primary" : "ghost"}
                  size="compact"
                  aria-pressed={selected}
                  onClick={() => handleViewModeChange(option.mode)}
                >
                  {option.label}
                </Button>
              );
            })}
          </div>
          <div className="flex items-center gap-1 rounded-lg border border-gray-200 bg-white p-1 shadow-sm">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={previousLabel(viewMode)}
              onClick={() =>
                handleDateChange(shiftDate(referenceDate, viewMode, -1))
              }
            >
              <ChevronLeft className="h-4 w-4" aria-hidden />
            </Button>
            <div className="min-w-52 px-2 text-center text-sm font-semibold text-gray-900">
              {label}
            </div>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={nextLabel(viewMode)}
              onClick={() =>
                handleDateChange(shiftDate(referenceDate, viewMode, 1))
              }
            >
              <ChevronRight className="h-4 w-4" aria-hidden />
            </Button>
          </div>
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => handleDateChange(new Date())}
          >
            Heute
          </Button>
          {viewMode !== "day" ? (
            <Button
              type="button"
              variant={showWeekend ? "primary" : "outline"}
              size="compact"
              aria-pressed={showWeekend}
              onClick={() => setShowWeekend((value) => !value)}
            >
              {hiddenWeekendCount > 0
                ? `Sa/So (${hiddenWeekendCount})`
                : "Sa/So"}
            </Button>
          ) : null}
          {onCreate ? (
            <Button type="button" size="compact" onClick={onCreate}>
              <Plus className="mr-1.5 h-4 w-4" aria-hidden />
              Neuer Termin
            </Button>
          ) : null}
        </div>
      </div>

      {error ? (
        <div className="rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
          {error}
        </div>
      ) : null}

      <div className="relative">
        {loading ? (
          <div className="absolute inset-0 z-10 grid place-items-center bg-white/70">
            <Loader2 className="h-6 w-6 animate-spin text-gray-500" />
          </div>
        ) : null}
        {viewMode === "day" ? (
          <div className="hidden overflow-hidden rounded-lg border border-gray-200 bg-white lg:block">
            <CalendarTimeGrid
              days={[referenceDate]}
              events={events}
              actions={actions}
            />
          </div>
        ) : null}

        {viewMode === "week" ? (
          <div className="hidden overflow-hidden rounded-lg border border-gray-200 bg-white lg:block">
            <CalendarTimeGrid
              days={visibleWeekDays}
              events={events}
              actions={actions}
            />
          </div>
        ) : null}

        {viewMode === "month" ? (
          <div
            className={`hidden overflow-hidden rounded-lg border border-gray-200 bg-white lg:grid ${
              showWeekend ? "lg:grid-cols-7" : "lg:grid-cols-5"
            }`}
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
        ) : null}

        <div className="space-y-3 lg:hidden">
          {visibleSortedEvents.length === 0 ? (
            <EmptyCalendarState viewMode={viewMode} />
          ) : (
            visibleSortedEvents.map((event) => (
              <CalendarEventItem
                key={event.id}
                event={event}
                actions={actions}
              />
            ))
          )}
        </div>

        {!loading && visibleSortedEvents.length === 0 ? (
          <div className="hidden lg:block">
            <EmptyCalendarState viewMode={viewMode} />
          </div>
        ) : null}
      </div>
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
      <div className={`space-y-2 ${compact ? "p-1.5" : "p-2"}`}>
        {events.map((event) => (
          <CalendarEventItem
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
  const layoutOptions: TimedLayoutOptions = {
    hasRespond: Boolean(actions.onRespond),
    hasOverview: Boolean(actions.onShowOverview),
    hasManage: Boolean(actions.onEdit ?? actions.onCancel ?? actions.onDelete),
    hasIcs: Boolean(actions.icsHrefBase),
  };
  const { startHour, endHour } = gridHours(
    dayBuckets.flatMap((bucket) => bucket.events),
    layoutOptions,
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
        <div className="w-14 shrink-0 border-r border-gray-200 bg-gray-50" />
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
          <div className="w-14 shrink-0 border-r border-gray-200 px-1 py-2 text-right text-[11px] text-gray-400">
            Ganztägig
          </div>
          <div className="grid flex-1" style={columnTemplate}>
            {dayBuckets.map(({ day, events: dayEvents }) => (
              <div
                key={toISODate(day)}
                className="space-y-1 border-r border-gray-200 p-1 last:border-r-0"
              >
                {dayEvents.filter(isAllDayLike).map((event) => (
                  <CalendarEventItem
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
          <div className="relative w-14 shrink-0 border-r border-gray-200">
            {hours.map((hour) =>
              hour === startHour ? null : (
                <div
                  key={hour}
                  className="absolute right-1 -translate-y-1/2 text-[11px] text-gray-400"
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
    {
      hasRespond: Boolean(actions.onRespond),
      hasOverview: Boolean(actions.onShowOverview),
      hasManage: Boolean(actions.onEdit ?? actions.onCancel ?? actions.onDelete),
      hasIcs: Boolean(actions.icsHrefBase),
    },
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
      {shiftBands.map((event) => {
        const tone = sourceTone[event.source];
        const startMinutes = clockToMinutes(event.start_time);
        const endMinutes = Math.max(
          clockToMinutes(event.end_time),
          startMinutes + 30,
        );
        return (
          <div
            key={event.id}
            className="absolute inset-x-0.5 z-0 overflow-hidden rounded-md border px-1.5 py-1"
            style={{
              top: ((startMinutes - gridStartMinutes) / 60) * HOUR_PX + 1,
              height: ((endMinutes - startMinutes) / 60) * HOUR_PX - 2,
              backgroundColor: tone.bg,
              borderColor: `${tone.bar}55`,
            }}
          >
            <div className="flex flex-wrap items-center gap-1 text-[11px] text-gray-700">
              <span className="rounded bg-white/80 px-1 py-0.5 font-semibold">
                {tone.label}
              </span>
              <span className="font-semibold text-gray-900">{event.title}</span>
              <span>{eventTime(event)}</span>
            </div>
            {event.description ? (
              <p className="mt-0.5 truncate text-[11px] text-gray-600">
                {event.description}
              </p>
            ) : null}
          </div>
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
  const { event, startMinutes, endMinutes, column, columnCount } = placement;
  const tone = sourceTone[event.source];
  const recipientId = event.recipient_id;
  const cancelled = event.cancelled === true;
  const responding =
    Boolean(recipientId) && respondingRecipientId === recipientId;
  // A cancelled appointment can no longer be answered — hide RSVP, matching
  // CalendarEventItem.
  const showRespond = Boolean(
    !cancelled && event.can_respond && recipientId && onRespond,
  );
  const showOverview = Boolean(
    event.can_view_overview && event.appointment_id && onShowOverview,
  );
  // Day/week timed blocks must offer the same management + export as the month
  // and mobile CalendarEventItem, so organizers and parents never have to switch
  // views to edit, cancel, delete, or download a timed appointment.
  const canManage =
    event.source === "appointment" &&
    event.can_edit &&
    Boolean(event.appointment_id) &&
    Boolean(onEdit ?? onCancel ?? onDelete);
  const managing =
    Boolean(event.appointment_id) && busyAppointmentId === event.appointment_id;
  const showIcs = Boolean(
    !cancelled &&
      event.source === "appointment" &&
      event.appointment_id &&
      icsHrefBase,
  );
  // endMinutes ist bereits das effektive Render-Ende aus layoutTimedEvents —
  // die Mindesthöhe steckt in der Platzierung, damit nichts überdeckt wird.
  const height = ((endMinutes - startMinutes) / 60) * HOUR_PX - 2;
  const widthPercent = 100 / columnCount;
  return (
    <article
      className="absolute z-10 overflow-hidden rounded-md border border-gray-200 p-1.5 shadow-sm"
      style={{
        top: ((startMinutes - gridStartMinutes) / 60) * HOUR_PX + 1,
        height,
        left: `calc(${column * widthPercent}% + 2px)`,
        width: `calc(${widthPercent}% - 4px)`,
        borderLeft: `3px solid ${tone.bar}`,
        backgroundColor: tone.bg,
      }}
    >
      <div className="flex flex-wrap items-center gap-1">
        <span className="rounded bg-white/80 px-1 py-0.5 text-[10px] font-semibold text-gray-700">
          {tone.label}
        </span>
        {event.response_status ? (
          <span className="rounded bg-white/80 px-1 py-0.5 text-[10px] font-semibold text-gray-700">
            {responseLabel[event.response_status] ?? event.response_status}
          </span>
        ) : null}
      </div>
      <div className="truncate text-xs font-semibold text-gray-950">
        {event.title}
      </div>
      <div className="text-[11px] text-gray-600">{eventTime(event)}</div>
      {event.student_name || event.school_name ? (
        <div className="truncate text-[11px] text-gray-600">
          {[event.student_name, event.school_name].filter(Boolean).join(" · ")}
        </div>
      ) : null}
      {event.location ? (
        <div className="flex items-center gap-1 text-[11px] text-gray-600">
          <MapPin className="h-3 w-3 shrink-0" aria-hidden />
          <span className="truncate">{event.location}</span>
        </div>
      ) : null}
      {event.description ? (
        <p className="mt-0.5 line-clamp-2 text-[11px] leading-4 text-gray-600">
          {event.description}
        </p>
      ) : null}
      {showOverview ? (
        <Button
          type="button"
          size="compact"
          variant="ghost"
          className="mt-1 w-full bg-white/50"
          onClick={() => onShowOverview!(event.appointment_id!)}
        >
          <Users className="h-4 w-4" aria-hidden />
          Teilnehmer
        </Button>
      ) : null}
      {showRespond ? (
        <div className="mt-1 flex gap-1">
          <Button
            type="button"
            size="compact"
            variant="outline"
            className="flex-1"
            disabled={responding}
            onClick={() => onRespond!(recipientId!, "accepted")}
          >
            <Check className="h-4 w-4" aria-hidden />
            Zusagen
          </Button>
          <Button
            type="button"
            size="compact"
            variant="outline_danger"
            className="flex-1"
            disabled={responding}
            onClick={() => onRespond!(recipientId!, "declined")}
          >
            <X className="h-4 w-4" aria-hidden />
            Absagen
          </Button>
        </div>
      ) : null}
      {showIcs ? (
        <a
          href={`${icsHrefBase}/${encodeURIComponent(event.appointment_id!)}/ics`}
          download
          className="mt-1 inline-flex h-8 w-full items-center justify-center gap-1.5 rounded-md bg-white/50 px-2.5 text-xs font-medium text-gray-700 transition-colors hover:bg-white"
        >
          <CalendarPlus className="h-4 w-4" aria-hidden />
          Zum Kalender hinzufügen
        </a>
      ) : null}
      {canManage ? (
        <div className="mt-1 flex flex-wrap gap-1 border-t border-white/60 pt-1.5">
          {onEdit && !cancelled ? (
            <Button
              type="button"
              size="compact"
              variant="ghost"
              className="bg-white/50"
              disabled={managing}
              onClick={() => onEdit(event)}
            >
              <Pencil className="h-4 w-4" aria-hidden />
              Bearbeiten
            </Button>
          ) : null}
          {onCancel && !cancelled ? (
            <Button
              type="button"
              size="compact"
              variant="ghost"
              className="bg-white/50 text-[#CC2626]"
              disabled={managing}
              onClick={() => onCancel(event)}
            >
              <Ban className="h-4 w-4" aria-hidden />
              Absagen
            </Button>
          ) : null}
          {onDelete ? (
            <Button
              type="button"
              size="compact"
              variant="ghost"
              className="bg-white/50 text-[#CC2626]"
              disabled={managing}
              onClick={() => onDelete(event)}
            >
              <Trash2 className="h-4 w-4" aria-hidden />
              Löschen
            </Button>
          ) : null}
        </div>
      ) : null}
    </article>
  );
}

function CalendarEventItem({
  event,
  actions,
}: Readonly<{
  event: CalendarEvent;
  actions: CalendarEventActions;
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
  const canManage =
    event.source === "appointment" &&
    event.can_edit &&
    Boolean(event.appointment_id) &&
    Boolean(onEdit ?? onCancel ?? onDelete);
  const managing =
    Boolean(event.appointment_id) && busyAppointmentId === event.appointment_id;
  return (
    <article
      className={`rounded-lg border border-gray-200 bg-white p-3 shadow-sm ${
        cancelled ? "opacity-70" : ""
      }`}
      style={{ borderLeft: `4px solid ${tone.bar}`, backgroundColor: tone.bg }}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="rounded-md bg-white/80 px-1.5 py-0.5 text-[11px] font-semibold text-gray-700">
              {tone.label}
            </span>
            {cancelled ? (
              <span className="rounded-md bg-[#FF3130]/10 px-1.5 py-0.5 text-[11px] font-semibold text-[#CC2626]">
                Abgesagt
              </span>
            ) : null}
            {!cancelled && event.response_status ? (
              <span className="rounded-md bg-white/80 px-1.5 py-0.5 text-[11px] font-semibold text-gray-700">
                {responseLabel[event.response_status] ?? event.response_status}
              </span>
            ) : null}
          </div>
          <h2
            className={`mt-1 truncate text-sm font-semibold text-gray-950 ${
              cancelled ? "line-through" : ""
            }`}
          >
            {event.title}
          </h2>
        </div>
      </div>
      <div className="mt-2 space-y-1 text-xs text-gray-700">
        <div className="flex items-center gap-1.5">
          <Clock className="h-3.5 w-3.5" aria-hidden />
          <span>{eventTime(event)}</span>
        </div>
        {event.location ? (
          <div className="flex items-center gap-1.5">
            <MapPin className="h-3.5 w-3.5" aria-hidden />
            <span className="truncate">{event.location}</span>
          </div>
        ) : null}
        {event.student_name || event.school_name ? (
          <div className="truncate">
            {[event.student_name, event.school_name]
              .filter(Boolean)
              .join(" · ")}
          </div>
        ) : null}
      </div>
      {event.description ? (
        <p className="mt-2 line-clamp-3 text-xs leading-5 text-gray-700">
          {event.description}
        </p>
      ) : null}
      {event.can_view_overview && event.appointment_id && onShowOverview ? (
        <Button
          type="button"
          size="compact"
          variant="ghost"
          className="mt-3 w-full bg-white/50"
          onClick={() => onShowOverview(event.appointment_id!)}
        >
          <Users className="h-4 w-4" aria-hidden />
          Teilnehmer
        </Button>
      ) : null}
      {!cancelled &&
      event.source === "appointment" &&
      event.appointment_id &&
      icsHrefBase ? (
        <a
          href={`${icsHrefBase}/${encodeURIComponent(event.appointment_id)}/ics`}
          download
          className="mt-2 inline-flex h-8 w-full items-center justify-center gap-1.5 rounded-md bg-white/50 px-2.5 text-sm font-medium text-gray-700 transition-colors hover:bg-white"
        >
          <CalendarPlus className="h-4 w-4" aria-hidden />
          Zum Kalender hinzufügen
        </a>
      ) : null}
      {!cancelled && event.can_respond && recipientId && onRespond ? (
        <div className="mt-3 grid gap-1.5">
          <Button
            type="button"
            size="compact"
            variant="outline"
            className="w-full"
            disabled={responding}
            onClick={() => onRespond(recipientId, "accepted")}
          >
            <Check className="h-4 w-4" aria-hidden />
            Zusagen
          </Button>
          <Button
            type="button"
            size="compact"
            variant="outline_danger"
            className="w-full"
            disabled={responding}
            onClick={() => onRespond(recipientId, "declined")}
          >
            <X className="h-4 w-4" aria-hidden />
            Absagen
          </Button>
        </div>
      ) : null}
      {canManage ? (
        <div className="mt-3 flex flex-wrap gap-1.5 border-t border-white/60 pt-2.5">
          {onEdit && !cancelled ? (
            <Button
              type="button"
              size="compact"
              variant="ghost"
              className="bg-white/50"
              disabled={managing}
              onClick={() => onEdit(event)}
            >
              <Pencil className="h-4 w-4" aria-hidden />
              Bearbeiten
            </Button>
          ) : null}
          {onCancel && !cancelled ? (
            <Button
              type="button"
              size="compact"
              variant="ghost"
              className="bg-white/50 text-[#CC2626]"
              disabled={managing}
              onClick={() => onCancel(event)}
            >
              <Ban className="h-4 w-4" aria-hidden />
              Absagen
            </Button>
          ) : null}
          {onDelete ? (
            <Button
              type="button"
              size="compact"
              variant="ghost"
              className="bg-white/50 text-[#CC2626]"
              disabled={managing}
              onClick={() => onDelete(event)}
            >
              <Trash2 className="h-4 w-4" aria-hidden />
              Löschen
            </Button>
          ) : null}
        </div>
      ) : null}
    </article>
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
          <div className="rounded-lg bg-[#ECF7DA] px-2 py-2 text-gray-800">
            <div className="font-semibold">{accepted}</div>
            <div>Zugesagt</div>
          </div>
          <div className="rounded-lg bg-[#FF3130]/10 px-2 py-2 text-[#CC2626]">
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
    <div className="rounded-lg border border-dashed border-gray-300 bg-white p-8 text-center text-sm text-gray-500">
      {emptyLabel(viewMode)}
    </div>
  );
}
