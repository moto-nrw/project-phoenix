/**
 * What the pages call of the installed barrier (session-end-barrier.ts),
 * stated here so that this module imports nothing from it.
 */
export interface SessionEndBarrier {
  fetchThroughSessionBarrier(
    send: typeof fetch,
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response>;
  beginSessionEnd(options?: { keepClosed?: boolean }): SessionEnd;
}

export interface SessionEnd {
  /** The logout failed or was cancelled: the session stays, requests go on. */
  fail(): void;
}

/**
 * What every page knows of the session-end barrier (#5399): whether one is
 * installed and whether it is closed. The barrier itself is installed only
 * where a session is used (`<SessionEndBarrier />`: the tenant shell's
 * protected layout and the other portals' providers), so the public tenant
 * pages (password reset, Anmeldung, demo, invitation, Träger hand-off) do not
 * carry its code. No logout starts there, and a response that only re-read
 * the session writes no cookie anyway (session-cookie-writes).
 */

let barrier: SessionEndBarrier | null = null;
let closed = false;
const listeners = new Set<() => void>();

/** For useSyncExternalStore: notified when the barrier closes or opens. */
export const subscribeSessionBarrier = (listener: () => void) => {
  listeners.add(listener);
  return () => listeners.delete(listener);
};

/** Whether this tab holds its BFF requests for a session end. */
export const isSessionBarrierClosed = () => closed;

/** The installed barrier, or null on a page without one. */
export const sessionEndBarrier = () => barrier;

export function installSessionEndBarrier(installed: SessionEndBarrier) {
  barrier = installed;
}

/** The barrier reports each change of its state. */
export function announceSessionBarrier(next: boolean) {
  closed = next;
  for (const listener of listeners) listener();
}
