"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

interface NavigationGuard {
  /** The destination of an intercepted navigation, or null when none is pending. */
  pendingHref: string | null;
  /** Proceed with the intercepted navigation (discarding whatever was blocking). */
  confirmNavigation: () => void;
  /** Abandon the intercepted navigation and stay on the page. */
  cancelNavigation: () => void;
}

// Guards against losing unsaved work. `beforeunload` only fires for hard
// unloads (tab close / reload); Next.js client-side route changes never trigger
// it, so a sidebar/header <Link> click would silently discard the draft. We
// intercept those anchor clicks in the document capture phase before Next's
// router handles them: next/link bails when the click's default is already
// prevented (see next/dist/client/link.js), so preventDefault here stops the
// SPA navigation and lets the caller confirm first.
export function useNavigationGuard(shouldBlock: boolean): NavigationGuard {
  const router = useRouter();
  const [pendingHref, setPendingHref] = useState<string | null>(null);

  useEffect(() => {
    if (!shouldBlock) return;

    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };

    const onClick = (e: MouseEvent) => {
      // Let modified clicks (new tab/window), non-primary buttons, and clicks
      // a closer handler already cancelled fall through to default behaviour.
      if (
        e.defaultPrevented ||
        e.button !== 0 ||
        e.metaKey ||
        e.ctrlKey ||
        e.shiftKey ||
        e.altKey
      ) {
        return;
      }
      const anchor = (e.target as HTMLElement | null)?.closest("a");
      if (!anchor) return;
      if (anchor.target && anchor.target !== "_self") return;
      if (anchor.hasAttribute("download")) return;
      const href = anchor.getAttribute("href");
      if (!href || href.startsWith("#")) return;

      // Only intercept same-origin, in-app navigations to a different URL.
      const dest = new URL(anchor.href, window.location.href);
      if (dest.origin !== window.location.origin) return;
      const current =
        window.location.pathname +
        window.location.search +
        window.location.hash;
      const target = dest.pathname + dest.search + dest.hash;
      if (target === current) return;

      e.preventDefault();
      setPendingHref(target);
    };

    window.addEventListener("beforeunload", onBeforeUnload);
    document.addEventListener("click", onClick, true);
    return () => {
      window.removeEventListener("beforeunload", onBeforeUnload);
      document.removeEventListener("click", onClick, true);
    };
  }, [shouldBlock]);

  const confirmNavigation = useCallback(() => {
    setPendingHref((href) => {
      if (href) router.push(href);
      return null;
    });
  }, [router]);

  const cancelNavigation = useCallback(() => setPendingHref(null), []);

  return { pendingHref, confirmNavigation, cancelNavigation };
}
