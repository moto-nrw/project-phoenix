"use client";

import { useEffect, useState } from "react";

import { berlinTodayISO } from "~/lib/date-helpers";

/**
 * The current Berlin calendar day as "YYYY-MM-DD", re-rendering the caller when
 * the day rolls over at Berlin midnight. Use instead of a bare
 * `berlinTodayISO()` call on any page that can stay mounted across midnight
 * (e.g. the meal plan) so the selected week and the "today" highlight follow the
 * date without a manual reload — a bare call is only re-read when something else
 * happens to re-render.
 *
 * Polls once a minute: `berlinTodayISO()` is cheap and the setState updater
 * no-ops when the value is unchanged, so a mounted page costs one string compare
 * per minute and re-renders only on the actual rollover.
 */
export function useBerlinToday(): string {
  const [today, setToday] = useState(berlinTodayISO);
  useEffect(() => {
    const id = setInterval(() => {
      const now = berlinTodayISO();
      setToday((prev) => (prev === now ? prev : now));
    }, 60_000);
    return () => clearInterval(id);
  }, []);
  return today;
}
