"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import {
  CalendarOverviewList,
  PersonalCalendar,
  type CalendarViewMode,
} from "~/components/calendar/personal-calendar";
import { Modal } from "~/components/ui/modal";
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

function startOfCurrentWeek(): Date {
  return getWeekRange(new Date()).from;
}

function messageFromError(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function calendarRange(referenceDate: Date, viewMode: CalendarViewMode) {
  if (viewMode === "day") return { from: referenceDate, to: referenceDate };
  if (viewMode === "month") {
    const first = new Date(
      referenceDate.getFullYear(),
      referenceDate.getMonth(),
      1,
    );
    const last = new Date(
      referenceDate.getFullYear(),
      referenceDate.getMonth() + 1,
      0,
    );
    return { from: first, to: last };
  }
  return getWeekRange(referenceDate);
}

export default function ParentCalendarPage() {
  const toast = useToast();
  const [referenceDate, setReferenceDate] = useState(startOfCurrentWeek);
  const [viewMode, setViewMode] = useState<CalendarViewMode>("week");
  const [data, setData] = useState<CalendarResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [overview, setOverview] = useState<CalendarAppointmentOverview | null>(
    null,
  );
  const [overviewLoading, setOverviewLoading] = useState(false);
  const [respondingRecipientId, setRespondingRecipientId] = useState<
    number | null
  >(null);

  const range = useMemo(
    () => calendarRange(referenceDate, viewMode),
    [referenceDate, viewMode],
  );
  const rangeKey = `${viewMode}-${toISODate(range.from)}-${toISODate(range.to)}`;

  const load = useCallback(
    async (options?: { readonly silent?: boolean }) => {
      const silent = options?.silent ?? false;
      if (!silent) setLoading(true);
      try {
        const response = await getParentCalendar(range.from, range.to);
        setData(response);
        setError(null);
      } catch (err) {
        setError(
          messageFromError(err, "Kalender konnte nicht geladen werden."),
        );
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [range.from, range.to],
  );

  useEffect(() => {
    void load();
  }, [load, rangeKey]);

  const handleRespond = async (
    recipientId: number,
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

  const handleShowOverview = async (appointmentId: string | number) => {
    setOverviewLoading(true);
    try {
      setOverview(await getParentAppointmentOverview(appointmentId));
    } catch (err) {
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
    <main className="mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <PersonalCalendar
        title="Familienkalender"
        subtitle="Termine, Einladungen und Betreuungsangebote Ihrer Kinder."
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
      />
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
    </main>
  );
}
