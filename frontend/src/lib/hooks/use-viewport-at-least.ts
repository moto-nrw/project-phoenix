"use client";

import { useEffect, useState } from "react";
import { atLeast, type TailwindBreakpoint } from "~/lib/hooks/use-media-query";

/**
 * Tracks whether Tailwind's `<breakpoint>:` variants apply, updating when the
 * viewport crosses it. Use for layout branches that switch at a breakpoint
 * (e.g. the filter popover's mobile↔desktop placement).
 *
 * SSR-safe: starts `false` on the server and reads the real value on mount.
 */
export function useViewportAtLeast(breakpoint: TailwindBreakpoint): boolean {
  const query = atLeast(breakpoint);
  const [matches, setMatches] = useState(() =>
    typeof window === "undefined" || typeof window.matchMedia !== "function"
      ? false
      : window.matchMedia(query).matches,
  );

  useEffect(() => {
    const mediaQuery = window.matchMedia(query);
    const update = () => setMatches(mediaQuery.matches);
    update();
    mediaQuery.addEventListener("change", update);
    return () => mediaQuery.removeEventListener("change", update);
  }, [query]);

  return matches;
}
