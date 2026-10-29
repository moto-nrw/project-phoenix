import { onOtherTabOwnerChange } from "./personal-storage";

let pending: { controller: AbortController; promise: Promise<void> } | null =
  null;

/** Automatic expiry handlers must not compete with this explicit exit. */
export function isTenantSessionEnding(): boolean {
  return pending !== null;
}

/** One tenant logout/relogin per document; stale attempts never own a new slot. */
export function runTenantSessionEnd(
  endSession: (signal: AbortSignal) => Promise<void>,
): Promise<void> {
  if (pending) return pending.promise;
  const controller = new AbortController();
  const { signal } = controller;
  const cancel = () => {
    controller.abort();
    if (pending?.controller === controller) pending = null;
    dispose();
  };
  const onPageShow = (event: PageTransitionEvent) => {
    if (event.persisted) cancel();
  };
  const stopOwnerListener = onOtherTabOwnerChange(cancel);
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
      return endSession(signal);
    })
    .catch((error: unknown) => {
      // Leaving/restoring the document is not a failed logout to report.
      if (!signal.aborted) throw error;
    })
    .finally(cancel);
  pending = { controller, promise };
  return promise;
}
