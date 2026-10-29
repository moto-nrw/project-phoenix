/**
 * Keeps a logout's cookie deletion the last word on the session (#5126).
 *
 * Every authenticated BFF answer carries the session cookie again (Auth.js
 * route handlers append it). A request that left this tab with the cookie and
 * is answered after a logout deleted it restores the ended session: the next
 * page, or the target school after a relogin, sees the old account again.
 *
 * While a session ends, the barrier holds new same-origin requests of the tab,
 * waits for the ones already in flight and asks Server-Sent Events to close.
 * Auth.js' own sign-out requests pass. Once the sign-out answer has deleted the
 * cookie, new requests leave without it and pass again; the held ones stay
 * held while the tab leaves.
 *
 * If the tab stays instead (sign-out failed, navigation aborted, or a new
 * sign-in in this document), the barrier resets: it aborts the held requests,
 * reports the discarded writes and lets Server-Sent Events reconnect. A held
 * write is never sent later, because it could reach the server under a
 * different session.
 *
 * A page restored from the back-forward cache while a session ends is reloaded
 * instead (decision on #5126): its paused logout continuations (a csrf lookup,
 * a session refetch, a navigation) would otherwise resume next to a new
 * logout. The state is not reset; until the page unloads, the tab's
 * same-origin requests and every answer of the ended session never settle, so
 * nothing of the old attempt runs on. The barrier never opens for that
 * generation again. Should the reload not happen (a „Seite verlassen?“ prompt
 * of the unsaved-changes guard declined), the tab stays frozen and the
 * NotificationBridge shows a lasting message with „Seite neu laden“.
 *
 * Every request that passes while a session ends (Auth.js' csrf and sign-out,
 * and requests after the deletion) carries a deadline. If the sign-out has not
 * answered by then, the barrier aborts those requests with `general.unavailable`
 * and resets, so a hanging logout never keeps the tab locked; the caller's
 * sign-out fails and shows that the logout did not go through.
 *
 * State machine, one generation per hold:
 *   idle ──hold──▶ ending(g) ──sign-out answered──▶ left(g) ──page leaves──▶ (gone)
 *                    │                                 │
 *                    └─ cancel / deadline ─────────────┴─ aborted navigation,
 *                       / new sign-in ──▶ idle            no page change ──▶ idle
 *   ending(g) or left(g) ──bfcache restore──▶ reload (state kept, tab frozen)
 * Invariants:
 *  - At most one logout runs per tab (`endSessionOnce` in session-end-flight);
 *    the barrier still guards every transition by its generation.
 *  - `cancel`, timers, sign-out answers and navigation watchers act only on
 *    the generation that created them; a stale one is a no-op.
 *  - Every way out of `ending` or `left` (failure, deadline, aborted
 *    navigation, no page change, new sign-in) goes through `open`, exactly
 *    once per generation. It aborts the generation's requests still in flight
 *    (a hanging sign-out ends here), clears its timers, rejects the held
 *    requests, reports discarded writes, lets SSE reconnect and frees the
 *    single-flight slot, so the next logout can start.
 *  - A bfcache restore during a session end reloads the page exactly once.
 */

import { unavailableApiError } from "~/lib/api-error";
import { releaseSessionEnd } from "~/lib/session-end-flight";

/** Dispatched on window; see `SessionEndDetail`. */
export const SESSION_END_EVENT = "phoenix:session-end";

/**
 * `ending` is true at the start, false on reset; resets count discarded writes.
 * `reloadRequired` asks the person to reload when a bfcache restore froze the
 * tab and the reload did not take it away.
 */
export interface SessionEndDetail {
  ending: boolean;
  discardedWrites: number;
  reloadRequired?: boolean;
}

const SIGN_OUT_PATHS = new Set(["/api/auth/csrf", "/api/auth/signout"]);
const SIGN_IN_PREFIX = "/api/auth/callback/";
const SETTLE_TIMEOUT_MS = 5000;
// Without a page change in this time the navigation counts as aborted.
const NAVIGATION_TIMEOUT_MS = 10_000;
// Time Auth.js gets for its csrf and sign-out requests once requests are held.
const SIGN_OUT_TIMEOUT_MS = 10_000;
const BARRIER_OWNER = Symbol.for("phoenix.session-end-barrier");

// "open" = idle, "holding" = ending, "cookie-deleted" = left.
type Phase = "open" | "holding" | "cookie-deleted";

interface HeldRequest {
  write: boolean;
  cleanup: () => void;
  reject: (reason: unknown) => void;
}

let installed = false;
let phase: Phase = "open";
// Set while the page reloads after a bfcache restore during a session end.
let reloading = false;
// Increments with every hold; see the invariants above.
let generation = 0;
let held: HeldRequest[] = [];
const inFlight = new Set<Promise<void>>();
const discarded = new WeakSet<object>();
let stopNavigationWatch: (() => void) | null = null;
// Aborts the requests that pass while this session ends; see the module comment.
let endController: AbortController | null = null;
let signOutTimer: ReturnType<typeof setTimeout> | undefined;
// Set when the deadline passed: this generation's late sign-out must not go out.
let expired: { generation: number; reason: unknown } | null = null;

/** Whether a logout of this tab is under way. */
export function isSessionEnding(): boolean {
  return phase !== "open";
}

/**
 * Whether the error is (or wraps) a request the barrier discarded. The barrier
 * reports those itself, once; a caller's error display skips them.
 */
export function discardedBySessionEnd(error: unknown): boolean {
  const seen = new Set<object>();
  while (typeof error === "object" && error !== null && !seen.has(error)) {
    if (discarded.has(error)) return true;
    seen.add(error);
    error = (error as { cause?: unknown }).cause;
  }
  return false;
}

function requestUrl(input: RequestInfo | URL): URL {
  const raw = input instanceof Request ? input.url : input.toString();
  return new URL(raw, window.location.href);
}

function requestMethod(input: RequestInfo | URL, init?: RequestInit): string {
  return (
    init?.method ?? (input instanceof Request ? input.method : "GET")
  ).toUpperCase();
}

/**
 * The one way out of a session end: opens the barrier, once, if `ending` is
 * still the current generation. `releaseFlight` is false only when a new hold
 * of the running logout closes a stale generation first.
 */
function open(
  ending: number,
  reason: unknown = unavailableApiError(
    new DOMException("The logout ended before it finished", "AbortError"),
  ),
  releaseFlight = true,
): void {
  if (reloading || ending !== generation || phase === "open") return;
  phase = "open";
  clearTimeout(signOutTimer);
  // A request of this generation still in flight, such as a hanging sign-out,
  // must not keep the logout (and with it the single-flight slot) waiting.
  endController?.abort(reason);
  endController = null;
  stopNavigationWatch?.();
  stopNavigationWatch = null;
  const queue = held;
  held = [];
  for (const request of queue) {
    request.cleanup();
    const error = new DOMException(
      "The logout discarded this request",
      "AbortError",
    );
    discarded.add(error);
    request.reject(error);
  }
  const detail: SessionEndDetail = {
    ending: false,
    discardedWrites: queue.filter((request) => request.write).length,
  };
  window.dispatchEvent(new CustomEvent(SESSION_END_EVENT, { detail }));
  if (releaseFlight) releaseSessionEnd();
}

/** After the cookie deletion the tab should leave; if it stays, open. */
function watchNavigation(ending: number): void {
  const navigation = (window as { navigation?: EventTarget }).navigation;
  const stay = () => open(ending);
  const left = () => {
    // The page is leaving. If it comes back from the cache, `pageshow` reloads.
    clearTimeout(timer);
    navigation?.removeEventListener("navigateerror", stay);
  };
  const timer = setTimeout(stay, NAVIGATION_TIMEOUT_MS);
  window.addEventListener("pagehide", left, { once: true });
  navigation?.addEventListener("navigateerror", stay);
  stopNavigationWatch = () => {
    left();
    window.removeEventListener("pagehide", left);
  };
}

/**
 * The request's own signal combined with this session end's deadline, without
 * AbortSignal.any, which older Safari versions lack.
 */
function withDeadline(
  input: RequestInfo | URL,
  init: RequestInit | undefined,
): RequestInit | undefined {
  const deadline = endController?.signal;
  if (!deadline) return init;
  const own =
    init?.signal ?? (input instanceof Request ? input.signal : undefined);
  const combined = new AbortController();
  for (const signal of [own, deadline]) {
    if (!signal) continue;
    if (signal.aborted) {
      combined.abort(signal.reason);
      break;
    }
    signal.addEventListener("abort", () => combined.abort(signal.reason), {
      once: true,
    });
  }
  return { ...init, signal: combined.signal };
}

/** The sign-out did not answer in time: abort what passed and open. */
function expireSignOut(ending: number): void {
  // A reloading page keeps its state: nothing of the old attempt may resume.
  if (reloading || ending !== generation) return;
  const reason = unavailableApiError(
    new DOMException("The logout did not finish in time", "TimeoutError"),
  );
  if (phase !== "holding") {
    // Left already: only the requests after the deletion still hang.
    endController?.abort(reason);
    return;
  }
  expired = { generation: ending, reason };
  open(ending, reason);
}

/** Never settles: what the reloading page asked for must not run on. */
function frozen<T>(): Promise<T> {
  return new Promise<T>(() => undefined);
}

/** A bfcache restore while a session ends: reload, once, without a reset. */
function reloadAbandonedSessionEnd(): void {
  if (reloading || phase === "open") return;
  // For good: no timer, answer or caller opens this generation again.
  reloading = true;
  // Seen only if the reload does not happen, e.g. a declined prompt.
  const detail: SessionEndDetail = {
    ending: true,
    discardedWrites: 0,
    reloadRequired: true,
  };
  window.dispatchEvent(new CustomEvent(SESSION_END_EVENT, { detail }));
  window.location.reload();
}

/** Wraps window.fetch once; requests started before it are not awaited. */
export function installSessionEndBarrier(): void {
  if (installed || typeof window === "undefined") return;
  installed = true;
  const originalFetch = window.fetch.bind(window);
  // One page, one barrier: only the latest installed copy handles pageshow
  // (module copies only differ in tests that reload the module).
  const owner = {};
  (window as unknown as Record<symbol, object>)[BARRIER_OWNER] = owner;
  window.addEventListener("pageshow", (event) => {
    const current = (window as unknown as Record<symbol, object>)[
      BARRIER_OWNER
    ];
    if (event.persisted && current === owner) reloadAbandonedSessionEnd();
  });

  window.fetch = (input, init) => {
    const url = requestUrl(input);
    if (url.origin !== window.location.origin)
      return originalFetch(input, init);
    if (reloading) return frozen();
    const method = requestMethod(input, init);
    if (expired?.generation === generation && SIGN_OUT_PATHS.has(url.pathname))
      return Promise.reject(expired.reason);
    if (phase === "holding" && !SIGN_OUT_PATHS.has(url.pathname)) {
      return new Promise<Response>((_resolve, reject) => {
        const signal =
          init?.signal !== undefined
            ? init.signal
            : input instanceof Request
              ? input.signal
              : undefined;
        if (signal?.aborted) {
          reject(signal.reason);
          return;
        }
        const abort = () => {
          held = held.filter((entry) => entry !== request);
          request.cleanup();
          reject(signal?.reason);
        };
        const request: HeldRequest = {
          // Background telemetry is not an unsaved user change.
          write:
            url.pathname !== "/api/logs" &&
            !["GET", "HEAD", "OPTIONS"].includes(method),
          cleanup: () => signal?.removeEventListener("abort", abort),
          reject,
        };
        held.push(request);
        signal?.addEventListener("abort", abort, { once: true });
      });
    }

    // A new sign-in in this document is not part of the ending session.
    const sent = originalFetch(
      input,
      phase === "open" || url.pathname.startsWith(SIGN_IN_PREFIX)
        ? init
        : withDeadline(input, init),
    );
    // An answer arriving while the page reloads resumes nothing.
    const response =
      phase === "open"
        ? sent
        : sent.then(
            (answer) => (reloading ? frozen<Response>() : answer),
            (error: unknown) =>
              reloading ? frozen<Response>() : Promise.reject(error),
          );
    const settled: Promise<void> = response.then(
      () => undefined,
      () => undefined,
    );
    inFlight.add(settled);
    void settled.then(() => inFlight.delete(settled));
    if (url.pathname === "/api/auth/signout" && phase === "holding") {
      // The answer has deleted the cookie: later requests cannot restore it.
      const ending = generation;
      void response.then(
        () => {
          if (ending !== generation || phase !== "holding") return;
          phase = "cookie-deleted";
          watchNavigation(ending);
        },
        () => undefined,
      );
    }
    if (
      url.pathname.startsWith(SIGN_IN_PREFIX) &&
      method === "POST" &&
      phase === "cookie-deleted"
    ) {
      // A new sign-in in this document replaces the ended session.
      const ending = generation;
      void response.then(
        (answer) => {
          if (answer.ok) open(ending);
        },
        () => undefined,
      );
    }
    return response;
  };
}

/**
 * Holds new requests, waits up to five seconds for those in flight and returns
 * `cancel` for a failed logout, which resets the barrier. The sign-out then has
 * ten seconds before the barrier aborts it
 * and resets on its own. Call it right before deleting the cookie. After the timeout a request still open may restore the
 * cookie; the next page then still sees the old session, as before.
 */
export async function holdRequestsUntilSessionEnds(): Promise<() => void> {
  installSessionEndBarrier();
  // A previous generation still ending is opened first, exactly once. It
  // belongs to an earlier logout; the running one keeps its slot.
  open(generation, undefined, false);
  generation += 1;
  const ending = generation;
  expired = null;
  endController = new AbortController();
  phase = "holding";
  const detail: SessionEndDetail = { ending: true, discardedWrites: 0 };
  window.dispatchEvent(new CustomEvent(SESSION_END_EVENT, { detail }));
  let timer: ReturnType<typeof setTimeout> | undefined;
  await Promise.race([
    Promise.allSettled(inFlight),
    new Promise((resolve) => {
      timer = setTimeout(resolve, SETTLE_TIMEOUT_MS);
    }),
  ]);
  clearTimeout(timer);
  // While the page reloads, this attempt must not go on to sign out.
  if (reloading) return frozen();
  signOutTimer = setTimeout(() => expireSignOut(ending), SIGN_OUT_TIMEOUT_MS);
  return () => cancel(ending);
}

/** A failed logout of `ending`: open it, and let a retry's sign-out go out. */
function cancel(ending: number): void {
  if (ending !== generation) return;
  expired = null;
  open(ending);
}
