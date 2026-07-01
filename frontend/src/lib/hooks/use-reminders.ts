"use client";

import { useSWRAuth } from "~/lib/swr/hooks";
import {
  fetchReminders,
  REMINDERS_SWR_KEY,
  type RemindersResult,
} from "~/lib/reminders-api";

// 60s tick when the feature is on: time-based reminders ("in 10 min") cross
// their threshold purely by time passing, with no backend event — so we
// re-evaluate on a fixed cadence.
const ACTIVE_REFRESH_INTERVAL_MS = 60 * 1000;

// All reminder types default OFF, so most tenants never enable the feature. A
// disabled tenant has no time-based thresholds to catch, so there is no reason
// to poll every minute — we only need to notice, eventually, when an admin
// switches a type on (the admin's own tab flips instantly via the settings-page
// mutate; other open tabs discover it on this slower tick). Polling every 5 min
// in the disabled case cuts the pointless background load ~5x.
const IDLE_REFRESH_INTERVAL_MS = 5 * 60 * 1000;

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
    {
      refreshInterval: (latest) =>
        latest?.enabled ? ACTIVE_REFRESH_INTERVAL_MS : IDLE_REFRESH_INTERVAL_MS,
    },
  );

  return {
    reminders: data?.reminders ?? [],
    count: data?.count ?? 0,
    enabled: data?.enabled ?? false,
    // Raw payload so consumers can distinguish "loaded and disabled"
    // (data.enabled === false) from "not loaded yet" (data undefined) — the
    // /reminders route guard needs that to avoid redirecting during the initial
    // load / no-token window, where the derived `enabled` is falsy too.
    data,
    error,
    isLoading,
  };
}
