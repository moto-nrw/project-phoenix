"use client";

/**
 * Live channel of ~/lib/swr (#5017): whether this tab's SSE stream currently
 * delivers invalidations for the session's scope, and what follows from it.
 *
 * - A reader that polls only to catch changes other people make asks
 *   `useLiveRefreshInterval(fallbackMs)`: while the stream of its own scope
 *   (school and account) is connected, it reconciles every five minutes;
 *   otherwise, and for every other scope, it keeps its fallback.
 * - The global SSE consumer binds its stream to `useLiveEventScope()`,
 *   publishes the stream with `usePublishLiveChannel` and asks
 *   `accepts(bound)` before an event acts: an event of a stream bound to
 *   another school or account, or arriving while another tab's session
 *   change is being confirmed, does nothing.
 *
 * SWR already skips interval polls of hidden tabs (`refreshWhenHidden`
 * stays false), so this module only decides the interval.
 */

import {
  useEffect,
  useLayoutEffect,
  useState,
  useSyncExternalStore,
} from "react";
import { useScopedSWRKey } from "./hooks";
import { isSWRSessionPending } from "./scope";

/** Reconcile interval of a reader whose changes arrive over SSE. */
export const LIVE_RECONCILE_INTERVAL_MS = 5 * 60_000;
/** Poll interval of such a reader while no stream of its scope is connected. */
export const LIVE_FALLBACK_INTERVAL_MS = 60_000;

interface LiveChannel {
  /** Scope key prefix (`scopedSWRKey(prefix, "")`) the stream serves. */
  readonly scope: string | null;
  readonly connected: boolean;
}

const IDLE: LiveChannel = { scope: null, connected: false };
let channel: LiveChannel = IDLE;
// The scope of the publisher's latest commit: an event of a stream bound to
// another scope no longer counts.
let currentScope: string | null = null;
const listeners = new Set<() => void>();

function publish(next: LiveChannel): void {
  if (next.scope === channel.scope && next.connected === channel.connected) {
    return;
  }
  channel = next;
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

const read = () => channel;
const readServer = () => IDLE;

/** Whether an event of a stream bound to `bound` may still act. */
function acceptsScope(bound: string | null): boolean {
  return bound !== null && bound === currentScope && !isSWRSessionPending();
}

/**
 * The scope this tab's SSE events are bound to, and whether an event of a
 * stream bound to a scope may still act. `scope` keeps the last scope with
 * session access while another tab's session change is being confirmed, so
 * the stream is not replaced for nothing; `accepts` is false meanwhile. A new
 * school or account binds the new scope. One caller per tab (the global SSE
 * consumer).
 */
export function useLiveEventScope(): {
  scope: string | null;
  accepts: (bound: string | null) => boolean;
} {
  const current = useScopedSWRKey("");
  const [scope, setScope] = useState(current);
  if (current !== null && current !== scope) setScope(current);
  useLayoutEffect(() => {
    currentScope = current;
  }, [current]);
  useEffect(
    () => () => {
      currentScope = null;
    },
    [],
  );
  return { scope: current ?? scope, accepts: acceptsScope };
}

/**
 * Publishes this tab's SSE stream as the live channel of `scope` (from
 * useLiveEventScope). Readers of that scope reconcile slowly while it is
 * connected.
 */
export function usePublishLiveChannel(
  scope: string | null,
  connected: boolean,
): void {
  useLayoutEffect(() => {
    publish({ scope, connected: connected && scope !== null });
  }, [scope, connected]);
  useEffect(() => () => publish(IDLE), []);
}

/**
 * Interval for a reader that polls only for other people's changes: five
 * minutes while the SSE stream of its own scope is connected, `fallbackMs`
 * otherwise. A fallback above five minutes stays as it is.
 */
export function useLiveRefreshInterval(
  fallbackMs: number = LIVE_FALLBACK_INTERVAL_MS,
): number {
  const scope = useScopedSWRKey("");
  const live = useSyncExternalStore(subscribe, read, readServer);
  return scope !== null && live.connected && live.scope === scope
    ? Math.max(fallbackMs, LIVE_RECONCILE_INTERVAL_MS)
    : fallbackMs;
}
