"use client";

import {
  type ReactNode,
  type RefObject,
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import {
  BuildingsIcon,
  CalendarDotsIcon,
  CaretLeftIcon,
  CaretRightIcon,
  CheckCircleIcon,
  ClockIcon,
  MapPinIcon,
  UserIcon,
  XIcon,
} from "@phosphor-icons/react/ssr";
import { CalendarSubscribePanel } from "~/components/calendar/calendar-subscribe-panel";
import {
  ParentPage,
  ParentPageHeader,
  ParentSectionSkeleton,
} from "~/components/parent/parent-page";
import { Alert } from "~/components/ui/alert";
import { AnchoredPopover } from "~/components/ui/anchored-popover";
import { Button } from "~/components/ui/button";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { EmptyState } from "~/components/ui/empty-state";
import { Skeleton } from "~/components/ui/skeleton";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { useToast } from "~/contexts/ToastContext";
import {
  berlinTodayISO,
  isValidISODate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { BELOW_LG, useMediaQuery } from "~/lib/hooks/use-media-query";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import {
  formatCalendarDate,
  formatLocalizedDate,
} from "~/lib/localized-date-format";
import {
  getParentCalendar,
  respondParentCalendar,
  type CalendarEvent,
} from "~/lib/personal-calendar-api";

const logger = createLogger({ component: "ParentCalendarPage" });
const MAX_CALENDAR_WINDOW_OFFSET = 91;
const MAX_VISIBLE_EVENTS = 2;

type EventState =
  "cancelled" | "needsResponse" | "accepted" | "declined" | "info";
type EventGroupKey = "thisWeek" | "nextWeek" | "later";

function eventState(event: CalendarEvent): EventState {
  if (event.cancelled) return "cancelled";
  if (event.can_respond && event.response_status === "pending") {
    return "needsResponse";
  }
  if (event.response_status === "accepted") return "accepted";
  if (event.response_status === "declined") return "declined";
  return "info";
}

function eventColors(event: CalendarEvent) {
  const state = eventState(event);
  if (state === "cancelled") return MOTO_COLOR_PALETTE.red;
  if (state === "needsResponse") return MOTO_COLOR_PALETTE.orange;
  if (state === "accepted") return MOTO_COLOR_PALETTE.green;
  if (state === "declined") return MOTO_COLOR_PALETTE.neutral;
  return MOTO_COLOR_PALETTE.blue;
}

function pendingHatch(color: string) {
  return `repeating-linear-gradient(135deg, transparent 0 7px, color-mix(in srgb, ${color} 12%, transparent) 7px 9px)`;
}

function addDays(dateISO: string, amount: number): string {
  const date = parseISODate(dateISO);
  date.setDate(date.getDate() + amount);
  return toISODate(date);
}

function endOfWeek(dateISO: string): string {
  const date = parseISODate(dateISO);
  const mondayIndex = (date.getDay() + 6) % 7;
  return addDays(dateISO, 6 - mondayIndex);
}

function monthDays(referenceDate: Date): Date[] {
  const first = new Date(
    referenceDate.getFullYear(),
    referenceDate.getMonth(),
    1,
  );
  const firstWeekday = (first.getDay() + 6) % 7;
  const start = new Date(first);
  start.setDate(first.getDate() - firstWeekday);
  const daysInMonth = new Date(
    referenceDate.getFullYear(),
    referenceDate.getMonth() + 1,
    0,
  ).getDate();
  const visibleDayCount = Math.ceil((firstWeekday + daysInMonth) / 7) * 7;
  return Array.from({ length: visibleDayCount }, (_, index) => {
    const day = new Date(start);
    day.setDate(start.getDate() + index);
    return day;
  });
}

function monthReference(dateISO: string): Date {
  const date = parseISODate(dateISO);
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function sortEvents(events: readonly CalendarEvent[]): CalendarEvent[] {
  return [...events].sort(
    (a, b) =>
      a.start_date.localeCompare(b.start_date) ||
      (a.all_day ? "" : a.start_time).localeCompare(
        b.all_day ? "" : b.start_time,
      ) ||
      a.title.localeCompare(b.title),
  );
}

function eventsForDay(
  events: readonly CalendarEvent[],
  dateISO: string,
): CalendarEvent[] {
  return sortEvents(
    events.filter(
      (event) => event.start_date <= dateISO && event.end_date >= dateISO,
    ),
  );
}

function groupEvents(
  events: readonly CalendarEvent[],
  today: string,
): Record<EventGroupKey, CalendarEvent[]> {
  const thisWeekEnd = endOfWeek(today);
  const nextWeekEnd = addDays(thisWeekEnd, 7);
  const groups: Record<EventGroupKey, CalendarEvent[]> = {
    thisWeek: [],
    nextWeek: [],
    later: [],
  };
  for (const event of sortEvents(events)) {
    if (event.end_date < today) continue;
    const groupDate = event.start_date < today ? today : event.start_date;
    if (groupDate <= thisWeekEnd) groups.thisWeek.push(event);
    else if (groupDate <= nextWeekEnd) groups.nextWeek.push(event);
    else groups.later.push(event);
  }
  return groups;
}

function eventType(
  event: CalendarEvent,
  t: ReturnType<typeof useTranslations>,
) {
  if (event.source === "timetable") return t("types.care");
  if (event.delivery_mode === "rsvp_required") return t("types.invitation");
  return t("types.appointment");
}

function eventTime(event: CalendarEvent, allDayLabel: string): string {
  if (event.all_day) return allDayLabel;
  if (!event.end_time) return event.start_time;
  return `${event.start_time} - ${event.end_time}`;
}

export function ParentCalendarPage() {
  const t = useTranslations("parentCalendar");
  const locale = useLocale();
  const toast = useToast();
  const searchParams = useSearchParams();
  const today = useMemo(() => berlinTodayISO(), []);
  const rangeEnd = useMemo(
    () => addDays(today, MAX_CALENDAR_WINDOW_OFFSET),
    [today],
  );
  const lastCompleteMonth = useMemo(() => {
    const end = parseISODate(rangeEnd);
    return new Date(end.getFullYear(), end.getMonth() - 1, 1);
  }, [rangeEnd]);
  const focusDate = searchParams.get("date");
  const validFocusDate =
    focusDate !== null && isValidISODate(focusDate) ? focusDate : null;
  const focusMonth = validFocusDate ? monthReference(validFocusDate) : null;
  const focusIsLoaded =
    validFocusDate !== null &&
    validFocusDate >= today &&
    validFocusDate <= rangeEnd &&
    focusMonth !== null &&
    focusMonth <= lastCompleteMonth;
  const initialDate = focusIsLoaded ? validFocusDate : today;
  const [referenceDate, setReferenceDate] = useState(() =>
    monthReference(initialDate),
  );
  const [events, setEvents] = useState<CalendarEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [respondingId, setRespondingId] = useState<string | null>(null);
  const seqRef = useRef(0);

  const load = useCallback(async () => {
    const seq = ++seqRef.current;
    setLoading(true);
    try {
      const response = await getParentCalendar(
        parseISODate(today),
        parseISODate(rangeEnd),
      );
      if (seq !== seqRef.current) return;
      setEvents(response.events);
      setError(false);
    } catch (err) {
      if (seq !== seqRef.current) return;
      logger.warn("parent_calendar_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(true);
    } finally {
      if (seq === seqRef.current) setLoading(false);
    }
  }, [rangeEnd, today]);

  useEffect(() => {
    void load();
  }, [load]);

  const respond = async (
    event: CalendarEvent,
    status: "accepted" | "declined",
  ) => {
    if (!event.recipient_id) return;
    setRespondingId(event.id);
    try {
      await respondParentCalendar(event.recipient_id, status);
      const update = (candidate: CalendarEvent) =>
        candidate.id === event.id
          ? { ...candidate, response_status: status }
          : candidate;
      setEvents((current) => current.map(update));
      toast.success(status === "accepted" ? t("accepted") : t("declined"));
    } catch (err) {
      logger.warn("parent_calendar_respond_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(t("respondError"));
    } finally {
      setRespondingId(null);
    }
  };

  const groupedEvents = useMemo(
    () => groupEvents(events, today),
    [events, today],
  );
  const hasEvents = Object.values(groupedEvents).some(
    (group) => group.length > 0,
  );

  if (loading && events.length === 0) {
    return (
      <ParentPage>
        <ParentPageHeader
          kicker={t("kicker")}
          title={t("title")}
          description={t("description")}
        />
        <CalendarPageSkeleton />
      </ParentPage>
    );
  }

  return (
    <ParentPage>
      <ParentPageHeader
        kicker={t("kicker")}
        title={t("title")}
        description={t("description")}
      />

      {error ? <Alert type="error" message={t("loadError")} /> : null}

      <div className="grid items-stretch gap-5 lg:grid-cols-[minmax(20rem,26rem)_minmax(0,1fr)]">
        {hasEvents ? (
          <CalendarEventList
            groups={groupedEvents}
            locale={locale}
            respondingId={respondingId}
            onRespond={respond}
          />
        ) : !error ? (
          <section className="moto-content-surface h-full rounded-2xl border p-5 shadow-sm sm:p-6">
            <EmptyState
              icon={<CalendarDotsIcon className="size-8" />}
              title={t("empty")}
            />
          </section>
        ) : null}

        <div className="hidden h-full lg:col-start-2 lg:block">
          <CalendarMonthPanel
            events={events}
            referenceDate={referenceDate}
            today={today}
            rangeEnd={rangeEnd}
            locale={locale}
            respondingId={respondingId}
            canGoBack={referenceDate > monthReference(today)}
            canGoForward={referenceDate < lastCompleteMonth}
            onChangeMonth={(offset) => {
              const next = new Date(
                referenceDate.getFullYear(),
                referenceDate.getMonth() + offset,
                1,
              );
              setReferenceDate(next);
            }}
            onRespond={respond}
          />
        </div>
      </div>

      <CalendarSubscribePanel />
    </ParentPage>
  );
}

function CalendarPageSkeleton() {
  return (
    <>
      <div
        data-testid="parent-calendar-skeleton"
        className="grid items-stretch gap-5 lg:grid-cols-[minmax(20rem,26rem)_minmax(0,1fr)]"
        aria-hidden="true"
      >
        <section className="moto-content-surface h-full space-y-5 rounded-2xl border p-4 shadow-sm">
          {[2, 1].map((rowCount) => (
            <div key={rowCount} className="space-y-3">
              <Skeleton className="h-6 w-32" />
              {Array.from({ length: rowCount }, (_, index) => (
                <Skeleton key={index} className="h-20 w-full rounded-r-lg" />
              ))}
            </div>
          ))}
        </section>
        <section className="moto-content-surface hidden h-full overflow-hidden rounded-2xl border shadow-sm lg:block">
          <div className="flex items-center justify-between border-b border-gray-200 p-4">
            <Skeleton className="size-8 rounded-md" />
            <Skeleton className="h-5 w-36" />
            <Skeleton className="size-8 rounded-md" />
          </div>
          <div className="grid grid-cols-7 gap-px bg-gray-100 p-px">
            {Array.from({ length: 42 }, (_, index) => (
              <Skeleton key={index} className="h-24 rounded-none bg-white" />
            ))}
          </div>
        </section>
      </div>
      <ParentSectionSkeleton rows={2} />
    </>
  );
}

function CalendarEventList({
  groups,
  locale,
  respondingId,
  onRespond,
}: Readonly<{
  groups: Record<EventGroupKey, CalendarEvent[]>;
  locale: string;
  respondingId: string | null;
  onRespond: (
    event: CalendarEvent,
    status: "accepted" | "declined",
  ) => Promise<void>;
}>) {
  const t = useTranslations("parentCalendar");
  const groupOrder: EventGroupKey[] = ["thisWeek", "nextWeek", "later"];
  return (
    <section className="moto-content-surface h-full space-y-5 rounded-2xl border p-4 shadow-sm">
      {groupOrder.map((groupKey) => {
        const group = groups[groupKey];
        if (group.length === 0) return null;
        const headingId = `parent-calendar-${groupKey}`;
        return (
          <section key={groupKey} aria-labelledby={headingId}>
            <h2 id={headingId} className="text-sm font-semibold text-gray-700">
              {t(`groups.${groupKey}`)}
            </h2>
            <div className="mt-2 space-y-2">
              {group.map((event) => (
                <CalendarListEvent
                  key={event.id}
                  event={event}
                  locale={locale}
                  responding={respondingId === event.id}
                  onRespond={(status) => onRespond(event, status)}
                />
              ))}
            </div>
          </section>
        );
      })}
    </section>
  );
}

function CalendarListEvent({
  event,
  locale,
  responding,
  onRespond,
}: Readonly<{
  event: CalendarEvent;
  locale: string;
  responding: boolean;
  onRespond: (status: "accepted" | "declined") => Promise<void>;
}>) {
  const t = useTranslations("parentCalendar");
  const state = eventState(event);
  const colors = eventColors(event);
  const date = formatCalendarDate(event.start_date, locale, {
    weekday: "short",
    day: "numeric",
    month: "short",
  });
  const when = event.all_day
    ? t("whenAllDay", { date })
    : t("when", { date, time: eventTime(event, t("allDay")) });
  const answered =
    state === "accepted"
      ? t("answeredAccepted")
      : state === "declined"
        ? t("answeredDeclined")
        : null;
  const pending = state === "needsResponse";

  return (
    <article
      aria-label={event.title}
      data-rsvp-state={pending ? "pending" : undefined}
      className={`overflow-hidden rounded-l-none rounded-r-lg border border-l-4 bg-white ${pending ? "border-dashed border-gray-400" : "border-solid border-gray-200"}`}
      style={{
        borderLeftColor: colors.base,
        backgroundImage: pending ? pendingHatch(colors.base) : undefined,
      }}
    >
      <CalendarEventPopover
        event={event}
        locale={locale}
        responding={responding}
        onRespond={onRespond}
        renderTrigger={({ ref, open, panelId, toggle }) => (
          <Button
            ref={ref}
            type="button"
            variant="ghost"
            size="touch"
            onClick={toggle}
            aria-label={`${event.title}, ${eventTime(event, t("allDay"))}`}
            aria-expanded={open}
            aria-controls={open ? panelId : undefined}
            className="h-auto min-h-11 w-full justify-start rounded-none px-3 py-2 text-left font-normal hover:bg-gray-50 lg:min-h-10"
          >
            <span className="flex min-w-0 flex-col items-start">
              <span
                className="text-xs leading-5 font-medium tabular-nums"
                style={{ color: colors.strong }}
              >
                {when}
              </span>
              <span
                className={`text-[15px] leading-5 font-semibold text-gray-900 ${event.cancelled ? "line-through" : ""}`}
              >
                {event.title}
              </span>
              {event.student_name ? (
                <span className="text-xs leading-5 text-gray-500">
                  {t("forChild", { name: event.student_name })}
                </span>
              ) : null}
            </span>
          </Button>
        )}
      />

      {pending ? (
        <div className="flex flex-wrap items-center justify-between gap-2 px-3 pb-3">
          <span className="text-moto-orange-strong text-xs leading-5 font-medium">
            {t("responseRequired")}
          </span>
          <div className="ms-auto flex items-center gap-2">
            <Button
              type="button"
              variant="primary"
              size="md"
              isLoading={responding}
              onClick={() => void onRespond("accepted")}
            >
              {t("accept")}
            </Button>
            <Button
              type="button"
              variant="outline_danger"
              size="md"
              disabled={responding}
              onClick={() => void onRespond("declined")}
            >
              {t("decline")}
            </Button>
          </div>
        </div>
      ) : answered ? (
        <p className="flex items-center gap-1.5 px-3 pb-2.5 text-sm font-medium text-gray-600">
          <CheckCircleIcon
            size={16}
            weight="bold"
            style={{ color: colors.base }}
            aria-hidden="true"
          />
          {answered}
        </p>
      ) : state === "cancelled" ? (
        <p className="px-3 pb-2.5 text-sm font-medium text-gray-600">
          {t("cancelledByOgs")}
        </p>
      ) : null}
    </article>
  );
}

function CalendarMonthPanel({
  events,
  referenceDate,
  today,
  rangeEnd,
  locale,
  respondingId,
  canGoBack,
  canGoForward,
  onChangeMonth,
  onRespond,
}: Readonly<{
  events: readonly CalendarEvent[];
  referenceDate: Date;
  today: string;
  rangeEnd: string;
  locale: string;
  respondingId: string | null;
  canGoBack: boolean;
  canGoForward: boolean;
  onChangeMonth: (offset: number) => void;
  onRespond: (
    event: CalendarEvent,
    status: "accepted" | "declined",
  ) => Promise<void>;
}>) {
  const t = useTranslations("parentCalendar");
  const days = useMemo(() => monthDays(referenceDate), [referenceDate]);
  const monthLabel = new Intl.DateTimeFormat(locale, {
    month: "long",
    year: "numeric",
  }).format(referenceDate);
  return (
    <section
      data-testid="parent-calendar-month-grid"
      className="moto-content-surface h-full overflow-hidden rounded-2xl border shadow-sm"
      aria-labelledby="parent-calendar-month"
    >
      <div className="grid grid-cols-[2rem_minmax(0,1fr)_2rem] items-center gap-2 border-b border-gray-200 p-3 sm:p-4">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={t("previousMonth")}
          disabled={!canGoBack}
          onClick={() => onChangeMonth(-1)}
        >
          <CaretLeftIcon size={16} weight="bold" aria-hidden="true" />
        </Button>
        <h2
          id="parent-calendar-month"
          className="truncate text-center text-base font-semibold text-gray-900 sm:text-lg"
        >
          {monthLabel}
        </h2>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={t("nextMonth")}
          disabled={!canGoForward}
          onClick={() => onChangeMonth(1)}
        >
          <CaretRightIcon size={16} weight="bold" aria-hidden="true" />
        </Button>
      </div>
      <WeekdayHeader locale={locale} />
      <div className="grid grid-cols-7">
        {days.map((day) => {
          const dateISO = toISODate(day);
          const outsideMonth = day.getMonth() !== referenceDate.getMonth();
          const dayEvents = eventsForDay(events, dateISO);
          const visibleEvents = dayEvents.slice(0, MAX_VISIBLE_EVENTS);
          const moreCount = dayEvents.length - visibleEvents.length;
          const inLoadedRange = dateISO >= today && dateISO <= rangeEnd;
          return (
            <div
              key={dateISO}
              className={`min-h-14 border-r border-b border-gray-100 p-1 md:min-h-28 md:p-1.5 [&:nth-child(7n)]:border-r-0 ${outsideMonth ? "bg-gray-50/60" : "bg-white"}`}
            >
              <span
                aria-current={dateISO === today ? "date" : undefined}
                className={`inline-flex size-7 items-center justify-center rounded-full text-sm tabular-nums ${
                  dateISO === today
                    ? "bg-gray-900 text-white"
                    : outsideMonth || !inLoadedRange
                      ? "text-gray-400"
                      : "text-gray-700"
                }`}
              >
                {day.getDate()}
              </span>
              <div className="mt-0.5 flex min-h-2 justify-center gap-0.5 md:hidden">
                {dayEvents.slice(0, 3).map((event) => (
                  <span
                    key={event.id}
                    className={`size-1.5 rounded-full ${eventState(event) === "needsResponse" ? "ring-1 ring-offset-1" : ""}`}
                    style={{
                      backgroundColor: eventColors(event).base,
                      color: eventColors(event).base,
                    }}
                    aria-hidden="true"
                  />
                ))}
              </div>
              <div className="mt-1 hidden space-y-1 md:block">
                {visibleEvents.map((event) => (
                  <MonthEventButton
                    key={`${dateISO}-${event.id}`}
                    event={event}
                    locale={locale}
                    responding={respondingId === event.id}
                    onRespond={(status) => onRespond(event, status)}
                  />
                ))}
                {moreCount > 0 ? (
                  <span className="block px-1 text-[11px] font-medium text-gray-500">
                    {t("moreEvents", { count: moreCount })}
                  </span>
                ) : null}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function WeekdayHeader({ locale }: Readonly<{ locale: string }>) {
  const monday = new Date(2026, 7, 17);
  return (
    <div className="grid grid-cols-7 border-b border-gray-200 bg-gray-50">
      {Array.from({ length: 7 }, (_, index) => {
        const day = new Date(monday);
        day.setDate(monday.getDate() + index);
        return (
          <div
            key={index}
            className="px-1 py-2 text-center text-xs font-medium text-gray-500"
          >
            {new Intl.DateTimeFormat(locale, { weekday: "short" }).format(day)}
          </div>
        );
      })}
    </div>
  );
}

function MonthEventButton({
  event,
  locale,
  responding,
  onRespond,
}: Readonly<{
  event: CalendarEvent;
  locale: string;
  responding: boolean;
  onRespond: (status: "accepted" | "declined") => Promise<void>;
}>) {
  const t = useTranslations("parentCalendar");
  const colors = eventColors(event);
  const pending = eventState(event) === "needsResponse";
  const accessibleStatus = pending ? `, ${t("responseRequired")}` : "";
  return (
    <CalendarEventPopover
      event={event}
      locale={locale}
      responding={responding}
      onRespond={onRespond}
      renderTrigger={({ ref, open, panelId, toggle }) => (
        <Button
          ref={ref}
          type="button"
          variant="ghost"
          size="compact"
          onClick={toggle}
          aria-label={`${event.title}, ${eventTime(event, t("allDay"))}${accessibleStatus}`}
          aria-expanded={open}
          aria-controls={open ? panelId : undefined}
          data-rsvp-state={pending ? "pending" : undefined}
          className={`h-auto min-h-7 w-full justify-start !rounded-l-none rounded-r-md border-l-4 px-1.5 py-1 text-left text-[11px] leading-4 font-semibold text-gray-900 hover:brightness-95 ${pending ? "border border-dashed border-gray-400" : "border-solid"}`}
          style={{
            backgroundColor: colors.soft,
            backgroundImage: pending ? pendingHatch(colors.base) : undefined,
            borderLeftColor: colors.base,
          }}
        >
          <span className="line-clamp-2">{event.title}</span>
        </Button>
      )}
    />
  );
}

type EventPopoverTrigger = (props: {
  ref: RefObject<HTMLButtonElement | null>;
  open: boolean;
  panelId: string;
  toggle: () => void;
}) => ReactNode;

function CalendarEventPopover({
  event,
  locale,
  responding,
  onRespond,
  renderTrigger,
}: Readonly<{
  event: CalendarEvent;
  locale: string;
  responding: boolean;
  onRespond: (status: "accepted" | "declined") => Promise<void>;
  renderTrigger: EventPopoverTrigger;
}>) {
  const [open, setOpen] = useState(false);
  const t = useTranslations("parentCalendar");
  const isCompact = useMediaQuery(BELOW_LG);
  const mobileTriggerRef = useRef<HTMLButtonElement>(null);
  const mobilePanelId = useId();
  const state = eventState(event);
  const status: { label: string; tone: StatusBadgeTone } | null =
    state === "cancelled"
      ? { label: t("cancelledByOgs"), tone: "red" }
      : state === "needsResponse"
        ? { label: t("responseRequired"), tone: "orange" }
        : state === "accepted"
          ? { label: t("answeredAccepted"), tone: "green" }
          : state === "declined"
            ? { label: t("answeredDeclined"), tone: "gray" }
            : null;

  if (isCompact) {
    const close = () => {
      setOpen(false);
      mobileTriggerRef.current?.focus();
    };

    return (
      <>
        {renderTrigger({
          ref: mobileTriggerRef,
          open,
          panelId: mobilePanelId,
          toggle: () => setOpen((current) => !current),
        })}
        <Drawer
          open={open}
          onOpenChange={(nextOpen) => {
            setOpen(nextOpen);
            if (!nextOpen) mobileTriggerRef.current?.focus();
          }}
        >
          <DrawerContent
            id={mobilePanelId}
            data-mobile-calendar-drawer="true"
            data-testid="mobile-calendar-drawer"
            className="max-h-[calc(100dvh-env(safe-area-inset-top)-1rem)] overflow-hidden bg-white"
          >
            <DrawerHeader className="sr-only">
              <DrawerTitle>{event.title}</DrawerTitle>
              <DrawerDescription>{eventType(event, t)}</DrawerDescription>
            </DrawerHeader>
            <div className="min-h-0 scrollbar-thin overflow-y-auto overscroll-contain">
              <CalendarEventDetails
                event={event}
                locale={locale}
                responding={responding}
                status={status}
                state={state}
                mobile
                onClose={close}
                onRespond={onRespond}
              />
            </div>
          </DrawerContent>
        </Drawer>
      </>
    );
  }

  return (
    <AnchoredPopover
      open={open}
      onOpenChange={setOpen}
      ariaLabel={event.title}
      preferredWidth={380}
      className="overflow-hidden p-0"
      renderTrigger={renderTrigger}
    >
      {({ close }) => (
        <CalendarEventDetails
          event={event}
          locale={locale}
          responding={responding}
          status={status}
          state={state}
          onClose={close}
          onRespond={onRespond}
        />
      )}
    </AnchoredPopover>
  );
}

function CalendarEventDetails({
  event,
  locale,
  responding,
  status,
  state,
  mobile = false,
  onClose,
  onRespond,
}: Readonly<{
  event: CalendarEvent;
  locale: string;
  responding: boolean;
  status: { label: string; tone: StatusBadgeTone } | null;
  state: EventState;
  mobile?: boolean;
  onClose: () => void;
  onRespond: (status: "accepted" | "declined") => Promise<void>;
}>) {
  const t = useTranslations("parentCalendar");

  return (
    <>
      <div className="flex items-start justify-between gap-3 border-b border-gray-100 px-4 py-3">
        <div className="min-w-0">
          <p className="text-xs font-medium text-gray-500">
            {eventType(event, t)}
          </p>
          <h3
            className={`mt-0.5 text-base font-semibold text-pretty text-gray-950 ${event.cancelled ? "line-through" : ""}`}
          >
            {event.title}
          </h3>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={t("close")}
          className="shrink-0"
          onClick={onClose}
        >
          <XIcon size={16} weight="bold" aria-hidden="true" />
        </Button>
      </div>

      <div className="space-y-4 px-4 py-3">
        {status ? (
          <StatusBadge label={status.label} tone={status.tone} />
        ) : null}
        <div className="space-y-2.5 text-sm text-gray-700">
          <DetailRow
            icon={<CalendarDotsIcon size={16} />}
            label={formatLocalizedDate(event.start_date, locale)}
          />
          <DetailRow
            icon={<ClockIcon size={16} />}
            label={eventTime(event, t("allDay"))}
          />
          {event.student_name ? (
            <DetailRow
              icon={<UserIcon size={16} />}
              label={t("forChild", { name: event.student_name })}
            />
          ) : null}
          {event.location ? (
            <DetailRow icon={<MapPinIcon size={16} />} label={event.location} />
          ) : null}
          {event.school_name ? (
            <DetailRow
              icon={<BuildingsIcon size={16} />}
              label={event.school_name}
            />
          ) : null}
        </div>
        {event.description ? (
          <p className="border-t border-gray-100 pt-3 text-sm leading-6 text-pretty whitespace-pre-line text-gray-700">
            {event.description}
          </p>
        ) : null}
      </div>
      {state === "needsResponse" ? (
        <div
          className={`flex items-center justify-end gap-2 border-t border-gray-100 px-4 py-3 ${mobile ? "pb-[calc(0.75rem+env(safe-area-inset-bottom))]" : ""}`}
        >
          <Button
            type="button"
            variant="primary"
            size="md"
            isLoading={responding}
            onClick={() => void onRespond("accepted")}
          >
            {t("accept")}
          </Button>
          <Button
            type="button"
            variant="outline_danger"
            size="md"
            disabled={responding}
            onClick={() => void onRespond("declined")}
          >
            {t("decline")}
          </Button>
        </div>
      ) : null}
    </>
  );
}

function DetailRow({
  icon,
  label,
}: Readonly<{ icon: React.ReactNode; label: string }>) {
  return (
    <div className="flex items-center gap-2">
      <span
        className="shrink-0"
        style={{ color: MOTO_COLOR_PALETTE.blue.base }}
        aria-hidden="true"
      >
        {icon}
      </span>
      <span>{label}</span>
    </div>
  );
}
