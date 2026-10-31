import { onOtherTabOwnerChange } from "./personal-storage";
import type { SessionEnd } from "./session-end-barrier";
import { sessionEndBarrier } from "./session-end-state";
import {
  allowConfirmedUnload,
  attemptNavigation,
} from "./hooks/navigation-guard-store";

let pending: {
  controller: AbortController;
  promise: Promise<void>;
  phase: "confirming" | "working" | "navigating";
} | null = null;

/** Automatic expiry handlers must not compete once the explicit exit starts. */
export function isTenantSessionEnding(): boolean {
  return pending !== null && pending.phase !== "confirming";
}

/** Navigation started, but the old document still owns its explicit exit. */
export function markTenantSessionNavigation(signal: AbortSignal): void {
  if (pending?.controller.signal === signal) pending.phase = "navigating";
}

/** One tenant logout/relogin per document; stale attempts never own a new slot. */
export function runTenantSessionEnd(
  endSession: (signal: AbortSignal) => Promise<void>,
  destination = "/",
): Promise<void> {
  if (pending) return pending.promise;
  const controller = new AbortController();
  const { signal } = controller;
  let restoreUnloadGuard: (() => void) | undefined;
  let barrier: SessionEnd | undefined;
  const cancel = () => {
    controller.abort();
    // A failed, cancelled or left exit lets the held requests go on.
    barrier?.fail();
    if (pending?.controller === controller) pending = null;
    restoreUnloadGuard?.();
    dispose();
  };
  const onPageShow = (event: PageTransitionEvent) => {
    if (event.persisted) cancel();
  };
  const stopOwnerListener = onOtherTabOwnerChange((owner) => {
    // A second tab echoes this logout by clearing its ended session's storage.
    // Only a newly bound owner supersedes the explicit exit, not that echo.
    if (owner !== null) cancel();
  });
  const dispose = () => {
    window.removeEventListener("pagehide", cancel);
    window.removeEventListener("pageshow", onPageShow);
    stopOwnerListener();
  };
  window.addEventListener("pagehide", cancel);
  window.addEventListener("pageshow", onPageShow);
  // Defer work until the slot is assigned, even if the callback throws.
  const promise = Promise.resolve()
    .then(() => {
      signal.throwIfAborted();
      return new Promise<void>((resolve, reject) => {
        const stopWaiting = () => resolve();
        signal.addEventListener("abort", stopWaiting, { once: true });
        const proceed = (confirmed: boolean) => {
          signal.removeEventListener("abort", stopWaiting);
          if (signal.aborted || pending?.controller !== controller) {
            resolve();
            return;
          }
          pending.phase = "working";
          // From here no answer of the ending session may set its cookie
          // (#5399). The document navigates itself after the signout, so the
          // barrier stays closed until it leaves.
          barrier = sessionEndBarrier()?.beginSessionEnd({ keepClosed: true });
          if (confirmed) restoreUnloadGuard = allowConfirmedUnload();
          try {
            resolve(endSession(signal));
          } catch (error) {
            reject(error);
          }
        };
        const blocked = attemptNavigation(
          () => proceed(true),
          destination,
          () => {
            cancel();
            resolve();
          },
        );
        if (!blocked) proceed(false);
      });
    })
    .catch((error: unknown) => {
      // Leaving/restoring the document is not a failed logout to report.
      if (!signal.aborted) throw error;
    })
    .finally(() => {
      if (
        pending?.controller === controller &&
        pending.phase === "navigating"
      ) {
        // Stop push cleanup and other completed work immediately, but keep the
        // exit slot until pagehide. Assigning href does not leave the document.
        controller.abort();
      } else cancel();
    });
  pending = { controller, promise, phase: "confirming" };
  return promise;
}
