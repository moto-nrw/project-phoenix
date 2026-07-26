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

// When the flagged boundary is already in the past (client clock skew, or the
// timer resolving at the exact minute), refetch after this short delay rather
// than a whole day later.
const PAST_BOUNDARY_REFETCH_MS = 500;

// The backend reports next_change_at in Berlin wall-clock time (formatMinutes on
// timezone.Now()), so the timer delay must be measured against Berlin time, NOT
// the browser's local zone. A device on a different timezone (traveling staff, a
// misconfigured tablet, a UTC browser) would otherwise schedule the refetch at
// the wrong instant. Intl resolves the offset — including DST — for us.
const BERLIN_TIME_ZONE = "Europe/Berlin";

const BERLIN_TIME_PARTS_FORMATTER = new Intl.DateTimeFormat("en-GB", {
  timeZone: BERLIN_TIME_ZONE,
  hour12: false,
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
});

const BERLIN_DATE_TIME_PARTS_FORMATTER = new Intl.DateTimeFormat("en-GB", {
  timeZone: BERLIN_TIME_ZONE,
  hour12: false,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
});

// berlinSecondsOfDay returns the current wall-clock time in Europe/Berlin as
// seconds since local midnight (0..86399), independent of the browser's own
// timezone.
function berlinSecondsOfDay(now: Date): number {
  const parts = BERLIN_TIME_PARTS_FORMATTER.formatToParts(now);
  const value = (type: string) =>
    Number(parts.find((p) => p.type === type)?.value ?? "0");
  // Some engines render midnight as "24" under hour12:false; normalize to 0.
  const hour = value("hour") % 24;
  return hour * 3600 + value("minute") * 60 + value("second");
}

// berlinUtcOffsetSeconds returns Europe/Berlin's UTC offset in seconds at the
// given instant: +7200 in summer (CEST) or +3600 in winter (CET). Used to
// correct the scheduled delay across a DST transition (see nextChangeDelay).
//
// The offset is derived by formatting the instant as Berlin wall-clock fields
// and differencing that against the real instant. This relies only on Intl
// timeZone support (already required by berlinSecondsOfDay), NOT the newer
// "longOffset" option — so it stays correct on older WebViews instead of
// silently falling back to no DST correction (which would fire the refetch ~1h
// off on the two transition nights per year).
function berlinUtcOffsetSeconds(at: Date): number {
  const parts = BERLIN_DATE_TIME_PARTS_FORMATTER.formatToParts(at);
  const value = (type: string) =>
    Number(parts.find((p) => p.type === type)?.value ?? "0");
  // Some engines render midnight as "24" under hour12:false; normalize to 0.
  const asUtc = Date.UTC(
    value("year"),
    value("month") - 1,
    value("day"),
    value("hour") % 24,
    value("minute"),
    value("second"),
  );
  return Math.round((asUtc - at.getTime()) / 1000);
}

interface NextChangeDelay {
  // How long to wait before refetching, in ms.
  delayMs: number;
  // Whether the flagged boundary is a genuine future instant. Drives whether the
  // one-shot timer re-arms itself after firing (see the effect): a boundary that
  // is still in the future after the refetch — a day-agnostic "24:00" that maps
  // to the *next* midnight — must recur; one that has fallen into the past is a
  // spent one-shot for this occurrence.
  inFuture: boolean;
}

// nextChangeDelay computes how long to wait before refetching so the refetch
// lands just after the Berlin wall-clock "HH:MM" the backend flagged as the next
// time-based change, and whether that boundary is still in the future. Returns
// null for an unparseable value. When the target is already in the past (clock
// skew, or the timer resolving at the exact minute) it returns a short delay
// (inFuture: false) so we refetch promptly rather than a whole day later.
//
// "24:00" is accepted: the backend emits it from formatMinutes(1440) for a
// boundary at end-of-day (e.g. a 23:59 pickup flipping overdue at minute 1440).
// It maps to 86400 seconds — one full day of Berlin seconds ahead of midnight —
// so the timer fires at the next midnight instead of being dropped.
function nextChangeDelay(hhmm: string): NextChangeDelay | null {
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
  if (realDeltaSec <= 0) {
    return { delayMs: PAST_BOUNDARY_REFETCH_MS, inFuture: false };
  }
  return {
    delayMs: realDeltaSec * 1000 + NEXT_CHANGE_BUFFER_MS,
    inFuture: true,
  };
}

function scheduleNextChangeRefresh(
  nextChangeAt: string,
  refresh: () => unknown,
): () => void {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const arm = (isReArm: boolean) => {
    const next = nextChangeDelay(nextChangeAt);
    if (next === null) return;
    if (!next.inFuture) {
      if (!isReArm) {
        timer = setTimeout(() => {
          void refresh();
        }, next.delayMs);
      }
      return;
    }
    timer = setTimeout(() => {
      void refresh();
      arm(true);
    }, next.delayMs);
  };

  arm(false);
  return () => {
    if (timer) clearTimeout(timer);
  };
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
  // schedule a single timer to the exact Berlin wall-clock minute the backend
  // reported as the next change (a pickup/activity entering or leaving its
  // window). Because it fires one buffer past the boundary, the server excludes
  // the just-passed minute and the fresh response carries a later next_change_at
  // in the normal within-day case — a new string that re-runs this effect for
  // the following boundary.
  //
  // The one-shot timer RE-ARMS ITSELF after firing rather than relying on that
  // re-run, because the re-run is not guaranteed: a day-agnostic boundary can
  // repeat verbatim across midnight (a persistent end-of-day "24:00", or a daily
  // fixed threshold), and the refetch then returns a payload SWR compares equal
  // and does not re-render on — so the effect would never re-run and the timer
  // would never re-arm, silently degrading to the 60s poll from the first
  // midnight on. Rescheduling in the callback makes the timer self-sustaining.
  // We only re-arm when the boundary is still in the future (the "24:00" case,
  // which now maps to the *next* midnight); a boundary that has fallen into the
  // past is a spent one-shot, and re-arming it would busy-refetch under large
  // clock skew, so we let the refreshed response drive the next arm instead.
  //
  // Backstops for the residual cases: while the tab is hidden the browser
  // throttles/freezes this timer AND SWR pauses the refreshInterval poll
  // (refreshWhenHidden defaults off), so neither runs — the
  // `revalidateOnFocus: true` above is what refetches the moment the tab is
  // refocused. Client clock skew larger than the buffer is caught by the next
  // 60s poll once the tab is visible again.
  useEffect(() => {
    if (!enabled || !nextChangeAt) return;
    return scheduleNextChangeRefresh(nextChangeAt, mutate);
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
