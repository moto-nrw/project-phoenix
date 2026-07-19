"use client";

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
import { toISODate } from "~/lib/date-helpers";
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
      weekday: "long",
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    });
  }
  if (viewMode === "month") {
    return referenceDate.toLocaleDateString("de-DE", {
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
            <CalendarDayColumn
              day={referenceDate}
              events={eventsForDay(events, referenceDate)}
              actions={actions}
              className="min-h-96"
            />
          </div>
        ) : null}

        {viewMode === "week" ? (
          <div className="hidden overflow-hidden rounded-lg border border-gray-200 bg-white lg:grid lg:grid-cols-7">
            {days.map((day) => (
              <CalendarDayColumn
                key={toISODate(day)}
                day={day}
                events={eventsForDay(events, day)}
                actions={actions}
                className="min-h-96 border-r border-gray-200 last:border-r-0"
              />
            ))}
          </div>
        ) : null}

        {viewMode === "month" ? (
          <div className="hidden overflow-hidden rounded-lg border border-gray-200 bg-white lg:grid lg:grid-cols-7">
            {monthDays.map((day) => {
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
          {sortedEvents.length === 0 ? (
            <EmptyCalendarState viewMode={viewMode} />
          ) : (
            sortedEvents.map((event) => (
              <CalendarEventItem
                key={event.id}
                event={event}
                actions={actions}
              />
            ))
          )}
        </div>

        {!loading && sortedEvents.length === 0 ? (
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
