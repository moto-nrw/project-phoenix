"use client";

import { useEffect, useRef } from "react";

/** Set on <html> while a mounted chat sees the soft keyboard open. */
const CHAT_KEYBOARD_ATTRIBUTE = "data-chat-keyboard";

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
      // iOS Safari scrolls the layout viewport to reveal a focused input even
      // though html/body overflow is set to hidden, then leaves that offset
      // stuck once the keyboard dismisses after sending: the sticky header
      // scrolls out (the topbar goes white) and, with page scroll locked, the
      // user can't scroll it back — the chat looks frozen. The chat pins page
      // scroll, so the document must always sit at the top; snap any stray
      // offset back whenever the viewport changes (fires on keyboard open/close
      // via visualViewport). A no-op on platforms that honour overflow:hidden.
      if (window.scrollY !== 0) window.scrollTo(0, 0);
      // Measure against the VISUAL viewport, not window.innerHeight: when the
      // iOS soft keyboard opens it shrinks visualViewport.height (and fires only
      // visualViewport's own resize/scroll, never window's resize) while leaving
      // window.innerHeight unchanged. Keying off innerHeight would leave the
      // composer + "Senden" button stranded behind the keyboard — exactly the
      // parent-on-mobile flow this chat targets. Fall back to innerHeight where
      // visualViewport is unavailable (older browsers / jsdom) so nothing else
      // changes.
      const viewportHeight = vv?.height ?? window.innerHeight;
      // Reserve the space the fixed mobile bottom nav occupies, otherwise the
      // composer + "Senden" button hide behind the floating nav pill. The
      // shell's <main> already pads its content by exactly that amount
      // (pb-[calc(7rem+safe-area)] on mobile, pb-8 at lg+ where the nav is
      // hidden), so reuse its padding-bottom to end where every other page's
      // content ends — no hardcoded nav height to drift.
      const main = el.closest("main");
      const bottomReserve = main
        ? parseFloat(getComputedStyle(main).paddingBottom) || 0
        : 8;
      // The fixed nav sits at the bottom of the LAYOUT viewport. With the soft
      // keyboard open only the visual viewport shrinks and the nav is behind
      // the keyboard, so end at whichever edge comes first instead of
      // subtracting the nav from the keyboard edge again (#3664).
      const navTop = window.innerHeight - bottomReserve;
      // The keyboard covers the nav when the visible area ends above it.
      // Scaling by vv.scale keeps a pinch zoom (which shrinks vv.height too)
      // from counting as an open keyboard.
      const keyboardOpen = vv ? vv.height * vv.scale < navTop : false;
      // Pages hide their intro above the chat while the keyboard is open
      // (`in-data-chat-keyboard:hidden`), otherwise a small phone has barely
      // any room left for the composer. Toggle before measuring `top`.
      document.documentElement.toggleAttribute(
        CHAT_KEYBOARD_ATTRIBUTE,
        keyboardOpen,
      );
      const top = el.getBoundingClientRect().top;
      const visible = viewportHeight - top;
      const available = Math.min(viewportHeight, navTop) - top;
      // Keep a usable minimum, but never past the visible area: page scroll is
      // locked, so anything below it — the composer, the line being typed —
      // is out of reach on a small phone with the keyboard open.
      const height = Math.max(available, Math.min(240, visible), 0);
      el.style.height = `${height}px`;
      // The hook owns the size: a consumer's CSS min-height (min-h-[20rem])
      // would otherwise push the composer back under the keyboard.
      el.style.minHeight = "0px";
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
      document.documentElement.removeAttribute(CHAT_KEYBOARD_ATTRIBUTE);
    };
  }, [ready]);

  return ref;
}
