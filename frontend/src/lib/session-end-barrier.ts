import { isOwnApiUrl } from "./session-account";
import { announceSessionBarrier, type SessionEnd } from "./session-end-state";

/**
 * Session-end barrier (#5399): an optimization of the document that logs
 * out, not its security. The guarantee is the server's: a response writes a
 * session cookie only for a changed session of a live refresh-token family,
 * and the session route reads a revoked family as ended
 * (session-cookie-writes, browser-session). Other tabs, a tab started during
 * a logout and a frozen one need nothing here: their answers pass the same
 * server gate.
 *
 * In this document, from the start of a logout until the cookie is gone:
 *
 * 1. Requests to this origin's `/api/` whose response headers have not
 *    arrived are aborted, so their answers neither land nor trigger renewals
 *    and redirects. A GET or HEAD is sent again once the barrier opens; any
 *    other method fails, as an interrupted connection would. Answered
 *    requests (an SSE stream, a body still being read) keep running.
 * 2. New requests wait until the barrier opens; a caller that gives up ends
 *    the wait. Only the exit's own requests pass (revocation, CSRF, Auth.js
 *    signout, preview end, push cleanup), and only for the running attempt.
 * 3. The SessionProvider stops polling (useSessionPollInterval).
 *
 * The barrier opens when the logout fails (the session stays, waiting
 * requests continue with it) or, unless the owner keeps it closed for its own
 * navigation, once the signout answer deleted the cookie. Axios needs no
 * barrier: the browser paths use fetch; Axios only runs on the server.
 */

let closed = false;
let reopenAfterDeletion = true;
/** Requests whose response headers have not arrived. */
const awaiting = new Set<AbortController>();
/** The same for the exit requests of the running attempt. */
const exits = new Set<AbortController>();
/** The latest attempt; live until it failed or deleted the cookie. */
let attempt: object | null = null;
let attemptLive = false;
const waiters = new Set<() => void>();

function abortAll(requests: Set<AbortController>) {
  for (const request of requests) request.abort();
  requests.clear();
}

function endAttempt() {
  attemptLive = false;
  abortAll(exits);
}

function setClosed(next: boolean) {
  if (next) abortAll(awaiting);
  else {
    for (const resume of waiters) resume();
    waiters.clear();
  }
  closed = next;
  announceSessionBarrier(next);
}

/** Sends one request that the given set can abort until its headers arrive. */
async function sendTracked(
  requests: Set<AbortController>,
  request: AbortController,
  send: () => Promise<Response>,
): Promise<Response> {
  requests.add(request);
  try {
    return await send();
  } finally {
    requests.delete(request);
  }
}

/** Whether this document holds its BFF requests for a session end. */
export function isSessionBarrierClosed(): boolean {
  return closed;
}

/**
 * Starts a logout of this document. `keepClosed`: the owner navigates itself
 * after the cookie is deleted, so waiting requests never go out (tenant
 * logout and relogin: a late redirect to the source school's login must not
 * race navigation).
 */
export function beginSessionEnd({
  keepClosed = false,
}: { keepClosed?: boolean } = {}): SessionEnd {
  endAttempt();
  const current = (attempt = {});
  attemptLive = true;
  reopenAfterDeletion = !keepClosed;
  setClosed(true);
  return {
    fail() {
      // Also after the signout (bfcache restore), never for a later attempt.
      if (attempt !== current || !closed) return;
      endAttempt();
      setClosed(false);
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
  const merged = (own: AbortController) =>
    callerSignal ? AbortSignal.any([callerSignal, own.signal]) : own.signal;
  if (exit && closed && attemptLive) {
    const current = attempt;
    const own = new AbortController();
    const response = await sendTracked(exits, own, () =>
      send(input, { ...init, signal: merged(own) }),
    );
    // The signout answer of this attempt deleted the session cookie.
    if (
      pathname.endsWith("/signout") &&
      response.ok &&
      attemptLive &&
      attempt === current
    ) {
      endAttempt();
      if (reopenAfterDeletion) setClosed(false);
    }
    return response;
  }

  const repeatable = /^(GET|HEAD)$/i.test(method);
  for (;;) {
    callerSignal?.throwIfAborted();
    if (closed) {
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
    const own = new AbortController();
    try {
      return await sendTracked(awaiting, own, () =>
        send(repeatable && isRequest ? input.clone() : input, {
          ...init,
          signal: merged(own),
        }),
      );
    } catch (error) {
      if (!own.signal.aborted || callerSignal?.aborted || !repeatable)
        throw error;
      // The barrier stopped this read; ask again once it opens.
    }
  }
}

/** Test seam: forget the state of a previous test. */
export function resetSessionBarrierForTests() {
  closed = false;
  reopenAfterDeletion = true;
  awaiting.clear();
  exits.clear();
  attempt = null;
  attemptLive = false;
  waiters.clear();
  announceSessionBarrier(false);
}
