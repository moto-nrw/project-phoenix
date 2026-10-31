import { useSyncExternalStore } from "react";
import type * as SessionEndBarrier from "./session-end-barrier";

/**
 * What every page knows of the session-end barrier (#5399): whether one is
 * installed and whether it is closed. The barrier itself is installed only
 * where a session is used (`<SessionEndBarrier />`: the tenant shell's
 * protected layout and the other portals' providers), so the public tenant
 * pages (password reset, Anmeldung, demo, invitation, Träger hand-off) do not
 * carry its code. No logout starts there, and a response that only re-read
 * the session writes no cookie anyway (session-cookie-writes).
 */

let barrier: typeof SessionEndBarrier | null = null;
let closed = false;
const listeners = new Set<() => void>();

const subscribe = (listener: () => void) => {
  listeners.add(listener);
  return () => listeners.delete(listener);
};

/** The installed barrier, or null on a page without one. */
export const sessionEndBarrier = () => barrier;

export function installSessionEndBarrier(installed: typeof SessionEndBarrier) {
  barrier = installed;
}

/** The barrier reports each change of its state. */
export function announceSessionBarrier(next: boolean) {
  closed = next;
  for (const listener of listeners) listener();
}

/**
 * The SessionProvider's `refetchInterval`: none while a logout runs (#5399).
 * Its timer stops at once instead of asking for the ending session.
 */
export function useSessionPollInterval(seconds: number): number {
  return useSyncExternalStore(
    subscribe,
    () => closed,
    () => false,
  )
    ? 0
    : seconds;
}
