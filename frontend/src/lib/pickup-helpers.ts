"use client";

import { useEffect, useState } from "react";

/**
 * Hook that returns a Date updated at every wall-clock minute boundary.
 * Used for time-status calculations — and for the day rollover that
 * derives "today" from this clock — across pages.
 *
 * The tick is re-aligned to the next :00 of the minute rather than fired 60s
 * after mount, so a day change advances within milliseconds of midnight
 * instead of up to a full minute late (#1939: a stale clock kept the page on
 * yesterday's date while the backend already resolved the new day). It also
 * resyncs immediately on focus/visibility, because a backgrounded tab
 * throttles timers and the clock can be minutes stale on return.
 */
export function useMinuteClock(): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    let timeoutId: ReturnType<typeof setTimeout>;
    const scheduleNextTick = () => {
      const msUntilNextMinute = 60_000 - (Date.now() % 60_000);
      timeoutId = setTimeout(() => {
        setNow(new Date());
        scheduleNextTick();
      }, msUntilNextMinute);
    };
    scheduleNextTick();

    const resync = () => setNow(new Date());
    const onVisibilityChange = () => {
      if (document.visibilityState === "visible") resync();
    };
    window.addEventListener("focus", resync);
    document.addEventListener("visibilitychange", onVisibilityChange);

    return () => {
      clearTimeout(timeoutId);
      window.removeEventListener("focus", resync);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, []);
  return now;
}
