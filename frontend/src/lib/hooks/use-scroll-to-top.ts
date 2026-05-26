"use client";

import { useEffect } from "react";

/**
 * Scrolls the window to the top on mount, and again whenever `key` changes.
 *
 * Why this is needed: Next.js App Router intentionally *maintains* the scroll
 * position on navigation when the new Page is already visible in the viewport
 * (see the `<Link>` scroll docs). Navigating from a scrolled-down list (e.g.
 * `/students/search`) to a long detail page in the same layout therefore lands
 * the user mid-page instead of at the top. Running `window.scrollTo(0, 0)` in
 * an effect overrides Next's handler (our effect runs after it) without
 * touching the global navigation, so back/forward scroll restoration on other
 * routes stays intact.
 *
 * Pass the route's identifying value (e.g. a student id) as `key` so the reset
 * also fires when navigating between two instances of the same route segment —
 * in that case the component does NOT remount, so a bare mount effect (`[]`)
 * would not fire.
 *
 * SSR-safe: the effect only runs in the browser.
 */
export function useScrollToTop(key?: unknown): void {
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [key]);
}
