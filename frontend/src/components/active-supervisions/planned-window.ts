"use client";

import { useEffect, useRef, useState } from "react";
import { berlinDateTimeISO, parseISODate } from "~/lib/date-helpers";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

/**
 * „Als Nächstes“ shows the planned blocks of the current window: the backend
 * keeps a block from its start minus the horizon (at least 15 minutes, at
 * least the start lead) until its end (operations_planned_now.go). That
 * window moves with the clock, not with a write, so no SSE event announces
 * it (#5017). The whole-day list carries every block's times; from them the
 * view knows the next moment the window changes and reloads exactly then.
 */
const PLANNED_WINDOW_HORIZON_MS = 15 * 60_000;
// The server decides on its own clock; a small lag keeps a reload from
// arriving just before the boundary it is meant for.
export const PLANNED_WINDOW_SLACK_MS = 2_000;
// A client clock ahead of the server must not turn into a reload loop.
const PLANNED_WINDOW_MIN_DELAY_MS = 15_000;

function instant(date: string, time: string): number | null {
  try {
    return Date.parse(berlinDateTimeISO(parseISODate(date), time));
  } catch {
    return null;
  }
}

/** The next instant after `now` at which a planned block enters or leaves the window. */
export function nextPlannedWindowBoundary(
  blocks: readonly PlannedTimetableInstance[],
  now: number,
): number | null {
  let next: number | null = null;
  const consider = (at: number | null | undefined) => {
    if (at == null || Number.isNaN(at) || at <= now) return;
    if (next === null || at < next) next = at;
  };
  for (const block of blocks) {
    if (block.status !== "planned") continue;
    const start = instant(block.date, block.startTime);
    consider(start === null ? null : start - PLANNED_WINDOW_HORIZON_MS);
    consider(
      block.startAvailableAt ? Date.parse(block.startAvailableAt) : null,
    );
    consider(block.startExpiresAt ? Date.parse(block.startExpiresAt) : null);
    if (!block.isSpontaneous) consider(instant(block.date, block.endTime));
  }
  return next;
}

/**
 * Calls `refresh` at the next window boundary of `blocks`. A hidden tab
 * reloads nothing; it catches up once it is visible again.
 */
export function usePlannedWindowRefresh(
  blocks: readonly PlannedTimetableInstance[] | undefined,
  refresh: () => void,
): void {
  const refreshRef = useRef(refresh);
  useEffect(() => {
    refreshRef.current = refresh;
  }, [refresh]);
  // A reload with an unchanged answer keeps SWR's data identity; the tick
  // still moves the timer to the following boundary.
  const [tick, setTick] = useState(0);
  const missedRef = useRef(false);

  useEffect(() => {
    if (!blocks) return;
    const boundary = nextPlannedWindowBoundary(blocks, Date.now());
    if (boundary === null) return;
    const delay = Math.max(
      boundary - Date.now() + PLANNED_WINDOW_SLACK_MS,
      PLANNED_WINDOW_MIN_DELAY_MS,
    );
    const timer = setTimeout(() => {
      if (document.visibilityState === "hidden") {
        missedRef.current = true;
      } else {
        refreshRef.current();
      }
      setTick((value) => value + 1);
    }, delay);
    return () => clearTimeout(timer);
  }, [blocks, tick]);

  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState !== "visible" || !missedRef.current) return;
      missedRef.current = false;
      refreshRef.current();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, []);
}
