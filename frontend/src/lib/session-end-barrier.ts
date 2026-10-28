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
 * If the tab stays instead (sign-out failed, navigation aborted, page restored
 * from the back-forward cache, or a new sign-in in this document), the barrier
 * resets: it aborts the held requests, reports the discarded writes and lets
 * Server-Sent Events reconnect. A held write is never sent later, because it
 * could reach the server under a different session.
 *
 * Every request that passes while a session ends (Auth.js' csrf and sign-out,
 * and requests after the deletion) carries a deadline. If the sign-out has not
 * answered by then, the barrier aborts those requests with `general.unavailable`
 * and resets, so a hanging logout never keeps the tab locked; the caller's
 * sign-out fails and shows that the logout did not go through.
 */

import { unavailableApiError } from "~/lib/api-error";

/** Dispatched on window; see `SessionEndDetail`. */
export const SESSION_END_EVENT = "phoenix:session-end";

/** `ending` is true at the start, false on reset; resets count discarded writes. */
export interface SessionEndDetail {
  ending: boolean;
  discardedWrites: number;
}

const SIGN_OUT_PATHS = new Set(["/api/auth/csrf", "/api/auth/signout"]);
const SIGN_IN_PREFIX = "/api/auth/callback/";
const SETTLE_TIMEOUT_MS = 5000;
// Without a page change in this time the navigation counts as aborted.
const NAVIGATION_TIMEOUT_MS = 10_000;
// Time Auth.js gets for its csrf and sign-out requests once requests are held.
const SIGN_OUT_TIMEOUT_MS = 10_000;

type Phase = "open" | "holding" | "cookie-deleted";

interface HeldRequest {
  write: boolean;
  cleanup: () => void;
  reject: (reason: unknown) => void;
}

let installed = false;
let phase: Phase = "open";
let held: HeldRequest[] = [];
const inFlight = new Set<Promise<void>>();
const discarded = new WeakSet<object>();
let stopNavigationWatch: (() => void) | null = null;
// Aborts the requests that pass while this session ends; see the module comment.
let endController: AbortController | null = null;
let signOutTimer: ReturnType<typeof setTimeout> | undefined;
// Set when the deadline passed: this logout's late sign-out must not go out.
let signOutExpiredReason: unknown = null;

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

function reset(): void {
  if (phase === "open") return;
  phase = "open";
  clearTimeout(signOutTimer);
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
}

/** After the cookie deletion the tab should leave; if it stays, reset. */
function watchNavigation(): void {
  const navigation = (window as { navigation?: EventTarget }).navigation;
  const left = () => {
    // The page is leaving. If it comes back from the cache, `pageshow` resets.
    clearTimeout(timer);
    navigation?.removeEventListener("navigateerror", reset);
  };
  const timer = setTimeout(reset, NAVIGATION_TIMEOUT_MS);
  window.addEventListener("pagehide", left, { once: true });
  navigation?.addEventListener("navigateerror", reset);
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

/** The sign-out did not answer in time: abort what passed and reset. */
function expireSignOut(): void {
  const reason = unavailableApiError(
    new DOMException("The logout did not finish in time", "TimeoutError"),
  );
  endController?.abort(reason);
  if (phase !== "holding") return;
  signOutExpiredReason = reason;
  reset();
}

/** Wraps window.fetch once; requests started before it are not awaited. */
export function installSessionEndBarrier(): void {
  if (installed || typeof window === "undefined") return;
  installed = true;
  const originalFetch = window.fetch.bind(window);
  window.addEventListener("pageshow", (event) => {
    if (event.persisted) reset();
  });

  window.fetch = (input, init) => {
    const url = requestUrl(input);
    if (url.origin !== window.location.origin)
      return originalFetch(input, init);
    const method = requestMethod(input, init);
    if (signOutExpiredReason && SIGN_OUT_PATHS.has(url.pathname))
      return Promise.reject(signOutExpiredReason);
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
    const response = originalFetch(
      input,
      phase === "open" || url.pathname.startsWith(SIGN_IN_PREFIX)
        ? init
        : withDeadline(input, init),
    );
    const settled: Promise<void> = response.then(
      () => undefined,
      () => undefined,
    );
    inFlight.add(settled);
    void settled.then(() => inFlight.delete(settled));
    if (url.pathname === "/api/auth/signout" && phase === "holding") {
      // The answer has deleted the cookie: later requests cannot restore it.
      void response.then(
        () => {
          if (phase !== "holding") return;
          phase = "cookie-deleted";
          watchNavigation();
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
      void response.then(
        (answer) => {
          if (answer.ok) reset();
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
 * ten seconds before the barrier aborts it and resets on its own. Call it right before
 * deleting the cookie. After the timeout a request still open may restore the
 * cookie; the next page then still sees the old session, as before.
 */
export async function holdRequestsUntilSessionEnds(): Promise<() => void> {
  installSessionEndBarrier();
  stopNavigationWatch?.();
  stopNavigationWatch = null;
  clearTimeout(signOutTimer);
  signOutExpiredReason = null;
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
  signOutTimer = setTimeout(expireSignOut, SIGN_OUT_TIMEOUT_MS);
  return cancel;
}

/** A failed logout: reset, and let the next attempt's sign-out go out. */
function cancel(): void {
  signOutExpiredReason = null;
  reset();
}
