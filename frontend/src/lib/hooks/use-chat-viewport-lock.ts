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
    const fit = () => {
      const top = el.getBoundingClientRect().top;
      // 8px breathing room at the bottom; never collapse below a usable size.
      el.style.height = `${Math.max(window.innerHeight - top - 8, 240)}px`;
    };
    fit();
    window.addEventListener("resize", fit);
    return () => window.removeEventListener("resize", fit);
  }, [ready]);

  return ref;
}
