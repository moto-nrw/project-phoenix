"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import { ChildDayCard } from "~/components/parent/child/child-day-card";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  NewsDetailModal,
  isOpenPoll,
} from "~/components/parent/news/news-components";
import { TodoList, type TodoItem } from "~/components/parent/start/todo-list";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { berlinTodayISO, parseISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  UNKNOWN_CHILD_TODAY,
  fetchParentProfile,
  getChildFeatures,
  getChildToday,
  listAnnouncements,
  listMessageThreads,
  listMyChildren,
  type Child,
  type ChildFeatures,
  type ChildToday,
  type ParentAnnouncement,
  type ThreadSummary,
} from "~/lib/parent-api";
import { parentPath } from "~/lib/parent-url";
import {
  getParentCalendar,
  type CalendarEvent,
} from "~/lib/personal-calendar-api";
import { useShellAuth } from "~/lib/shell-auth-context";

const logger = createLogger({ component: "ParentStartPage" });

/** Wie weit nach vorn Termineinladungen fuer "Zu erledigen" gesucht werden. */
const APPOINTMENT_LOOKAHEAD_DAYS = 90;

const TODO_TIME_FORMATTERS = new Map<string, Intl.DateTimeFormat>();
const TODO_DATE_FORMATTERS = new Map<string, Intl.DateTimeFormat>();

function todoTimeFormatter(locale: string): Intl.DateTimeFormat {
  const cached = TODO_TIME_FORMATTERS.get(locale);
  if (cached) return cached;
  const formatter = new Intl.DateTimeFormat(locale, {
    timeZone: "Europe/Berlin",
    hour: "2-digit",
    minute: "2-digit",
  });
  TODO_TIME_FORMATTERS.set(locale, formatter);
  return formatter;
}

function todoDateFormatter(
  locale: string,
  includeYear: boolean,
): Intl.DateTimeFormat {
  const key = `${locale}:${includeYear ? "year" : "short"}`;
  const cached = TODO_DATE_FORMATTERS.get(key);
  if (cached) return cached;
  const formatter = new Intl.DateTimeFormat(locale, {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: includeYear ? "2-digit" : undefined,
  });
  TODO_DATE_FORMATTERS.set(key, formatter);
  return formatter;
}

function formatTodoTimestamp(
  iso: string | undefined,
  locale: string,
  todayLabel: string,
  now: Date = new Date(),
): TodoItem["meta"] {
  if (!iso) return undefined;
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return undefined;

  const dateKey = berlinTodayISO(date);
  const todayKey = berlinTodayISO(now);
  const time = todoTimeFormatter(locale).format(date);

  if (dateKey === todayKey) return { date: todayLabel, time };

  const includeYear = dateKey.slice(0, 4) !== todayKey.slice(0, 4);
  const dateLabel = todoDateFormatter(locale, includeYear).format(date);
  return { date: dateLabel, time };
}

function formatAppointmentTimestamp(
  event: CalendarEvent,
  locale: string,
  todayLabel: string,
  allDayLabel: string,
): TodoItem["meta"] {
  const today = berlinTodayISO();
  const includeYear = event.start_date.slice(0, 4) !== today.slice(0, 4);
  const date =
    event.start_date === today
      ? todayLabel
      : todoDateFormatter(locale, includeYear).format(
          new Date(`${event.start_date}T12:00:00Z`),
        );
  const time = event.all_day ? allDayLabel : event.start_time.slice(0, 5);
  return { date, time };
}

/**
 * Die Stunde in Berlin, unabhaengig davon, wo das Geraet steht. Ein Elternteil
 * im Urlaub soll nicht "Guten Abend" lesen, waehrend die OGS Vormittag hat.
 */
function berlinHour(now: Date = new Date()): number {
  const hour = new Intl.DateTimeFormat("en-GB", {
    timeZone: "Europe/Berlin",
    hour: "2-digit",
    hour12: false,
  }).format(now);
  return Number.parseInt(hour, 10);
}

function greetingKey(hour: number): "morning" | "day" | "evening" {
  if (hour < 11) return "morning";
  if (hour < 18) return "day";
  return "evening";
}

interface StartData {
  readonly children: readonly Child[];
  readonly features: Readonly<Record<string, ChildFeatures>>;
  readonly today: Readonly<Record<string, ChildToday>>;
  readonly firstName: string;
}

const EMPTY_DATA: StartData = {
  children: [],
  features: {},
  today: {},
  firstName: "",
};

export function ParentStartPage() {
  const t = useTranslations("parentStart");
  const { profile } = useShellAuth();
  const [data, setData] = useState<StartData>(EMPTY_DATA);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  const load = useCallback(async () => {
    try {
      const [children, accountProfile] = await Promise.all([
        listMyChildren(),
        fetchParentProfile().catch(() => undefined),
      ]);
      const perChild = await Promise.all(
        children.map(async (child) => {
          const [features, today] = await Promise.all([
            getChildFeatures(child.student_id).catch(() => undefined),
            getChildToday(child.student_id).catch(() => UNKNOWN_CHILD_TODAY),
          ]);
          return [child.student_id, features, today] as const;
        }),
      );
      setData({
        children,
        features: Object.fromEntries(
          perChild
            .filter(([, features]) => features !== undefined)
            .map(([id, features]) => [id, features as ChildFeatures]),
        ),
        today: Object.fromEntries(perChild.map(([id, , today]) => [id, today])),
        firstName: accountProfile?.first_name?.trim() ?? "",
      });
      setFailed(false);
    } catch (err) {
      logger.warn("parent_start_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setFailed(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    const refresh = () => void load();
    window.addEventListener("parent-child-status-refresh", refresh);
    window.addEventListener("focus", refresh);
    return () => {
      window.removeEventListener("parent-child-status-refresh", refresh);
      window.removeEventListener("focus", refresh);
    };
  }, [load]);

  const greeting = useMemo(() => {
    const key = greetingKey(berlinHour());
    const name = data.firstName || profile?.firstName?.trim();
    return name ? t(`greeting.${key}`, { name }) : t(`greeting.${key}Plain`);
  }, [data.firstName, profile?.firstName, t]);

  return (
    <ParentPage>
      <ParentPageHeader
        kicker={t("kicker")}
        title={greeting}
        description={t("subtitle")}
        prominent
      />

      {/* Untereinander, beide ueber die volle Inhaltsbreite. Zweispaltig mit
          ungleichen Spalten las sich als Fehler: der schmale Block daneben
          sah aus, als fehle ihm etwas. Nebeneinander gilt nur bei exakt
          gleicher Breite; innerhalb eines Abschnitts liegen die Kinderkarten
          weiter in einem auto-fit-Raster. */}
      <div className="space-y-5">
        <StartTodoSection />

        <div className="space-y-4">
          {failed && <Alert type="error" message={t("loadError")} />}

          {loading ? (
            <StartChildCardSkeleton />
          ) : data.children.length === 0 && !failed ? (
            <p className="moto-content-surface rounded-2xl border p-5 text-sm leading-6 text-gray-600 shadow-sm backdrop-blur-md">
              {t("noChildren")}
            </p>
          ) : (
            /* auto-fit statt fester Spaltenzahl: ein Kind fuellt die Breite,
               zwei stehen nebeneinander, drei brechen um. Vorher blieb bei
               einem Kind die halbe Zeile leer. */
            <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,20rem),1fr))] gap-4">
              {data.children.map((child) => (
                <StartChildCard
                  key={child.student_id}
                  child={child}
                  today={data.today[child.student_id] ?? UNKNOWN_CHILD_TODAY}
                  features={data.features[child.student_id]}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </ParentPage>
  );
}

function StartChildCard({
  child,
  today,
  features,
}: Readonly<{
  child: Child;
  today: ChildToday;
  features?: ChildFeatures;
}>) {
  const pickupAside =
    today.at_ogs === true && today.pickup_time ? (
      <StartPickupSummary time={today.pickup_time} />
    ) : undefined;

  return (
    <ChildDayCard
      child={{
        studentId: child.student_id,
        firstName: child.first_name,
        lastName: child.last_name,
        schoolClass: child.school_class,
      }}
      today={today}
      features={features}
      statusAside={pickupAside}
      actionDisplay="compact"
    />
  );
}

function StartPickupSummary({ time }: Readonly<{ time: string }>) {
  const t = useTranslations("parentChild");

  return (
    <div className="flex min-w-0 items-start gap-3">
      <MotoConceptIcon
        concept="pickup"
        size={28}
        className="mt-0.5 shrink-0"
        aria-hidden="true"
      />
      <div className="min-w-0">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {t("today.pickupLabel")}
        </p>
        <p className="mt-1 text-sm leading-6 font-medium text-gray-900">
          {t("today.pickup", { time })}
        </p>
      </div>
    </div>
  );
}

function StartChildCardSkeleton() {
  const t = useTranslations("parentStart");

  return (
    <article
      role="status"
      data-testid="parent-start-child-skeleton"
      className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm"
    >
      <span className="sr-only">{t("childrenLoading")}</span>
      <div aria-hidden="true" className="space-y-5 p-5 sm:p-6">
        <div className="flex min-w-0 items-center justify-between gap-3">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <Skeleton className="size-11 shrink-0 rounded-xl" />
            <div className="min-w-0 flex-1">
              <Skeleton className="h-5 w-40 max-w-3/4" />
              <Skeleton className="mt-2 h-4 w-20" />
            </div>
          </div>
          <Skeleton className="h-11 w-28 shrink-0 rounded-lg" />
        </div>

        <div className="flex min-w-0 items-start gap-3 rounded-xl bg-gray-50 p-4">
          <Skeleton className="size-8 shrink-0 rounded-lg" />
          <div className="min-w-0 flex-1">
            <Skeleton className="h-3 w-12" />
            <Skeleton className="mt-2 h-7 w-44 max-w-4/5" />
            <Skeleton className="mt-2 h-4 w-32 max-w-2/3" />
          </div>
        </div>
      </div>
    </article>
  );
}

interface TodoSources {
  readonly announcements: readonly ParentAnnouncement[];
  readonly threads: readonly ThreadSummary[];
  readonly appointments: readonly CalendarEvent[];
}

const EMPTY_SOURCES: TodoSources = {
  announcements: [],
  threads: [],
  appointments: [],
};

/**
 * Laedt und baut "Zu erledigen". Getrennt vom Kinderteil, damit ein hakendes
 * Neuigkeiten-Backend die Tageskarten nicht aufhaelt (#2308).
 */
function StartTodoSection() {
  const t = useTranslations("parentStart");
  const locale = useLocale();
  const [sources, setSources] = useState<TodoSources>(EMPTY_SOURCES);
  const [loaded, setLoaded] = useState(false);
  const [partialFailure, setPartialFailure] = useState(false);
  const [openAnnouncementId, setOpenAnnouncementId] = useState<string | null>(
    null,
  );

  const load = useCallback(async () => {
    const from = parseISODate(berlinTodayISO());
    const to = new Date(from);
    to.setDate(to.getDate() + APPOINTMENT_LOOKAHEAD_DAYS);
    const [announcements, threads, calendar] = await Promise.allSettled([
      listAnnouncements(),
      listMessageThreads(),
      getParentCalendar(from, to),
    ]);
    let failed = false;
    const next = { ...EMPTY_SOURCES } as {
      announcements: readonly ParentAnnouncement[];
      threads: readonly ThreadSummary[];
      appointments: readonly CalendarEvent[];
    };
    if (announcements.status === "fulfilled") {
      next.announcements = announcements.value;
    } else {
      failed = true;
      logger.warn("parent_start_news_failed", {
        error: String(announcements.reason),
      });
    }
    if (threads.status === "fulfilled") {
      next.threads = threads.value;
    } else {
      failed = true;
      logger.warn("parent_start_threads_failed", {
        error: String(threads.reason),
      });
    }
    if (calendar.status === "fulfilled") {
      next.appointments = calendar.value.events;
    } else {
      failed = true;
      logger.warn("parent_start_calendar_failed", {
        error: String(calendar.reason),
      });
    }
    setSources(next);
    setPartialFailure(failed);
    setLoaded(true);
  }, []);

  useEffect(() => {
    void load();
    const refresh = () => void load();
    window.addEventListener("parent-threads-refresh", refresh);
    window.addEventListener("parent-news-unread-refresh", refresh);
    window.addEventListener("focus", refresh);
    return () => {
      window.removeEventListener("parent-threads-refresh", refresh);
      window.removeEventListener("parent-news-unread-refresh", refresh);
      window.removeEventListener("focus", refresh);
    };
  }, [load]);

  const applyAnnouncementPatch = useCallback(
    (id: string, patch: Partial<ParentAnnouncement>) => {
      setSources((prev) => ({
        ...prev,
        announcements: prev.announcements.map((item) =>
          item.id === id ? { ...item, ...patch } : item,
        ),
      }));
    },
    [],
  );

  const items = useMemo(
    () => buildTodoItems(sources, locale, t, setOpenAnnouncementId),
    [sources, locale, t],
  );

  const openAnnouncement =
    sources.announcements.find((item) => item.id === openAnnouncementId) ??
    null;

  if (!loaded) {
    return <StartTodoSkeleton />;
  }

  if (partialFailure && items.length === 0) {
    return <Alert type="warning" message={t("todo.partialLoadError")} />;
  }

  return (
    <>
      <TodoList items={items} />
      {partialFailure && (
        <Alert type="warning" message={t("todo.partialLoadError")} />
      )}
      {openAnnouncement && (
        <NewsDetailModal
          item={openAnnouncement}
          onClose={() => setOpenAnnouncementId(null)}
          onUpdated={applyAnnouncementPatch}
          onStale={() => void load()}
        />
      )}
    </>
  );
}

function StartTodoSkeleton() {
  const t = useTranslations("parentStart");

  return (
    <section
      role="status"
      data-testid="parent-start-todo-skeleton"
      className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md"
    >
      <span className="sr-only">{t("todo.loading")}</span>
      <div aria-hidden="true">
        <Skeleton className="h-7 w-36" />
        <Skeleton className="mt-1 h-6 w-72 max-w-full" />
      </div>
    </section>
  );
}

type StartTranslator = ReturnType<typeof useTranslations<"parentStart">>;

/**
 * Reihenfolge nach Dringlichkeit: offene Umfragen (eine Frist laeuft), dann
 * Termineinladungen (die OGS wartet auf eine Antwort), dann ungelesene
 * Nachrichten, zuletzt die uebrigen Aushaenge.
 */
function buildTodoItems(
  sources: TodoSources,
  locale: string,
  t: StartTranslator,
  openAnnouncement: (id: string) => void,
): TodoItem[] {
  const openAnnouncements = sources.announcements.filter(
    (item) =>
      !item.read ||
      isOpenPoll(item) ||
      (item.requires_acknowledgement && !item.acknowledged),
  );
  const polls = openAnnouncements.filter(isOpenPoll);
  const notices = openAnnouncements.filter((item) => !isOpenPoll(item));

  const appointments = sources.appointments
    .filter(
      (event) =>
        event.source === "appointment" &&
        event.can_respond &&
        event.response_status === "pending" &&
        event.cancelled !== true,
    )
    .sort((a, b) => a.start_date.localeCompare(b.start_date));

  const threads = sources.threads
    .filter((thread) => thread.unread > 0)
    .sort(
      (a, b) =>
        new Date(b.last_message_at ?? 0).getTime() -
        new Date(a.last_message_at ?? 0).getTime(),
    );

  return [
    ...polls.map((item) =>
      todoFromAnnouncement(item, locale, t, true, openAnnouncement),
    ),
    ...appointments.map((event) => todoFromAppointment(event, locale, t)),
    ...threads.map((thread) => todoFromThread(thread, locale, t)),
    ...notices.map((item) =>
      todoFromAnnouncement(item, locale, t, false, openAnnouncement),
    ),
  ];
}

function todoFromAnnouncement(
  item: ParentAnnouncement,
  locale: string,
  t: StartTranslator,
  poll: boolean,
  openAnnouncement: (id: string) => void,
): TodoItem {
  const needsAcknowledgement =
    item.requires_acknowledgement && !item.acknowledged;
  return {
    key: `announcement-${item.id}`,
    concept: poll
      ? "polls"
      : needsAcknowledgement
        ? "confirmations"
        : "parentMessages",
    title: item.title,
    meta: formatTodoTimestamp(item.published_at, locale, t("todo.today")),
    unread: !item.read,
    context: poll
      ? t("todo.pollContext", { school: item.school_name })
      : needsAcknowledgement
        ? t("todo.ackContext", { school: item.school_name })
        : t("todo.newsContext", { school: item.school_name }),
    // Kein Ziel: der Aushang oeffnet sich an Ort und Stelle, damit Lesen und
    // Antworten die Startseite nicht verlassen.
    onSelect: () => openAnnouncement(item.id),
  };
}

function todoFromAppointment(
  event: CalendarEvent,
  locale: string,
  t: StartTranslator,
): TodoItem {
  return {
    key: `appointment-${event.id}`,
    concept: "calendar",
    title: event.title,
    meta: formatAppointmentTimestamp(
      event,
      locale,
      t("todo.today"),
      t("todo.allDay"),
    ),
    context: event.student_name
      ? t("todo.appointmentContext", { name: event.student_name })
      : t("todo.appointmentContextPlain"),
    href: parentPath(
      `/parents/calendar?date=${encodeURIComponent(event.start_date)}`,
    ),
  };
}

function todoFromThread(
  thread: ThreadSummary,
  locale: string,
  t: StartTranslator,
): TodoItem {
  return {
    key: `thread-${thread.thread_id}`,
    concept: "parentConversations",
    title:
      thread.unread === 1
        ? t("todo.messageTitleOne")
        : t("todo.messageTitleMany", { count: thread.unread }),
    context: t("todo.messageContext", { name: thread.student_name }),
    meta: formatTodoTimestamp(thread.last_message_at, locale, t("todo.today")),
    unread: true,
    href: parentPath(`/parents/messages/${thread.student_id}`),
  };
}
