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
import { Modal } from "~/components/ui/modal";
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
// Detail-Sheet (Klick), daher kein Button-Platz mehr. Muss zum Markup in
// TimeGridEventBlock passen, sonst würde kurzer Text abgeschnitten.
function blockMinHeightPx(event: CalendarEvent): number {
  return (
    38 +
    ((event.student_name ?? event.school_name) ? 15 : 0) +
    (event.location ? 15 : 0)
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
  // Mobile lacks the space for the time grid, so it renders a per-day agenda.
  // The day set mirrors the desktop view (single day / visible week / visible
  // month) so the weekend toggle and range stay consistent across breakpoints.
  const mobileDays =
    viewMode === "day"
      ? [referenceDate]
      : viewMode === "month"
        ? visibleMonthDays
        : visibleWeekDays;

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">{title}</h1>
          {subtitle ? (
            <p className="mt-1 text-sm leading-6 text-gray-600">{subtitle}</p>
          ) : null}
        </div>
        <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center lg:justify-end">
          {/* Date navigation — full width on mobile so the range label has room
              instead of being squeezed between wrapping buttons. */}
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
            <div className="flex-1 px-2 text-center text-sm font-semibold text-gray-900 sm:min-w-52">
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
            <Button
              type="button"
              variant="outline"
              size="compact"
              className="bg-white"
              onClick={() => handleDateChange(new Date())}
            >
              Heute
            </Button>
            {viewMode !== "day" ? (
              <Button
                type="button"
                variant={showWeekend ? "primary" : "outline"}
                size="compact"
                className={showWeekend ? "" : "bg-white"}
                aria-pressed={showWeekend}
                onClick={() => setShowWeekend((value) => !value)}
              >
                {hiddenWeekendCount > 0
                  ? `Sa/So (${hiddenWeekendCount})`
                  : "Sa/So"}
              </Button>
            ) : null}
            {onCreate ? (
              <Button
                type="button"
                size="compact"
                className="w-full justify-center sm:w-auto"
                onClick={onCreate}
              >
                <Plus className="h-4 w-4" aria-hidden />
                Neuer Termin
              </Button>
            ) : null}
          </div>
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

        <div className="lg:hidden">
          <MobileAgenda
            days={mobileDays}
            events={events}
            viewMode={viewMode}
            actions={actions}
          />
        </div>

        {!loading && visibleSortedEvents.length === 0 ? (
          <div className="hidden lg:block">
            <EmptyCalendarState viewMode={viewMode} />
          </div>
        ) : null}
      </div>

      <Modal
        isOpen={selectedEvent !== null}
        onClose={() => setSelectedEvent(null)}
        title="Termin"
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-lg"
      >
        {selectedEvent ? (
          <CalendarEventDetail
            event={selectedEvent}
            actions={actions}
            onClose={() => setSelectedEvent(null)}
          />
        ) : null}
      </Modal>
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
}: Readonly<{
  days: readonly Date[];
  events: readonly CalendarEvent[];
  viewMode: CalendarViewMode;
  actions: CalendarEventActions;
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
    <div className="divide-y divide-gray-100 overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
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
            className="absolute inset-x-0.5 z-0 overflow-hidden rounded-md border px-1.5 py-1 text-left transition-[filter] hover:brightness-95"
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
          <MapPin className="h-3 w-3 shrink-0" aria-hidden />
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
  const allDay = isAllDayLike(event);
  const badge = cancelled
    ? { label: "Abgesagt", cls: "bg-[#FF3130]/10 text-[#CC2626]" }
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

// Schmale einzeilige Pille für Ganztägig-Einträge (Wochen-/Tagesraster + Mobile)
// und Monatszellen. Tap öffnet das Detail-Sheet.
function EventPill({
  event,
  actions,
}: Readonly<{ event: CalendarEvent; actions: CalendarEventActions }>) {
  const tone = sourceTone[event.source];
  const cancelled = event.cancelled === true;
  const showTime = !event.all_day && event.start_date === event.end_date;
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
      {showTime ? (
        <span className="shrink-0 text-[11px] text-gray-500 tabular-nums">
          {event.start_time}
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
          <span className="rounded-md bg-[#FF3130]/10 px-2 py-0.5 text-[11px] font-semibold text-[#CC2626]">
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
          <CalendarDays
            className="h-4 w-4 shrink-0 text-gray-400"
            aria-hidden
          />
          <span>{dateLabel}</span>
        </div>
        <div className="flex items-center gap-2">
          <Clock className="h-4 w-4 shrink-0 text-gray-400" aria-hidden />
          <span>{eventTime(event)}</span>
        </div>
        {event.location ? (
          <div className="flex items-center gap-2">
            <MapPin className="h-4 w-4 shrink-0 text-gray-400" aria-hidden />
            <span>{event.location}</span>
          </div>
        ) : null}
        {subtitle ? (
          <div className="flex items-center gap-2">
            <Users className="h-4 w-4 shrink-0 text-gray-400" aria-hidden />
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
              <CalendarPlus className="h-4 w-4" aria-hidden />
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
              <Users className="h-4 w-4" aria-hidden />
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
                  className="gap-2 text-[#CC2626]"
                  disabled={managing}
                  onClick={() => {
                    onCancel(event);
                    onClose();
                  }}
                >
                  <Ban className="h-4 w-4" aria-hidden />
                  Absagen
                </Button>
              ) : null}
              {onDelete ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="md"
                  className="gap-2 text-[#CC2626]"
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
