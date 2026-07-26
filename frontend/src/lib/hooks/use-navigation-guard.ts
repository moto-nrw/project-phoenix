"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";

import { registerNavigationBlocker } from "~/lib/hooks/navigation-guard-store";

interface NavigationGuard {
  /** The destination of an intercepted navigation, or null when none is pending. */
  pendingHref: string | null;
  /** Proceed with the intercepted navigation (discarding whatever was blocking). */
  confirmNavigation: () => void;
  /** Abandon the intercepted navigation and stay on the page. */
  cancelNavigation: () => void;
}

// Guards against losing unsaved work across the three ways a user can leave a
// page, none of which Next.js surfaces through a single hook:
//
//  1. Hard unload (tab close / reload) — `beforeunload`. Next.js client-side
//     route changes never trigger it.
//  2. In-app <Link>/anchor click — intercepted in the document capture phase
//     before Next's router handles it: next/link bails when the click's
//     default is already prevented (see next/dist/client/link.js), so
//     preventDefault here stops the SPA navigation and lets the caller confirm.
//  3. Browser Back/Forward — `popstate`. No anchor click occurs and Next
//     handles the history change without a page unload, so neither (1) nor (2)
//     fires. We push a same-URL sentinel history entry while blocking so the
//     first Back press pops back onto this same URL: the URL is unchanged, so
//     Next.js does not navigate away and the drafts stay mounted. The popstate
//     handler then re-arms the sentinel and opens the confirmation modal.
export function useNavigationGuard(shouldBlock: boolean): NavigationGuard {
  const router = useRouter();
  const [pendingHref, setPendingHref] = useState<string | null>(null);
  // True when the pending navigation is a Back/Forward (history traversal with
  // no target URL) rather than a link click (router.push to a known href).
  const pendingPopRef = useRef(false);
  // Set right before we programmatically traverse history on confirm, so our
  // own popstate handler ignores the resulting event instead of re-trapping.
  const bypassPopRef = useRef(false);
  // True while our same-URL sentinel sits on top of the history stack. Cleared
  // by confirmNavigation (which leaves the stack in a navigated state), so the
  // effect cleanup knows whether it still has a sentinel to collapse.
  const armedRef = useRef(false);
  // Holds the deferred navigation for a *programmatic* nav (router.push via
  // useTenantRouter) that the guard intercepted. Distinct from the href/pop
  // cases: the router call has not happened yet, so on confirm we run this
  // thunk (after collapsing the sentinel) rather than router.replace-ing an
  // href or traversing history.
  const pendingActionRef = useRef<null | (() => void)>(null);
  // Set on a programmatic confirm: run once we have popped the sentinel back
  // onto this page (via the bypassed popstate), so the deferred push lands on a
  // clean [previous, this-page, target] stack instead of leaving a duplicate.
  const proceedAfterPopRef = useRef<null | (() => void)>(null);

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
      pendingPopRef.current = false;
      setPendingHref(target);
    };

    // Arm the Back/Forward trap: a same-URL entry on top of this page.
    window.history.pushState(null, "", window.location.href);
    armedRef.current = true;

    const onPopState = () => {
      // Ignore the popstate we triggered ourselves on confirm.
      if (bypassPopRef.current) {
        bypassPopRef.current = false;
        // A programmatic confirm popped the sentinel back onto this page; now
        // run the deferred navigation so it lands on a clean history stack.
        if (proceedAfterPopRef.current) {
          const proceed = proceedAfterPopRef.current;
          proceedAfterPopRef.current = null;
          proceed();
        }
        return;
      }
      // The Back press popped our sentinel; re-arm it so we stay on this URL
      // (and remain protected against a second Back press while the modal is
      // open), then ask the user to confirm.
      window.history.pushState(null, "", window.location.href);
      pendingPopRef.current = true;
      setPendingHref(
        window.location.pathname +
          window.location.search +
          window.location.hash,
      );
    };

    // Cover programmatic in-app navigation (a <button> whose handler calls
    // router.push via useTenantRouter) — no anchor click and no popstate fires
    // for those, so the click/popstate traps above miss them. The router
    // consults this registry before navigating and hands us the deferred call.
    const onProgrammaticNav = (proceed: () => void, href: string) => {
      pendingPopRef.current = false;
      pendingActionRef.current = proceed;
      setPendingHref(href);
    };
    const unregisterBlocker = registerNavigationBlocker(onProgrammaticNav);

    window.addEventListener("beforeunload", onBeforeUnload);
    document.addEventListener("click", onClick, true);
    window.addEventListener("popstate", onPopState);
    return () => {
      unregisterBlocker();
      window.removeEventListener("beforeunload", onBeforeUnload);
      document.removeEventListener("click", onClick, true);
      window.removeEventListener("popstate", onPopState);
      // Disarming in place (Save/Discard cleared the dirty state, or the page
      // unmounts) leaves our same-URL sentinel on top of the stack. Collapse it
      // so it doesn't pile up across edit→save→edit cycles — left in place it
      // makes the next Back press look dead and throws off the go(-2) on a
      // later popstate confirm. confirmNavigation clears armedRef before it
      // pushes/traverses, so we never pop an entry a navigation already owns.
      if (armedRef.current) {
        armedRef.current = false;
        window.history.go(-1);
      }
    };
  }, [shouldBlock]);

  const confirmNavigation = useCallback(() => {
    // Programmatic navigation (router.push via useTenantRouter). The router
    // call was deferred, not yet run. Collapse the same-URL sentinel back onto
    // this page, then run the deferred push from the popstate handler so it
    // lands on [previous, this-page, target] — what an unguarded push produces.
    if (pendingActionRef.current) {
      const proceed = pendingActionRef.current;
      pendingActionRef.current = null;
      setPendingHref(null);
      // Disarm before traversing so the effect cleanup doesn't also pop an
      // entry the navigation now owns.
      armedRef.current = false;
      bypassPopRef.current = true;
      proceedAfterPopRef.current = proceed;
      window.history.go(-1);
      return;
    }
    if (pendingPopRef.current) {
      pendingPopRef.current = false;
      setPendingHref(null);
      // We are sitting on the re-armed sentinel, so the page the user wanted
      // is two entries back (sentinel → this page → previous page). Suppress
      // the resulting popstate so the handler doesn't re-trap the traversal.
      // go(-2) consumes the sentinel, so the cleanup must not pop it again.
      armedRef.current = false;
      bypassPopRef.current = true;
      window.history.go(-2);
      return;
    }
    // We are still sitting on the same-URL sentinel we pushed when blocking
    // started. router.replace collapses it into the target instead of stacking
    // a new entry on top, so history lands on [previous, this-page, target] —
    // what an unguarded link click would produce. A plain router.push would
    // leave a duplicate this-page entry under the target, making the second
    // Back press from the target look dead. Disarm first so the effect cleanup
    // doesn't pop an entry the navigation now owns.
    armedRef.current = false;
    setPendingHref((href) => {
      if (href) router.replace(href);
      return null;
    });
  }, [router]);

  const cancelNavigation = useCallback(() => {
    pendingPopRef.current = false;
    pendingActionRef.current = null;
    setPendingHref(null);
  }, []);

  return { pendingHref, confirmNavigation, cancelNavigation };
}
