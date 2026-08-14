"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  CalendarOverviewList,
  PersonalCalendar,
  type CalendarViewMode,
} from "~/components/calendar/personal-calendar";
import { CalendarSubscribePanel } from "~/components/calendar/calendar-subscribe-panel";
import { Modal } from "~/components/ui/modal";
import { ParentPage } from "~/components/parent/parent-page";
import { useToast } from "~/contexts/ToastContext";
import {
  getParentAppointmentOverview,
  getParentCalendar,
  respondParentCalendar,
  type CalendarAppointmentOverview,
  type CalendarResponse,
} from "~/lib/personal-calendar-api";
import { toISODate } from "~/lib/date-helpers";
import { getWeekRange } from "~/lib/timetable-helpers";

function messageFromError(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function calendarRange(referenceDate: Date, viewMode: CalendarViewMode) {
  if (viewMode === "day") return { from: referenceDate, to: referenceDate };
  if (viewMode === "month") {
    // Month view renders a 42-day grid starting at the week containing the 1st
    // (see monthGridDays in personal-calendar.tsx). Fetch that full visible
    // range so appointments on the leading/trailing adjacent-month days show,
    // instead of only the calendar month itself.
    const firstOfMonth = new Date(
      referenceDate.getFullYear(),
      referenceDate.getMonth(),
      1,
    );
    const from = getWeekRange(firstOfMonth).from;
    const to = new Date(from);
    to.setDate(from.getDate() + 41);
    return { from, to };
  }
  return getWeekRange(referenceDate);
}

export default function ParentCalendarPage() {
  const toast = useToast();
  // Focal date defaults to today; the calendar component derives the week
  // range for week view, so today shows the current week / month / day
  // correctly (not the start of the week or the wrong month at boundaries).
  const [referenceDate, setReferenceDate] = useState(() => new Date());
  const [viewMode, setViewMode] = useState<CalendarViewMode>("week");
  const [data, setData] = useState<CalendarResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [overview, setOverview] = useState<CalendarAppointmentOverview | null>(
    null,
  );
  const [overviewLoading, setOverviewLoading] = useState(false);
  const [respondingRecipientId, setRespondingRecipientId] = useState<
    string | null
  >(null);

  const range = useMemo(
    () => calendarRange(referenceDate, viewMode),
    [referenceDate, viewMode],
  );
  const rangeKey = `${viewMode}-${toISODate(range.from)}-${toISODate(range.to)}`;

  // Guards against out-of-order responses: quickly switching views/ranges can
  // leave an older getParentCalendar request in flight that resolves last and
  // overwrites the current range's events. Only the most recently issued load
  // is allowed to touch state.
  const latestRequestRef = useRef(0);
  const load = useCallback(
    async (options?: { readonly silent?: boolean }) => {
      const silent = options?.silent ?? false;
      const requestId = ++latestRequestRef.current;
      if (!silent) setLoading(true);
      try {
        const response = await getParentCalendar(range.from, range.to);
        if (requestId !== latestRequestRef.current) return;
        setData(response);
        setError(null);
      } catch (err) {
        if (requestId !== latestRequestRef.current) return;
        // A visible (range-change) load failed: drop the previous range's data
        // so old appointments aren't shown under the newly selected range.
        // Silent refreshes (e.g. after an RSVP) keep the current data.
        if (!silent) setData(null);
        setError(
          messageFromError(err, "Kalender konnte nicht geladen werden."),
        );
      } finally {
        if (!silent && requestId === latestRequestRef.current) {
          setLoading(false);
        }
      }
    },
    [range.from, range.to],
  );

  useEffect(() => {
    void load();
  }, [load, rangeKey]);

  const handleRespond = async (
    recipientId: string,
    status: "accepted" | "declined",
  ) => {
    setRespondingRecipientId(recipientId);
    try {
      await respondParentCalendar(recipientId, status);
      await load({ silent: true });
      toast.success(
        status === "accepted" ? "Termin zugesagt." : "Termin abgesagt.",
      );
    } catch (err) {
      toast.error(
        messageFromError(err, "Antwort konnte nicht gespeichert werden."),
      );
    } finally {
      setRespondingRecipientId(null);
    }
  };

  const handleShowOverview = async (appointmentId: string) => {
    // Clear any previous appointment's attendees so a failed/slow request
    // can't reopen the modal with stale attendees from a different appointment.
    setOverview(null);
    setOverviewLoading(true);
    try {
      setOverview(await getParentAppointmentOverview(appointmentId));
    } catch (err) {
      setOverview(null);
      toast.error(
        messageFromError(
          err,
          "Teilnehmerübersicht konnte nicht geladen werden.",
        ),
      );
    } finally {
      setOverviewLoading(false);
    }
  };

  return (
    <ParentPage>
      <PersonalCalendar
        title="Familienkalender"
        subtitle="Termine, Einladungen und Betreuungsangebote Ihrer Kinder."
        cardHeader
        events={data?.events ?? []}
        referenceDate={referenceDate}
        viewMode={viewMode}
        loading={loading}
        error={error}
        onDateChange={setReferenceDate}
        onViewModeChange={setViewMode}
        onShowOverview={handleShowOverview}
        onRespond={handleRespond}
        respondingRecipientId={respondingRecipientId}
        icsHrefBase="/api/parent/calendar/appointments"
      />
      <CalendarSubscribePanel />
      <Modal
        isOpen={overview !== null || overviewLoading}
        onClose={() => {
          if (!overviewLoading) setOverview(null);
        }}
        title="Teilnehmer"
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-xl"
      >
        {overviewLoading ? (
          <div className="py-8 text-center text-sm text-gray-500">
            Teilnehmer werden geladen...
          </div>
        ) : overview ? (
          <CalendarOverviewList overview={overview} />
        ) : null}
      </Modal>
    </ParentPage>
  );
}
