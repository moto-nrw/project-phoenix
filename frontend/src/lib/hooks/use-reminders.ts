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

// The backend reports next_change_at in Berlin wall-clock time (formatMinutes on
// timezone.Now()), so the timer delay must be measured against Berlin time, NOT
// the browser's local zone. A device on a different timezone (traveling staff, a
// misconfigured tablet, a UTC browser) would otherwise schedule the refetch at
// the wrong instant. Intl resolves the offset — including DST — for us.
const BERLIN_TIME_ZONE = "Europe/Berlin";

// berlinSecondsOfDay returns the current wall-clock time in Europe/Berlin as
// seconds since local midnight (0..86399), independent of the browser's own
// timezone.
function berlinSecondsOfDay(now: Date): number {
  const parts = new Intl.DateTimeFormat("en-GB", {
    timeZone: BERLIN_TIME_ZONE,
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).formatToParts(now);
  const value = (type: string) =>
    Number(parts.find((p) => p.type === type)?.value ?? "0");
  // Some engines render midnight as "24" under hour12:false; normalize to 0.
  const hour = value("hour") % 24;
  return hour * 3600 + value("minute") * 60 + value("second");
}

// berlinUtcOffsetSeconds returns Europe/Berlin's UTC offset in seconds at the
// given instant: +7200 in summer (CEST) or +3600 in winter (CET). Used to
// correct the scheduled delay across a DST transition (see msUntilNextChange).
function berlinUtcOffsetSeconds(at: Date): number {
  const tzName = new Intl.DateTimeFormat("en-US", {
    timeZone: BERLIN_TIME_ZONE,
    timeZoneName: "longOffset",
  })
    .formatToParts(at)
    .find((p) => p.type === "timeZoneName")?.value;
  // longOffset renders a zero-padded "GMT+02:00" / "GMT-01:00" (bare "GMT" for
  // UTC, which Berlin never is). Anything unexpected falls back to 0 rather than
  // NaN-poisoning the delay.
  const match = /GMT([+-])(\d{2}):(\d{2})/.exec(tzName ?? "");
  if (!match) return 0;
  const sign = match[1] === "-" ? -1 : 1;
  return sign * (Number(match[2]) * 3600 + Number(match[3]) * 60);
}

// msUntilNextChange returns how long to wait before refetching so the refetch
// lands just after the Berlin wall-clock "HH:MM" the backend flagged as the next
// time-based change. Returns null for an unparseable value. When the target is
// already in the past (clock skew, or the timer resolving at the exact minute),
// it returns a short delay so we refetch promptly rather than a whole day later.
//
// "24:00" is accepted: the backend emits it from formatMinutes(1440) for a
// boundary at end-of-day (e.g. a 23:59 pickup flipping overdue at minute 1440).
// It maps to 86400 seconds — one full day of Berlin seconds ahead of midnight —
// so the timer fires at the next midnight instead of being dropped.
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
  const targetSec = hours * 3600 + minutes * 60; // 24:00 → 86400
  const now = new Date();
  // wallDeltaSec is a wall-clock delta, but setTimeout counts real elapsed ms.
  // On a Berlin DST-transition day the two diverge by 3600s across the boundary
  // (spring-forward loses an hour, fall-back gains one), so a raw wall-clock
  // subtraction would fire ~1h early or late. Correct by the change in UTC
  // offset between now and the (estimated) target instant — Berlin's offset is
  // piecewise-constant, so a single estimate lands in the right region.
  const wallDeltaSec = targetSec - berlinSecondsOfDay(now);
  const offsetNow = berlinUtcOffsetSeconds(now);
  const estTarget = new Date(now.getTime() + wallDeltaSec * 1000);
  const offsetTarget = berlinUtcOffsetSeconds(estTarget);
  const realDeltaSec = wallDeltaSec - (offsetTarget - offsetNow);
  const delay = realDeltaSec * 1000 + NEXT_CHANGE_BUFFER_MS;
  return delay > 0 ? delay : 500;
}

// berlinDateKey returns the current Berlin calendar day as "YYYY-MM-DD",
// independent of the browser's own timezone. It discriminates the scheduling
// effect across midnight: next_change_at is a day-agnostic "HH:MM", so a
// boundary that recurs at the same wall-clock time on consecutive days (a
// persistent end-of-day "24:00", or any daily fixed threshold) would otherwise
// hand the effect an identical dependency and never re-arm the one-shot timer
// after it fires once. Folding the Berlin date in makes a repeated HH:MM on a
// new day a fresh dependency. It stays constant within a day, so a boundary
// that resolves in the past still refetches only once (no busy loop).
function berlinDateKey(now: Date): string {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: BERLIN_TIME_ZONE,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(now);
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
  // Re-armed per response (data identity changes on every refetch) but only
  // observably different across a Berlin midnight, so a repeated next_change_at
  // still schedules the next day's boundary — see berlinDateKey / the effect.
  const nextChangeDay =
    enabled && nextChangeAt ? berlinDateKey(new Date()) : null;

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
  // schedule a single timer to the exact Berlin wall-clock minute the backend
  // reported as the next change (a pickup/activity entering or leaving its
  // window). The timer refetches on the threshold; because it fires one buffer
  // past the boundary, the server excludes the just-passed minute and the fresh
  // response carries a strictly-later next_change_at — a new string that
  // re-runs this effect and schedules the following boundary. The effect key
  // also folds in the Berlin day (nextChangeDay): within a day the string alone
  // advances, but across midnight a boundary can repeat verbatim (a persistent
  // end-of-day "24:00", or a daily fixed threshold), and without the date that
  // identical string would never re-arm the fired one-shot timer — the feature
  // would silently fall back to the 60s poll from the next midnight on. The date
  // is stable within a day, so a boundary that resolves in the past still
  // refetches only once (no busy-refetch loop). Backstops for the residual
  // cases: while the tab is hidden the browser throttles/freezes this timer AND
  // SWR pauses the refreshInterval poll (refreshWhenHidden defaults off), so
  // neither runs — the `revalidateOnFocus: true` above is what refetches the
  // moment the tab is refocused. Client clock skew larger than the buffer is
  // caught by the next 60s poll once the tab is visible again.
  useEffect(() => {
    if (!enabled || !nextChangeAt) return;
    const delay = msUntilNextChange(nextChangeAt);
    if (delay === null) return;
    const timer = setTimeout(() => {
      void mutate();
    }, delay);
    return () => clearTimeout(timer);
  }, [enabled, nextChangeAt, nextChangeDay, mutate]);

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
