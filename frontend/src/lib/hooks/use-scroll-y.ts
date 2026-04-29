"use client";

import { useEffect, useState } from "react";

/**
 * Tracks `window.scrollY` with `requestAnimationFrame`-throttled updates so
 * scroll-driven UI (e.g. compact-on-scroll headers) stays smooth without
 * burning a setState per scroll event.
 *
 * SSR-safe: starts at `0` and reads on mount.
 */
export function useScrollY(): number {
  const [scrollY, setScrollY] = useState(0);

  useEffect(() => {
    let frame = 0;
    const update = () => {
      frame = 0;
      setScrollY(window.scrollY);
    };
    const onScroll = () => {
      if (frame !== 0) return;
      frame = window.requestAnimationFrame(update);
    };

    setScrollY(window.scrollY);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => {
      window.removeEventListener("scroll", onScroll);
      if (frame !== 0) window.cancelAnimationFrame(frame);
    };
  }, []);

  return scrollY;
}
