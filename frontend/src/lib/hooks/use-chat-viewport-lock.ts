"use client";

import { useEffect, useRef } from "react";

/**
 * Pins a chat view to the real available viewport height and locks page
 * scrolling for as long as the chat is mounted — so the Nachrichten chat never
 * scrolls the whole page; only its inner message list scrolls.
 *
 * Why JS instead of a `h-[calc(100dvh-Xrem)]` class: the available height
 * depends on the surrounding shell (header height, main padding, mobile bottom
 * nav) which differs per portal/breakpoint. Measuring the element's real top
 * offset and filling to the viewport bottom is exact and shell-agnostic, with
 * no magic numbers to drift.
 *
 * `ready` should flip true once the element is actually rendered (e.g. after
 * the thread loads), so the height is measured against the final layout.
 */
export function useChatViewportLock<T extends HTMLElement>(ready: boolean) {
  const ref = useRef<T>(null);

  // Lock the document scroll for the whole time the chat is mounted.
  useEffect(() => {
    const html = document.documentElement;
    const body = document.body;
    const prevHtml = html.style.overflow;
    const prevBody = body.style.overflow;
    html.style.overflow = "hidden";
    body.style.overflow = "hidden";
    return () => {
      html.style.overflow = prevHtml;
      body.style.overflow = prevBody;
    };
  }, []);

  // Fit the container to the space between its top and the viewport bottom.
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const vv = window.visualViewport;
    const fit = () => {
      const top = el.getBoundingClientRect().top;
      // Measure against the VISUAL viewport, not window.innerHeight: when the
      // iOS soft keyboard opens it shrinks visualViewport.height (and fires only
      // visualViewport's own resize/scroll, never window's resize) while leaving
      // window.innerHeight unchanged. Keying off innerHeight would leave the
      // composer + "Senden" button stranded behind the keyboard — exactly the
      // parent-on-mobile flow this chat targets. Fall back to innerHeight where
      // visualViewport is unavailable (older browsers / jsdom) so nothing else
      // changes.
      const viewportHeight = vv?.height ?? window.innerHeight;
      // 8px breathing room at the bottom; never collapse below a usable size.
      el.style.height = `${Math.max(viewportHeight - top - 8, 240)}px`;
    };
    fit();
    window.addEventListener("resize", fit);
    // The iOS keyboard only nudges visualViewport, so listen there too.
    vv?.addEventListener("resize", fit);
    vv?.addEventListener("scroll", fit);
    return () => {
      window.removeEventListener("resize", fit);
      vv?.removeEventListener("resize", fit);
      vv?.removeEventListener("scroll", fit);
    };
  }, [ready]);

  return ref;
}
