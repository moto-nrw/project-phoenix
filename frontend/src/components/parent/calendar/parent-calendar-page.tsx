"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { CalendarSubscribePanel } from "~/components/calendar/calendar-subscribe-panel";
import {
  CheckCircle,
  Prohibit,
} from "~/components/parent/shell/parent-icons";
import { useToast } from "~/contexts/ToastContext";
import { berlinTodayISO, parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { formatLocalizedDate } from "~/lib/localized-date-format";
import {
  getParentCalendar,
  respondParentCalendar,
  type CalendarEvent,
} from "~/lib/personal-calendar-api";

const logger = createLogger({ component: "ParentCalendarPage" });

/** Wie weit nach vorn die Terminliste blickt. */
const LOOKAHEAD_DAYS = 90;

/**
 * Der Kalender der Eltern-App als chronologische Terminliste (Entscheidung
 * E10). `PersonalCalendar` des Personal-Portals wird hier nicht mehr
 * verwendet: ein Wochenraster mit Drag-Zielen ist ein Planungswerkzeug, kein
 * Elternkalender.
 *
 * Zu- und Absage stehen in der Zeile, nicht hinter einem Detailfenster. Ein
 * Monatsraster erscheint erst ab 1024 px als Zusatz neben der Liste, nie auf
 * dem Handy.
 */

type Group = "thisWeek" | "nextWeek" | "later";

/** Der Zustand eines Termins, aus dem Farbe UND Text folgen. */
type EventState =
  "cancelled" | "needsResponse" | "accepted" | "declined" | "info";

function eventState(event: CalendarEvent): EventState {
  if (event.cancelled) return "cancelled";
  if (event.can_respond && event.response_status === "pending")
    return "needsResponse";
  if (event.response_status === "accepted") return "accepted";
  if (event.response_status === "declined") return "declined";
  return "info";
}

const STATE_EDGE: Record<EventState, string> = {
  cancelled: "bg-parent-red",
  needsResponse: "bg-moto-orange",
  accepted: "bg-moto-green",
  declined: "bg-gray-300",
  info: "bg-moto-blue",
};

const STATE_TEXT: Record<EventState, string> = {
  cancelled: "text-parent-red-strong",
  needsResponse: "text-moto-orange-strong",
  accepted: "text-moto-green-strong",
  declined: "text-gray-500",
  info: "text-moto-blue-strong",
};

/** Montag der Woche, in der `iso` liegt, als "YYYY-MM-DD". */
function weekStartISO(iso: string): string {
  const date = parseISODate(iso);
  const shift = (date.getDay() + 6) % 7;
  date.setDate(date.getDate() - shift);
  return toISODate(date);
}

function addDaysISO(iso: string, days: number): string {
  const date = parseISODate(iso);
  date.setDate(date.getDate() + days);
  return toISODate(date);
}

function groupOf(eventDate: string, today: string): Group {
  const thisWeekStart = weekStartISO(today);
  const nextWeekStart = addDaysISO(thisWeekStart, 7);
  const laterStart = addDaysISO(thisWeekStart, 14);
  if (eventDate < nextWeekStart) return "thisWeek";
  if (eventDate < laterStart) return "nextWeek";
  return "later";
}

const GROUP_ORDER: readonly Group[] = ["thisWeek", "nextWeek", "later"];

export function ParentCalendarPage() {
  const t = useTranslations("parentCalendar");
  const locale = useLocale();
  const toast = useToast();
  const searchParams = useSearchParams();
  const focusDate = searchParams.get("date");
  const [events, setEvents] = useState<CalendarEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [respondingId, setRespondingId] = useState<string | null>(null);
  const today = useMemo(() => berlinTodayISO(), []);

  // Latest-wins: ein Nachladen nach einer Zusage darf nicht von einem aelteren
  // Lauf ueberschrieben werden.
  const seqRef = useRef(0);

  const load = useCallback(
    async (opts?: { silent?: boolean }) => {
      const seq = ++seqRef.current;
      if (!opts?.silent) setLoading(true);
      try {
        const from = parseISODate(today);
        const to = parseISODate(addDaysISO(today, LOOKAHEAD_DAYS));
        const response = await getParentCalendar(from, to);
        if (seq !== seqRef.current) return;
        setEvents(response.events);
        setError(false);
      } catch (err) {
        if (seq !== seqRef.current) return;
        logger.warn("parent_calendar_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!opts?.silent) setError(true);
      } finally {
        if (seq === seqRef.current && !opts?.silent) setLoading(false);
      }
    },
    [today],
  );

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
      await load({ silent: true });
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

  const sorted = useMemo(
    () =>
      [...events].sort(
        (a, b) =>
          a.start_date.localeCompare(b.start_date) ||
          (a.start_time ?? "").localeCompare(b.start_time ?? ""),
      ),
    [events],
  );

  const grouped = useMemo(() => {
    const map: Record<Group, CalendarEvent[]> = {
      thisWeek: [],
      nextWeek: [],
      later: [],
    };
    for (const event of sorted)
      map[groupOf(event.start_date, today)].push(event);
    return map;
  }, [sorted, today]);

  if (loading) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-10 w-48 rounded-xl" />
        <Skeleton className="h-40 w-full rounded-2xl" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Die Kopfzeile nennt die Seite bereits sichtbar; ein zweiter Titel
          darunter waere dieselbe Zeile zweimal. Fuer die Sprachausgabe muss
          die Seite trotzdem eine Ueberschrift haben. */}
      <h1 className="sr-only">{t("title")}</h1>

      {error && <Alert type="error" message={t("loadError")} />}

      <div className="lg:flex lg:items-start lg:gap-6">
        <div className="min-w-0 flex-1 space-y-6">
          {sorted.length === 0 && !error ? (
            <p className="rounded-2xl border border-gray-200 bg-white p-5 text-[17px] text-gray-600 shadow-sm">
              {t("empty")}
            </p>
          ) : (
            GROUP_ORDER.filter((group) => grouped[group].length > 0).map(
              (group) => (
                <section key={group} aria-labelledby={`calendar-${group}`}>
                  <h2
                    id={`calendar-${group}`}
                    className="mb-2 text-[20px] font-semibold text-gray-900"
                  >
                    {t(`groups.${group}`)}
                  </h2>
                  <ul className="space-y-2">
                    {grouped[group].map((event) => (
                      <li key={event.id}>
                        <EventRow
                          event={event}
                          locale={locale}
                          highlighted={event.start_date === focusDate}
                          responding={respondingId === event.id}
                          onRespond={respond}
                        />
                      </li>
                    ))}
                  </ul>
                </section>
              ),
            )
          )}
        </div>

        {/* Nur ab 1024 px. Auf dem Handy waere ein Raster mit 42 winzigen
            Feldern kein Gewinn gegenueber der Liste. */}
        <aside className="hidden w-72 shrink-0 lg:block">
          <MonthGrid events={sorted} today={today} locale={locale} />
        </aside>
      </div>

      {/* Das Abo bleibt erreichbar, steht aber am Ende: ein Einmalvorgang. */}
      <CalendarSubscribePanel />
    </div>
  );
}

function EventRow({
  event,
  locale,
  highlighted,
  responding,
  onRespond,
}: Readonly<{
  event: CalendarEvent;
  locale: string;
  highlighted: boolean;
  responding: boolean;
  onRespond: (
    event: CalendarEvent,
    status: "accepted" | "declined",
  ) => Promise<void>;
}>) {
  const t = useTranslations("parentCalendar");
  const state = eventState(event);
  const when = event.all_day
    ? t("whenAllDay", { date: formatLocalizedDate(event.start_date, locale) })
    : t("when", {
        date: formatLocalizedDate(event.start_date, locale),
        time: event.start_time,
      });

  return (
    <article
      className={`relative overflow-hidden rounded-2xl border bg-white shadow-sm ${
        highlighted ? "border-moto-blue" : "border-gray-200"
      }`}
    >
      <span
        className={`absolute inset-y-0 left-0 w-1 ${STATE_EDGE[state]}`}
        aria-hidden="true"
      />
      <div className="py-4 pr-4 pl-5 sm:pl-6">
        <p className={`text-[15px] font-medium ${STATE_TEXT[state]}`}>{when}</p>
        <p className="mt-0.5 text-[17px] font-semibold text-gray-900">
          {event.title}
        </p>
        {event.student_name && (
          <p className="mt-0.5 text-[15px] text-gray-600">
            {t("forChild", { name: event.student_name })}
          </p>
        )}
        {event.location && (
          <p className="mt-0.5 text-[15px] text-gray-600">{event.location}</p>
        )}

        {state === "cancelled" && (
          <p className="text-parent-red-strong mt-2 flex items-center gap-1.5 text-[15px] font-medium">
            <Prohibit size={18} aria-hidden="true" />
            {t("cancelledByOgs")}
          </p>
        )}

        {state === "needsResponse" && (
          <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2">
            <Button
              type="button"
              variant="primary"
              size="touch"
              className="w-full"
              isLoading={responding}
              onClick={() => void onRespond(event, "accepted")}
            >
              {t("accept")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="touch"
              className="w-full"
              disabled={responding}
              onClick={() => void onRespond(event, "declined")}
            >
              {t("decline")}
            </Button>
          </div>
        )}

        {state === "accepted" && (
          <p className="text-moto-green-strong mt-2 flex items-center gap-1.5 text-[15px] font-medium">
            <CheckCircle size={18} weight="fill" aria-hidden="true" />
            {t("answeredAccepted")}
          </p>
        )}
        {state === "declined" && (
          <p className="mt-2 flex items-center gap-1.5 text-[15px] font-medium text-gray-600">
            <Prohibit size={18} aria-hidden="true" />
            {t("answeredDeclined")}
          </p>
        )}
      </div>
    </article>
  );
}

/**
 * Ein ruhiges Monatsraster als Zusatz auf breiten Schirmen. Reine Uebersicht:
 * ein Punkt markiert einen Tag mit Termin, angeklickt wird nichts. Die Liste
 * daneben bleibt der Ort, an dem etwas passiert.
 */
function MonthGrid({
  events,
  today,
  locale,
}: Readonly<{
  events: readonly CalendarEvent[];
  today: string;
  locale: string;
}>) {
  const t = useTranslations("parentCalendar");
  const [year, month] = today.split("-").map(Number) as [number, number];
  const first = new Date(year, month - 1, 1);
  const leading = (first.getDay() + 6) % 7;
  const daysInMonth = new Date(year, month, 0).getDate();
  const withEvents = new Set(events.map((event) => event.start_date));
  const monthLabel = new Intl.DateTimeFormat(locale, {
    month: "long",
    year: "numeric",
  }).format(first);

  const cells: (string | null)[] = [
    ...Array.from({ length: leading }, () => null),
    ...Array.from({ length: daysInMonth }, (_, index) => {
      const day = String(index + 1).padStart(2, "0");
      return `${year}-${String(month).padStart(2, "0")}-${day}`;
    }),
  ];

  return (
    <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
      <h2 className="text-[17px] font-semibold text-gray-900">{monthLabel}</h2>
      <div className="mt-3 grid grid-cols-7 gap-1 text-center">
        {[1, 2, 3, 4, 5, 6, 7].map((weekday) => (
          <span key={weekday} className="text-[15px] text-gray-500">
            {t(`weekdayShort.${weekday}`)}
          </span>
        ))}
        {cells.map((iso, index) => (
          <span
            key={iso ?? `empty-${index}`}
            className={`flex h-9 flex-col items-center justify-center rounded-lg text-[15px] ${
              iso === today
                ? "bg-moto-blue-soft font-semibold text-gray-900"
                : "text-gray-700"
            }`}
          >
            {iso ? Number(iso.slice(8)) : ""}
            {iso && withEvents.has(iso) && (
              <span
                className="bg-moto-blue mt-0.5 block size-1.5 rounded-full"
                aria-hidden="true"
              />
            )}
          </span>
        ))}
      </div>
      <p className="mt-3 text-[15px] text-gray-500">{t("monthHint")}</p>
    </div>
  );
}
