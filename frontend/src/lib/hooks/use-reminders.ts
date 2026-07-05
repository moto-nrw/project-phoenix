"use client";

import { useEffect } from "react";
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

// Small cushion added to the scheduled refresh so the server-side minute
// boundary has definitely elapsed before we refetch: next_change_at is the top
// of a wall-clock minute, and client/server clocks can drift by a second or two.
const NEXT_CHANGE_BUFFER_MS = 2000;

// msUntilNextChange returns how long to wait before refetching so the refetch
// lands just after the wall-clock "HH:MM" the backend flagged as the next
// time-based change. Returns null for an unparseable value. When the target is
// already in the past (clock skew, or the timer resolving at the exact minute),
// it returns a short delay so we refetch promptly rather than a whole day later.
//
// "24:00" is accepted: the backend emits it from formatMinutes(1440) for a
// boundary at end-of-day (e.g. a 23:59 pickup flipping overdue at minute 1440).
// setHours(24, 0) normalizes to the next day's midnight, which is that exact
// instant — so the timer still fires on the boundary instead of being dropped.
function msUntilNextChange(hhmm: string): number | null {
  const match = /^(\d{2}):(\d{2})$/.exec(hhmm);
  if (!match) return null;
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  if (
    Number.isNaN(hours) ||
    Number.isNaN(minutes) ||
    hours > 24 ||
    (hours === 24 && minutes > 0) ||
    minutes > 59
  ) {
    return null;
  }
  const now = new Date();
  const target = new Date(now);
  target.setHours(hours, minutes, 0, 0);
  const delay = target.getTime() + NEXT_CHANGE_BUFFER_MS - now.getTime();
  return delay > 0 ? delay : 500;
}

/**
 * useReminders fetches the current visual reminders for the authenticated user.
 * The /reminders page renders the full list; the header bell reads the count
 * for its badge and previews the most urgent few. Both share the same SWR key,
 * so SWR dedupes to a single request.
 */
export function useReminders() {
  const { data, error, isLoading, mutate } = useSWRAuth<RemindersResult>(
    REMINDERS_SWR_KEY,
    fetchReminders,
    {
      refreshInterval: (latest) =>
        latest?.enabled ? ACTIVE_REFRESH_INTERVAL_MS : IDLE_REFRESH_INTERVAL_MS,
      // The global SWR config disables focus revalidation (SSE covers most
      // freshness). Reminders are the exception: a backgrounded tab pauses the
      // refreshInterval poll entirely, so without this a threshold crossed while
      // the tab was hidden would not surface until a manual reload. Re-enable it
      // for this key so returning to the tab refetches immediately.
      revalidateOnFocus: true,
    },
  );

  const enabled = data?.enabled ?? false;
  const nextChangeAt = data?.next_change_at;

  // Event-driven refresh. useGlobalSSE() runs above TenantProvider and cannot
  // build the tenant-prefixed reminders SWR key, so it dispatches
  // "phoenix:reminders-stale" whenever an attendance / activity / student-data
  // change may alter what's due (or who may see it). Revalidate here, where the
  // SWR key is correctly tenant-scoped via useSWRAuth. Gate on `enabled`: a
  // disabled tenant has nothing to recompute, so keep its cheap 5-min idle poll
  // rather than react to every attendance burst.
  useEffect(() => {
    if (!enabled) return;
    const handler = () => {
      void mutate();
    };
    window.addEventListener("phoenix:reminders-stale", handler);
    return () => window.removeEventListener("phoenix:reminders-stale", handler);
  }, [enabled, mutate]);

  // Precise time-based refresh. Time-based reminders cross their threshold with
  // no backend event, so instead of only catching them on the 60s poll we
  // schedule a single timer to the exact wall-clock minute the backend reported
  // as the next change (a pickup/activity entering or leaving its window). The
  // timer refetches on the threshold; the fresh response carries the following
  // next_change_at, which reschedules this effect. The poll stays as a backstop
  // for the case where a hidden tab throttles this timer.
  useEffect(() => {
    if (!enabled || !nextChangeAt) return;
    const delay = msUntilNextChange(nextChangeAt);
    if (delay === null) return;
    const timer = setTimeout(() => {
      void mutate();
    }, delay);
    return () => clearTimeout(timer);
  }, [enabled, nextChangeAt, mutate]);

  return {
    reminders: data?.reminders ?? [],
    count: data?.count ?? 0,
    enabled,
    // Raw payload so consumers can distinguish "loaded and disabled"
    // (data.enabled === false) from "not loaded yet" (data undefined) — the
    // /reminders route guard needs that to avoid redirecting during the initial
    // load / no-token window, where the derived `enabled` is falsy too.
    data,
    error,
    isLoading,
  };
}
