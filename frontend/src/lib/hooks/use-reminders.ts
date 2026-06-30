"use client";

import { useSWRAuth } from "~/lib/swr/hooks";
import {
  fetchReminders,
  REMINDERS_SWR_KEY,
  type RemindersResult,
} from "~/lib/reminders-api";

// 60s tick: time-based reminders ("in 10 min") cross their threshold purely by
// time passing, with no backend event — so we re-evaluate on a fixed cadence.
const REFRESH_INTERVAL_MS = 60 * 1000;

/**
 * useReminders fetches the current visual reminders for the authenticated user.
 * The /reminders page renders the full list; the header bell reads the count
 * for its badge and previews the most urgent few. Both share the same SWR key,
 * so SWR dedupes to a single request.
 */
export function useReminders() {
  const { data, error, isLoading } = useSWRAuth<RemindersResult>(
    REMINDERS_SWR_KEY,
    fetchReminders,
    { refreshInterval: REFRESH_INTERVAL_MS },
  );

  return {
    reminders: data?.reminders ?? [],
    count: data?.count ?? 0,
    enabled: data?.enabled ?? false,
    error,
    isLoading,
  };
}
