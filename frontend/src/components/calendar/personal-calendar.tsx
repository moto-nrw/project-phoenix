"use client";

import {
  CalendarDays,
  Check,
  ChevronLeft,
  ChevronRight,
  Clock,
  Loader2,
  MapPin,
  Plus,
  X,
} from "lucide-react";
import { Button } from "~/components/ui/button";
import type { CalendarEvent } from "~/lib/personal-calendar-api";
import { toISODate } from "~/lib/date-helpers";
import {
  formatDayHeader,
  formatWeekLabel,
  getWeekRange,
  getWeekdays,
} from "~/lib/timetable-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";

interface PersonalCalendarProps {
  readonly title: string;
  readonly subtitle?: string;
  readonly events: readonly CalendarEvent[];
  readonly weekStart: Date;
  readonly loading?: boolean;
  readonly error?: string | null;
  readonly onWeekChange: (nextWeekStart: Date) => void;
  readonly onCreate?: () => void;
  readonly onRespond?: (
    recipientId: string,
    status: "accepted" | "declined",
  ) => void;
  readonly respondingRecipientId?: string | null;
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

function shiftWeek(weekStart: Date, offset: number): Date {
  const next = new Date(weekStart);
  next.setDate(next.getDate() + offset * 7);
  return getWeekRange(next).from;
}

export function PersonalCalendar({
  title,
  subtitle,
  events,
  weekStart,
  loading,
  error,
  onWeekChange,
  onCreate,
  onRespond,
  respondingRecipientId,
}: PersonalCalendarProps) {
  const { from, to } = getWeekRange(weekStart);
  const days = getWeekdays(from);
  const sortedEvents = [...events].sort((a, b) =>
    eventSortValue(a).localeCompare(eventSortValue(b)),
  );

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
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label="Vorherige Woche"
              onClick={() => onWeekChange(shiftWeek(from, -1))}
            >
              <ChevronLeft className="h-4 w-4" aria-hidden />
            </Button>
            <div className="min-w-52 px-2 text-center text-sm font-semibold text-gray-900">
              {formatWeekLabel(from, to)}
            </div>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label="Nächste Woche"
              onClick={() => onWeekChange(shiftWeek(from, 1))}
            >
              <ChevronRight className="h-4 w-4" aria-hidden />
            </Button>
          </div>
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => onWeekChange(getWeekRange(new Date()).from)}
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
        <div className="hidden overflow-hidden rounded-lg border border-gray-200 bg-white lg:grid lg:grid-cols-7">
          {days.map((day) => {
            const dayEvents = eventsForDay(events, day);
            return (
              <section
                key={toISODate(day)}
                className="min-h-96 border-r border-gray-200 last:border-r-0"
              >
                <div className="border-b border-gray-200 bg-gray-50 px-3 py-2">
                  <div className="text-sm font-semibold text-gray-900">
                    {formatDayHeader(day)}
                  </div>
                  <div className="text-xs text-gray-500">
                    {dayEvents.length === 1
                      ? "1 Eintrag"
                      : `${dayEvents.length} Einträge`}
                  </div>
                </div>
                <div className="space-y-2 p-2">
                  {dayEvents.map((event) => (
                    <CalendarEventItem
                      key={`${event.id}-${toISODate(day)}`}
                      event={event}
                      onRespond={onRespond}
                      respondingRecipientId={respondingRecipientId}
                    />
                  ))}
                </div>
              </section>
            );
          })}
        </div>

        <div className="space-y-3 lg:hidden">
          {sortedEvents.length === 0 ? (
            <EmptyCalendarState />
          ) : (
            sortedEvents.map((event) => (
              <CalendarEventItem
                key={event.id}
                event={event}
                onRespond={onRespond}
                respondingRecipientId={respondingRecipientId}
              />
            ))
          )}
        </div>

        {!loading && sortedEvents.length === 0 ? (
          <div className="hidden lg:block">
            <EmptyCalendarState />
          </div>
        ) : null}
      </div>
    </div>
  );
}

function CalendarEventItem({
  event,
  onRespond,
  respondingRecipientId,
}: Readonly<{
  event: CalendarEvent;
  onRespond?: (recipientId: string, status: "accepted" | "declined") => void;
  respondingRecipientId?: string | null;
}>) {
  const tone = sourceTone[event.source];
  const recipientId = event.recipient_id;
  const responding =
    Boolean(recipientId) && respondingRecipientId === recipientId;
  return (
    <article
      className="rounded-lg border border-gray-200 bg-white p-3 shadow-sm"
      style={{ borderLeft: `4px solid ${tone.bar}`, backgroundColor: tone.bg }}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="rounded-md bg-white/80 px-1.5 py-0.5 text-[11px] font-semibold text-gray-700">
              {tone.label}
            </span>
            {event.response_status ? (
              <span className="rounded-md bg-white/80 px-1.5 py-0.5 text-[11px] font-semibold text-gray-700">
                {responseLabel[event.response_status] ?? event.response_status}
              </span>
            ) : null}
          </div>
          <h2 className="mt-1 truncate text-sm font-semibold text-gray-950">
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
      {event.can_respond && recipientId && onRespond ? (
        <div className="mt-3 flex gap-2">
          <Button
            type="button"
            size="compact"
            variant="outline"
            disabled={responding}
            onClick={() => onRespond(recipientId, "accepted")}
          >
            <Check className="mr-1 h-4 w-4" aria-hidden />
            Zusagen
          </Button>
          <Button
            type="button"
            size="compact"
            variant="outline_danger"
            disabled={responding}
            onClick={() => onRespond(recipientId, "declined")}
          >
            <X className="mr-1 h-4 w-4" aria-hidden />
            Absagen
          </Button>
        </div>
      ) : null}
    </article>
  );
}

function EmptyCalendarState() {
  return (
    <div className="rounded-lg border border-dashed border-gray-300 bg-white p-8 text-center text-sm text-gray-500">
      Keine Einträge in dieser Woche.
    </div>
  );
}
