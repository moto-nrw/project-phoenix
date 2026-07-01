// Module-level registry that lets a page block *programmatic* in-app
// navigation (e.g. a sidebar accordion whose <button> handler calls
// router.push) the same way `useNavigationGuard` already blocks <a> clicks,
// Back/Forward, and hard unloads.
//
// Why this exists: the Next.js App Router commits a router.push before it
// touches window.history (the pushState runs in a useInsertionEffect during
// commit), so there is no DOM event and no history hook to cancel a
// programmatic navigation after the fact. The only reliable interception point
// is *before* the router is called. `useTenantRouter` — the single choke point
// every in-app navigation goes through — consults this registry and defers to
// the active blocker instead of navigating.
//
// The registry is opt-in: with no blocker registered `attemptNavigation`
// returns false and the caller navigates immediately, so there is zero
// behaviour change on the vast majority of pages. Only a page that registers a
// blocker (via useNavigationGuard) changes anything.

/** Runs the navigation the guard intercepted, once the user confirms. */
export type ProceedFn = () => void;

/**
 * Invoked when a blocked navigation is attempted. The blocker is expected to
 * stash `proceed` and open its own confirmation UI, calling `proceed()` if the
 * user confirms and dropping it if they cancel. `href` is the intended
 * destination, for display in the confirmation.
 */
export type NavigationBlocker = (proceed: ProceedFn, href: string) => void;

// A stack rather than a single slot so nested/overlapping guards behave
// sanely: the most recently registered (top-most) blocker wins, and removing
// it restores the previous one.
const blockers: NavigationBlocker[] = [];

/**
 * Register a blocker as the active navigation guard. Returns an unregister
 * function; call it when the guard is no longer blocking (dirty state cleared
 * or the component unmounts).
 */
export function registerNavigationBlocker(
  blocker: NavigationBlocker,
): () => void {
  blockers.push(blocker);
  return () => {
    const idx = blockers.lastIndexOf(blocker);
    if (idx !== -1) blockers.splice(idx, 1);
  };
}

/**
 * Ask the active guard (if any) whether this navigation may proceed.
 *
 * - Returns `true` when a blocker intercepted the navigation: the caller must
 *   NOT navigate; the blocker owns `proceed` and will run it on confirm.
 * - Returns `false` when there is no active blocker: the caller navigates
 *   immediately.
 */
export function attemptNavigation(proceed: ProceedFn, href: string): boolean {
  const active = blockers[blockers.length - 1];
  if (!active) return false;
  active(proceed, href);
  return true;
}
