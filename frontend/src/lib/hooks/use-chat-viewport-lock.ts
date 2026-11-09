"use client";

import { useEffect, useRef } from "react";

/** Set on <html> while a mounted chat sees the soft keyboard open. */
const CHAT_KEYBOARD_ATTRIBUTE = "data-chat-keyboard";

/**
 * Set on <html> while a mounted chat has little height to work with (a small
 * phone, a short window). Pages and chats hide what a person does not need to
 * read and write there (`in-data-chat-compact:hidden`): the page intro, a
 * second readers line.
 */
const CHAT_COMPACT_ATTRIBUTE = "data-chat-compact";

/**
 * Marks the message list inside a chat container. Every chat that uses this
 * hook carries it on the element that scrolls its messages; the hook keeps
 * that list readable (see `MIN_MESSAGE_LIST_PX`).
 */
export const CHAT_SCROLL_ATTRIBUTE = "data-chat-scroll";

/**
 * Set on <html> while the chat locks page scrolling; `globals.css` turns it
 * into `overflow: hidden` on html and body. An attribute, not inline styles:
 * modals lock the page with inline `overflow` (`useScrollLock`) and restore
 * their saved value on close. Sharing that style, a chat that released its
 * lock under an open modal got it back from the modal and stayed stuck.
 */
export const CHAT_PAGE_LOCK_ATTRIBUTE = "data-chat-page-lock";

/**
 * The message list never gets less height than this. Below it a chat shows
 * one message or none (#5200 follow-up: on small phones the list shrank to
 * 24 px and could not be scrolled). When the chat chrome leaves less room, the
 * page scrolls instead of the chat clipping its own list and composer.
 */
export const MIN_MESSAGE_LIST_PX = 160;

/**
 * Usable height (viewport minus the mobile bottom nav) below which the chat
 * counts as compact. 360 × 640 phones and iPhones in Safari land below it;
 * tall phones, tablets and desktops do not.
 */
export const CHAT_COMPACT_BELOW_PX = 640;

/** Gap above the chat when the page scrolls it fully into view, without a shell. */
const SCROLLED_TOP_GAP_PX = 16;

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
 * The lock only holds while the chat fits. When the content above the chat
 * and the chat's own header and composer leave the message list
 * (`[data-chat-scroll]`) less than `MIN_MESSAGE_LIST_PX`, the hook releases
 * the page scroll and gives the chat the full visible height: the page
 * scrolls down to the chat, and the chat then fills the screen. Nothing is
 * ever clipped.
 *
 * `ready` should flip true once the element is actually rendered (e.g. after
 * the thread loads), so the height is measured against the final layout. Later
 * layout changes (content loading above the chat, the composer area growing)
 * re-run the fit on their own.
 */
/**
 * Height of the sticky or fixed bars across the top in front of `main` (the
 * desktop top bar): they stay on screen when the page scrolls. Bars that
 * scroll away, sidebars and the bottom nav do not count.
 */
function stickyHeightAbove(main: HTMLElement): number {
  let height = 0;
  for (
    let node: HTMLElement | null = main;
    node && node !== document.body;
    node = node.parentElement
  ) {
    for (
      let sibling = node.previousElementSibling;
      sibling;
      sibling = sibling.previousElementSibling
    ) {
      const { position, display } = getComputedStyle(sibling);
      if (display === "none") continue;
      if (position !== "sticky" && position !== "fixed") continue;
      // A bar across the top, not a full-height sidebar or the bottom nav.
      const rect = sibling.getBoundingClientRect();
      if (rect.top <= 1 && rect.height < window.innerHeight / 3) {
        height += rect.height;
      }
    }
  }
  return height;
}

/**
 * Height the chat container or a wrapper around the message list cuts off
 * (`overflow: hidden` with more content than room).
 */
function clippedBelow(list: HTMLElement, container: HTMLElement): number {
  let clipped = 0;
  for (let node = list.parentElement; node; node = node.parentElement) {
    clipped = Math.max(clipped, node.scrollHeight - node.clientHeight);
    if (node === container) break;
  }
  return clipped;
}

export function useChatViewportLock<T extends HTMLElement>(ready: boolean) {
  const ref = useRef<T>(null);

  // Lock the document scroll for the whole time the chat is mounted; fit
  // releases it while the chat does not fit on screen.
  useEffect(() => {
    const html = document.documentElement;
    html.setAttribute(CHAT_PAGE_LOCK_ATTRIBUTE, "");
    return () => html.removeAttribute(CHAT_PAGE_LOCK_ATTRIBUTE);
  }, []);

  // Fit the container to the space between its top and the viewport bottom.
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const vv = window.visualViewport;
    const pageLocked = () =>
      document.documentElement.hasAttribute(CHAT_PAGE_LOCK_ATTRIBUTE);
    const setPageLocked = (locked: boolean) =>
      document.documentElement.toggleAttribute(
        CHAT_PAGE_LOCK_ATTRIBUTE,
        locked,
      );
    const fit = () => {
      // iOS Safari scrolls the layout viewport to reveal a focused input even
      // though html/body overflow is set to hidden, then leaves that offset
      // stuck once the keyboard dismisses after sending: the sticky header
      // scrolls out (the topbar goes white) and, with page scroll locked, the
      // user can't scroll it back — the chat looks frozen. The chat pins page
      // scroll, so the document must always sit at the top; snap any stray
      // offset back whenever the viewport changes (fires on keyboard open/close
      // via visualViewport). A no-op on platforms that honour overflow:hidden.
      // While the page scrolls on purpose (the chat did not fit) the offset is
      // the person's own and stays.
      if (pageLocked() && window.scrollY !== 0) window.scrollTo(0, 0);
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
      const mainStyle = main ? getComputedStyle(main) : null;
      const bottomReserve = mainStyle
        ? parseFloat(mainStyle.paddingBottom) || 0
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
      // Decided from the layout viewport alone, not from what the chat shows:
      // hiding the intro must not flip the decision back on the next fit.
      document.documentElement.toggleAttribute(
        CHAT_COMPACT_ATTRIBUTE,
        navTop < CHAT_COMPACT_BELOW_PX,
      );
      // Document coordinates: once the page scrolls, the viewport top of the
      // chat moves with the scroll, its place in the page does not.
      const top = el.getBoundingClientRect().top + window.scrollY;
      const visibleBottom = Math.min(viewportHeight, navTop);
      const visible = viewportHeight - top;
      const available = visibleBottom - top;
      // Keep a usable minimum, but never past the visible area: page scroll is
      // locked, so anything below it — the composer, the line being typed —
      // is out of reach on a small phone with the keyboard open.
      const height = Math.max(available, Math.min(240, visible), 0);
      // What the chat needs besides its message list (header, quick actions,
      // composer), read from the current layout before resizing: the list is
      // the flexible part and takes whatever height is left. Content a
      // wrapper clips (the staff chats' cards are `overflow-hidden`) counts
      // too, otherwise a squeezed chat looks like it fits.
      const list = el.querySelector<HTMLElement>(`[${CHAT_SCROLL_ATTRIBUTE}]`);
      const needed = list
        ? el.clientHeight -
          list.clientHeight +
          clippedBelow(list, el) +
          MIN_MESSAGE_LIST_PX
        : 0;
      // The hook owns the size: a consumer's CSS min-height (min-h-[20rem])
      // would otherwise push the composer back under the keyboard.
      el.style.minHeight = "0px";
      // With the keyboard open the line being typed has priority (#3664):
      // the chat stays pinned to the keyboard edge, whatever the list keeps.
      if (!list || keyboardOpen || needed <= height) {
        el.style.height = `${height}px`;
        setPageLocked(true);
        return;
      }
      // The list would fall below a readable height: let the page scroll and
      // give the chat the whole visible height, so scrolling down shows the
      // full chat instead of a clipped one.
      setPageLocked(false);
      // Scrolled down, the chat starts where the shell's content starts: below
      // a sticky top bar and the main padding, like any page's first card.
      const contentTop =
        main && mainStyle
          ? stickyHeightAbove(main) + (parseFloat(mainStyle.paddingTop) || 0)
          : SCROLLED_TOP_GAP_PX;
      const fullView = visibleBottom - contentTop;
      el.style.height = `${Math.max(needed, fullView)}px`;
    };

    // Content above the chat (an intro, group messages, a dismissed panel)
    // and inside it (quick actions, a readers line) can change after the
    // first fit; every such change moves the chat or its chrome.
    let frame = 0;
    const scheduleFit = () => {
      if (frame) return;
      frame = window.requestAnimationFrame(() => {
        frame = 0;
        fit();
      });
    };
    fit();
    window.addEventListener("resize", fit);
    // The iOS keyboard only nudges visualViewport, so listen there too.
    vv?.addEventListener("resize", fit);
    vv?.addEventListener("scroll", fit);
    const resizeObserver =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(scheduleFit);
    resizeObserver?.observe(document.body);
    // Child and text changes only: fit itself writes styles, attributes would
    // loop.
    const mutationObserver = new MutationObserver(scheduleFit);
    mutationObserver.observe(el, {
      childList: true,
      subtree: true,
      characterData: true,
    });
    return () => {
      window.removeEventListener("resize", fit);
      vv?.removeEventListener("resize", fit);
      vv?.removeEventListener("scroll", fit);
      resizeObserver?.disconnect();
      mutationObserver.disconnect();
      if (frame) window.cancelAnimationFrame(frame);
      document.documentElement.removeAttribute(CHAT_KEYBOARD_ATTRIBUTE);
      document.documentElement.removeAttribute(CHAT_COMPACT_ATTRIBUTE);
      // The page lock itself belongs to the mount effect above; the next fit
      // (or its unmount cleanup) decides it.
    };
  }, [ready]);

  return ref;
}
