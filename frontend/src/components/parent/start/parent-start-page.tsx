"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import { ChildDayCard } from "~/components/parent/child/child-day-card";
import {
  NewsDetailModal,
  isOpenPoll,
} from "~/components/parent/news/news-components";
import { TodoList, type TodoItem } from "~/components/parent/start/todo-list";
import { berlinTodayISO, parseISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { formatLocalizedDate } from "~/lib/localized-date-format";
import {
  UNKNOWN_CHILD_TODAY,
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
}

const EMPTY_DATA: StartData = { children: [], features: {}, today: {} };

export function ParentStartPage() {
  const t = useTranslations("parentStart");
  const { profile } = useShellAuth();
  const [data, setData] = useState<StartData>(EMPTY_DATA);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  const load = useCallback(async () => {
    try {
      const children = await listMyChildren();
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
    const name = profile?.firstName?.trim();
    return name ? t(`greeting.${key}`, { name }) : t(`greeting.${key}Plain`);
  }, [profile?.firstName, t]);

  return (
    <div className="space-y-5">
      <h1 className="text-[30px] leading-tight font-extrabold tracking-tight text-balance text-gray-900">
        {greeting}
      </h1>

      {/* Ab 1024 px zwei Spalten: links, was zu tun ist, rechts die Kinder.
          Sonst bliebe die rechte Bildschirmhaelfte leer, und das offene
          Zeug rutschte auf breiten Schirmen weit nach oben weg. */}
      <div className="space-y-5 lg:grid lg:grid-cols-[minmax(0,22rem)_minmax(0,1fr)] lg:items-start lg:gap-6 lg:space-y-0">
        <div className="space-y-5">
          <StartTodoSection />
        </div>

        <div className="space-y-4">
          {failed && <Alert type="error" message={t("loadError")} />}

          {loading ? (
            <Skeleton className="h-56 w-full rounded-2xl" />
          ) : data.children.length === 0 && !failed ? (
            <p className="rounded-2xl border border-gray-200 bg-white p-5 text-[17px] text-gray-500 shadow-sm">
              {t("noChildren")}
            </p>
          ) : (
            /* auto-fit statt fester Spaltenzahl: ein Kind fuellt die Breite,
               zwei stehen nebeneinander, drei brechen um. Vorher blieb bei
               einem Kind die halbe Zeile leer. */
            <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,20rem),1fr))] gap-4">
              {data.children.map((child) => (
                <ChildDayCard
                  key={child.student_id}
                  child={{
                    studentId: child.student_id,
                    firstName: child.first_name,
                    lastName: child.last_name,
                    schoolClass: child.school_class,
                  }}
                  today={data.today[child.student_id] ?? UNKNOWN_CHILD_TODAY}
                  features={data.features[child.student_id]}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
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
    return <Skeleton className="h-32 w-full rounded-2xl" />;
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
      todoFromAnnouncement(item, t, true, openAnnouncement),
    ),
    ...appointments.map((event) => todoFromAppointment(event, locale, t)),
    ...threads.map((thread) => todoFromThread(thread, t)),
    ...notices.map((item) =>
      todoFromAnnouncement(item, t, false, openAnnouncement),
    ),
  ];
}

function todoFromAnnouncement(
  item: ParentAnnouncement,
  t: StartTranslator,
  poll: boolean,
  openAnnouncement: (id: string) => void,
): TodoItem {
  return {
    key: `announcement-${item.id}`,
    // Eine offene Umfrage wartet auf eine Antwort und traegt deshalb den
    // Aufmerksamkeits-Ton von "announcements"; ein gelesener Aushang ist
    // reine Information und traegt den blauen Ton von "news".
    concept: poll ? "announcements" : "news",
    title: item.title,
    context: poll
      ? t("todo.pollContext", { school: item.school_name })
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
  const date = formatLocalizedDate(event.start_date, locale);
  return {
    key: `appointment-${event.id}`,
    concept: "calendar",
    title: event.title,
    context: event.student_name
      ? t("todo.appointmentContext", { name: event.student_name, date })
      : t("todo.appointmentContextPlain", { date }),
    href: parentPath(
      `/parents/calendar?date=${encodeURIComponent(event.start_date)}`,
    ),
  };
}

function todoFromThread(thread: ThreadSummary, t: StartTranslator): TodoItem {
  return {
    key: `thread-${thread.thread_id}`,
    concept: "parentConversations",
    title: t("todo.messageTitle"),
    context: t("todo.messageContext", { name: thread.student_name }),
    href: parentPath(`/parents/messages/${thread.student_id}`),
  };
}
