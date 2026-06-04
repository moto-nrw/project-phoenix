"use client";

import { useEffect, useState } from "react";

/**
 * Tracks whether the viewport is at least `minWidth` px wide, updating on
 * resize. Use for layout branches that switch at a breakpoint (e.g. the
 * filter popover's mobile↔desktop placement).
 *
 * SSR-safe: starts `false` on the server and reads the real width on mount.
 */
export function useViewportAtLeast(minWidth: number): boolean {
  const [matches, setMatches] = useState(() =>
    typeof window === "undefined" ? false : window.innerWidth >= minWidth,
  );

  useEffect(() => {
    const update = () => setMatches(window.innerWidth >= minWidth);
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, [minWidth]);

  return matches;
}
