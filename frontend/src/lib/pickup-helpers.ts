"use client";

import { useEffect, useState } from "react";

/**
 * Hook that returns a Date updated every 60 seconds.
 * Used for time-status calculations across pages.
 */
export function useMinuteClock(): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 60_000);
    return () => clearInterval(id);
  }, []);
  return now;
}
