import { isOwnApiUrl } from "./session-account";

/**
 * Session-end barrier (#5399). A response sets its cookies when its headers
 * arrive, whatever the JavaScript that sent it does with the result. A
 * session lookup, refresh or BFF call still running when a logout deletes
 * the session cookie could therefore write the session back. From the start
 * of a logout until the cookie is gone, this tab sends nothing to its own BFF
 * that could renew the old session:
 *
 * 1. Closing the barrier aborts every fetch to this origin's `/api/` whose
 *    response headers have not arrived. The browser then drops their
 *    Set-Cookie. A GET or HEAD is sent again once the barrier opens; any
 *    other method fails, as an interrupted connection would.
 * 2. New requests wait until the barrier opens. Only the exit's own requests
 *    pass: revocation, CSRF, Auth.js signout, the preview end and the push
 *    cleanup.
 * 3. The SessionProvider stops polling while it is closed
 *    (useSessionPollInterval).
 * 4. Other tabs of this origin close too (BroadcastChannel). The owner
 *    repeats `begin` every HEARTBEAT_MS while its attempt runs, however long
 *    CSRF or signout hang. Another tab opens only on the owner's `ended` or
 *    `failed`, or OTHER_TAB_HOLD_MS after the owner's last `begin` (the owner
 *    is gone). An `ended` also aborts what an open tab still has in flight:
 *    those requests carry the cookie the signout just deleted.
 * 5. The exit's own requests pass only while this tab's attempt runs, and
 *    only for that attempt: started before the logout they are aborted like
 *    any other request; they end with a failed attempt or the deleted cookie.
 *
 * The barrier opens again when the logout fails (the session stays, and
 * waiting requests continue with it) or, unless the owner keeps it closed for
 * its own navigation, once the signout answer deleted the cookie: a request
 * sent after that carries no session. The server keeps a response that only
 * re-read the session from writing its cookie at all (session-cookie-writes).
 * Axios needs no barrier: the browser paths use fetch; Axios only runs on the
 * server, against the backend.
 */

/** The owner's sign of life to the other tabs while its logout runs. */
const HEARTBEAT_MS = 5_000;
/** Three missed heartbeats: the owner is gone. */
const OTHER_TAB_HOLD_MS = 15_000;

let closedBy: "self" | "other" | null = null;
let reopenAfterDeletion = true;
let epoch = new AbortController();
/** The current logout attempt: its exit requests carry this signal. */
let attempt = new AbortController();
let heartbeat: ReturnType<typeof setInterval> | undefined;
const waiters = new Set<() => void>();
let otherTabTimer: ReturnType<typeof setTimeout> | undefined;
const listeners = new Set<() => void>();
let channel: BroadcastChannel | undefined;

function abortInFlight() {
  epoch.abort();
  epoch = new AbortController();
}

/** Ends this tab's attempt: no more heartbeats, its exit requests abort. */
function endAttempt() {
  clearInterval(heartbeat);
  attempt.abort();
}

function setClosed(owner: typeof closedBy) {
  if (owner) abortInFlight();
  else {
    clearTimeout(otherTabTimer);
    for (const resume of waiters) resume();
    waiters.clear();
  }
  closedBy = owner;
  for (const listener of listeners) listener();
}

/** Hears the other tabs' logouts; the fetch observer starts it at load. */
export function listenForSessionEnds() {
  if (channel || typeof BroadcastChannel === "undefined") return;
  channel = new BroadcastChannel("moto-session-end");
  channel.onmessage = ({ data }: MessageEvent<string>) => {
    if (closedBy === "self") return;
    if (data === "begin") {
      if (!closedBy) setClosed("other");
      clearTimeout(otherTabTimer);
      otherTabTimer = setTimeout(setClosed, OTHER_TAB_HOLD_MS, null);
      return;
    }
    // Requests sent after the hold ran out carry the deleted cookie.
    if (data === "ended") abortInFlight();
    if (closedBy) setClosed(null);
  };
}

/** Whether this tab holds its BFF requests for a session end. */
export function isSessionBarrierClosed(): boolean {
  return closedBy !== null;
}

/** For useSyncExternalStore: notified when the barrier closes or opens. */
export function subscribeSessionBarrier(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export interface SessionEnd {
  /** The logout failed or was cancelled: the session stays, requests go on. */
  fail(): void;
}

/**
 * Starts a logout: closes the barrier here and in the other tabs of this
 * origin. `keepClosed`: the owner navigates itself after the cookie is
 * deleted, so waiting requests never go out (tenant logout and relogin:
 * a late redirect to the source school's login must not race navigation).
 */
export function beginSessionEnd({
  keepClosed = false,
}: { keepClosed?: boolean } = {}): SessionEnd {
  listenForSessionEnds();
  clearTimeout(otherTabTimer);
  endAttempt();
  const current = (attempt = new AbortController());
  reopenAfterDeletion = !keepClosed;
  setClosed("self");
  const beat = () => channel?.postMessage("begin");
  beat();
  heartbeat = setInterval(beat, HEARTBEAT_MS);
  return {
    fail() {
      // Also after the signout (bfcache restore), never for a later attempt.
      if (attempt !== current || closedBy !== "self") return;
      endAttempt();
      channel?.postMessage("failed");
      setClosed(null);
    },
  };
}

/** The exit's auth requests: revocation, CSRF, signout and preview end. */
const EXIT_PATH =
  /^\/api\/(?:[a-z]+\/)?auth\/(?:logout|csrf|signout|staff-preview\/end)$/;

/**
 * Sends one fetch through the barrier. Requests to other origins and paths
 * outside `/api/` pass unchanged; an open barrier sends at once, in the
 * caller's task, as fetch does.
 */
export async function fetchThroughSessionBarrier(
  send: typeof fetch,
  input: RequestInfo | URL,
  init?: RequestInit,
): Promise<Response> {
  const isRequest = input instanceof Request;
  const raw = isRequest ? input.url : input.toString();
  if (!isOwnApiUrl(raw)) return send(input, init);
  const { pathname } = new URL(raw, location.origin);
  const method = init?.method ?? (isRequest ? input.method : "GET");
  const callerSignal = init?.signal ?? (isRequest ? input.signal : undefined);
  const exit =
    EXIT_PATH.test(pathname) ||
    (method.toUpperCase() === "DELETE" &&
      pathname.endsWith("/push/subscriptions"));
  if (exit && closedBy === "self") {
    const own = attempt.signal;
    const response = await send(input, {
      ...init,
      signal: callerSignal ? AbortSignal.any([callerSignal, own]) : own,
    });
    // The signout answer of this attempt deleted the session cookie.
    if (pathname.endsWith("/signout") && response.ok && !own.aborted) {
      endAttempt();
      channel?.postMessage("ended");
      if (reopenAfterDeletion) setClosed(null);
    }
    return response;
  }

  const repeatable = /^(GET|HEAD)$/i.test(method);
  for (;;) {
    callerSignal?.throwIfAborted();
    if (closedBy) {
      await new Promise<void>((resolve, reject) => {
        const resume = () => {
          callerSignal?.removeEventListener("abort", cancel);
          resolve();
        };
        const cancel = () => {
          waiters.delete(resume);
          reject(callerSignal?.reason);
        };
        waiters.add(resume);
        callerSignal?.addEventListener("abort", cancel, { once: true });
      });
    }
    const ending = epoch.signal;
    try {
      return await send(repeatable && isRequest ? input.clone() : input, {
        ...init,
        signal: callerSignal ? AbortSignal.any([callerSignal, ending]) : ending,
      });
    } catch (error) {
      if (!ending.aborted || callerSignal?.aborted || !repeatable) throw error;
      // The barrier stopped this read before its answer could set a cookie.
      // Ask again once it opens, with the cookie that holds then.
    }
  }
}

/** Test seam: forget the state of a previous test. */
export function resetSessionBarrierForTests() {
  closedBy = null;
  reopenAfterDeletion = true;
  epoch = new AbortController();
  clearInterval(heartbeat);
  attempt = new AbortController();
  waiters.clear();
  clearTimeout(otherTabTimer);
  listeners.clear();
  channel?.close();
  channel = undefined;
}
