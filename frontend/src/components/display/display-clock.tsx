"use client";

import { useEffect, useState } from "react";

/**
 * Live clock for the info-point display. Rendered client-side only (first
 * paint is empty) so server and client markup never disagree.
 */
export function DisplayClock() {
  const [now, setNow] = useState<Date | null>(null);

  useEffect(() => {
    setNow(new Date());
    const timer = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  if (!now) {
    return <div className="h-24 w-64" aria-hidden />;
  }

  return (
    <div className="text-right">
      <p className="font-mono text-6xl font-bold text-gray-900 tabular-nums lg:text-7xl">
        {now.toLocaleTimeString("de-DE", {
          hour: "2-digit",
          minute: "2-digit",
        })}
      </p>
      <p className="mt-1 text-xl text-gray-500 lg:text-2xl">
        {now.toLocaleDateString("de-DE", {
          weekday: "long",
          day: "2-digit",
          month: "long",
        })}
      </p>
    </div>
  );
}
