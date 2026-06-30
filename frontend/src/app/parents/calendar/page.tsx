"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { PersonalCalendar } from "~/components/calendar/personal-calendar";
import { useToast } from "~/contexts/ToastContext";
import {
  getParentCalendar,
  respondParentCalendar,
  type CalendarResponse,
} from "~/lib/personal-calendar-api";
import { getWeekRange } from "~/lib/timetable-helpers";

function startOfCurrentWeek(): Date {
  return getWeekRange(new Date()).from;
}

function messageFromError(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

export default function ParentCalendarPage() {
  const toast = useToast();
  const [weekStart, setWeekStart] = useState(startOfCurrentWeek);
  const [data, setData] = useState<CalendarResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [respondingRecipientId, setRespondingRecipientId] = useState<
    number | null
  >(null);

  const range = useMemo(() => getWeekRange(weekStart), [weekStart]);

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
  }, [load]);

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

  return (
    <main className="mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <PersonalCalendar
        title="Familienkalender"
        subtitle="Termine, Einladungen und Betreuungsangebote Ihrer Kinder."
        events={data?.events ?? []}
        weekStart={range.from}
        loading={loading}
        error={error}
        onWeekChange={setWeekStart}
        onRespond={handleRespond}
        respondingRecipientId={respondingRecipientId}
      />
    </main>
  );
}
