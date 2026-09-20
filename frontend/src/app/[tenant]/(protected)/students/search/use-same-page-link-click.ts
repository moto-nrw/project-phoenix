import { useEffect, useRef } from "react";

/**
 * Calls `onClick` when the person clicks a link that points at the page they
 * are already on, without query parameters: the sidebar entry, the mobile
 * bottom nav, a breadcrumb (#3374).
 *
 * The App Router keys a page segment without its search params, so such a
 * click neither remounts the page nor changes anything the page could observe:
 * a search term lives in component state only, and the URL may already be
 * bare. The click itself is the only reliable signal, so it is read from the
 * document instead of from every shell component that renders such a link.
 *
 * The listener runs in the capture phase and does not cancel the event. The
 * link's own navigation still happens afterwards and lands on the same URL.
 */
export function useSamePageLinkClick(onClick: () => void): void {
  const onClickRef = useRef(onClick);
  useEffect(() => {
    onClickRef.current = onClick;
  }, [onClick]);

  useEffect(() => {
    const handleClick = (event: MouseEvent) => {
      // Modified clicks open a new tab or window; they leave this page alone.
      if (
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey
      ) {
        return;
      }
      if (!(event.target instanceof Element)) return;
      const anchor = event.target.closest("a[href]");
      if (!(anchor instanceof HTMLAnchorElement)) return;
      if (anchor.target !== "" && anchor.target !== "_self") return;
      if (anchor.hasAttribute("download")) return;

      let url: URL;
      try {
        url = new URL(anchor.href, window.location.href);
      } catch {
        return;
      }
      if (
        url.origin !== window.location.origin ||
        url.pathname !== window.location.pathname ||
        url.search !== "" ||
        // An in-page anchor (skip link, heading link) is not a navigation.
        url.hash !== ""
      ) {
        return;
      }
      onClickRef.current();
    };

    document.addEventListener("click", handleClick, true);
    return () => document.removeEventListener("click", handleClick, true);
  }, []);
}
