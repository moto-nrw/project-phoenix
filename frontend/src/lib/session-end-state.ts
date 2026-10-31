import type * as SessionEndBarrier from "./session-end-barrier";

/**
 * What every page knows of the session-end barrier (#5399): whether one is
 * installed and whether it is closed. The barrier itself is installed only
 * where a session is used (`<SessionEndBarrier />`: the tenant shell's
 * protected layout and the other portals' providers), so the public tenant
 * pages (password reset, Anmeldung, demo, invitation, Träger hand-off) do not
 * carry its code. There a logout neither starts nor runs, and a response
 * that only re-read the session writes no cookie anyway (session-cookie-
 * writes).
 */

let barrier: typeof SessionEndBarrier | null = null;
let closed = false;
const listeners = new Set<() => void>();

/** The installed barrier, or null on a page without one. */
export function sessionEndBarrier(): typeof SessionEndBarrier | null {
  return barrier;
}

export function installSessionEndBarrier(installed: typeof SessionEndBarrier) {
  if (typeof window === "undefined" || barrier) return;
  barrier = installed;
  installed.listenForSessionEnds();
}

/** Whether this tab holds its BFF requests for a session end. */
export function isSessionBarrierClosed(): boolean {
  return closed;
}

/** For useSyncExternalStore: notified when the barrier closes or opens. */
export function subscribeSessionBarrier(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** The barrier reports each change of its state. */
export function announceSessionBarrier(next: boolean) {
  closed = next;
  for (const listener of listeners) listener();
}
