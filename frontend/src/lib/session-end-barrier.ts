import { isOwnApiUrl } from "./session-account";

/**
 * Session-end barrier (#5399). A response sets its cookies when its headers
 * arrive, whatever the JavaScript that sent it does with the result. A
 * session lookup, refresh or BFF call still running when a logout deletes
 * the session cookie could therefore write the session back. From the start
 * of a logout until the cookie is gone, this tab sends nothing to its own BFF
 * that could renew the old session:
 *
 * 1. Closing the barrier aborts every request to this origin's `/api/` whose
 *    response headers have not arrived. The browser then drops their
 *    Set-Cookie. A GET or HEAD is sent again once the barrier opens; any
 *    other method fails, as an interrupted connection would.
 * 2. New requests wait until the barrier opens. Only the exit's own requests
 *    pass: revocation, CSRF, Auth.js signout, the preview end and the push
 *    cleanup.
 * 3. The SessionProvider stops polling while it is closed
 *    (useSessionPollInterval).
 * 4. Other tabs of this origin close too (BroadcastChannel). They open once
 *    the owner reports the deleted cookie or the failed logout, or after
 *    OTHER_TAB_HOLD_MS should the owner vanish.
 *
 * The barrier opens again when the logout fails (the session stays, and
 * waiting requests continue with it) or, unless the owner keeps it closed for
 * its own navigation, once the signout answer deleted the cookie: a request
 * sent after that carries no session. The server keeps a response that only
 * re-read the session from writing its cookie at all (session-cookie-writes).
 */

const CHANNEL = "moto-session-end";
/** Upper bound of a logout: revocation (5 s), CSRF and signout. */
const OTHER_TAB_HOLD_MS = 15_000;

type Owner = "self" | "other";
type Message = { type: "begin" | "ended" | "failed" };

let closedBy: Owner | null = null;
let reopenAfterDeletion = true;
let epoch = new AbortController();
let waiters: (() => void)[] = [];
let otherTabTimer: ReturnType<typeof setTimeout> | undefined;
const listeners = new Set<() => void>();
let channel: BroadcastChannel | null = null;

function notify() {
  for (const listener of listeners) listener();
}

function post(type: Message["type"]) {
  channel?.postMessage({ type } satisfies Message);
}

function close(owner: Owner) {
  closedBy = owner;
  epoch.abort(new DOMException("The session is ending", "AbortError"));
  epoch = new AbortController();
  notify();
}

function open() {
  if (closedBy === null) return;
  closedBy = null;
  clearTimeout(otherTabTimer);
  const release = waiters;
  waiters = [];
  for (const resume of release) resume();
  notify();
}

/** Hears the other tabs' logouts; the fetch observer starts it at load. */
export function listenForSessionEnds() {
  listen();
}

function listen() {
  if (channel || typeof BroadcastChannel === "undefined") return;
  channel = new BroadcastChannel(CHANNEL);
  channel.onmessage = (event: MessageEvent<Message>) => {
    if (closedBy === "self") return;
    if (event.data?.type === "begin") {
      close("other");
      clearTimeout(otherTabTimer);
      otherTabTimer = setTimeout(open, OTHER_TAB_HOLD_MS);
    } else if (closedBy === "other") open();
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
  listen();
  reopenAfterDeletion = !keepClosed;
  close("self");
  post("begin");
  let settled = false;
  return {
    fail() {
      if (settled || closedBy !== "self") return;
      settled = true;
      post("failed");
      open();
    },
  };
}

/** The portal's signout answer deleted the session cookie. */
function sessionCookieDeleted() {
  if (closedBy !== "self") return;
  post("ended");
  if (reopenAfterDeletion) open();
}

const EXIT_PATH =
  /^\/api\/(?:[a-z]+\/)?auth\/(?:logout|csrf|signout|staff-preview\/end)$/;
const PUSH_CLEANUP_PATH = /\/notifications\/push\/subscriptions$/;

function isExitRequest(url: URL, method: string): boolean {
  return (
    EXIT_PATH.test(url.pathname) ||
    (method === "DELETE" && PUSH_CLEANUP_PATH.test(url.pathname))
  );
}

function waitUntilOpen(signal: AbortSignal | null | undefined) {
  if (closedBy === null) return Promise.resolve();
  return new Promise<void>((resolve, reject) => {
    const abort = () => {
      waiters = waiters.filter((waiter) => waiter !== resume);
      reject(signal?.reason);
    };
    const resume = () => {
      signal?.removeEventListener("abort", abort);
      resolve();
    };
    if (signal?.aborted) {
      reject(signal.reason);
      return;
    }
    signal?.addEventListener("abort", abort, { once: true });
    waiters.push(resume);
  });
}

function requestUrl(input: RequestInfo | URL): string {
  return input instanceof Request ? input.url : input.toString();
}

/**
 * Sends one fetch through the barrier. Requests to other origins and paths
 * outside `/api/` pass unchanged.
 */
export async function fetchThroughSessionBarrier(
  send: typeof fetch,
  input: RequestInfo | URL,
  init?: RequestInit,
): Promise<Response> {
  const raw = requestUrl(input);
  if (!isOwnApiUrl(raw)) return send(input, init);
  listen();
  const url = new URL(raw, window.location.origin);
  const method = (
    init?.method ?? (input instanceof Request ? input.method : "GET")
  ).toUpperCase();
  if (isExitRequest(url, method)) {
    const response = await send(input, init);
    if (method === "POST" && url.pathname.endsWith("/signout") && response.ok)
      sessionCookieDeleted();
    return response;
  }

  const callerSignal =
    init?.signal ?? (input instanceof Request ? input.signal : undefined);
  const repeatable = method === "GET" || method === "HEAD";
  for (;;) {
    // An open barrier sends at once, in the caller's task, as fetch does.
    if (closedBy !== null) await waitUntilOpen(callerSignal);
    const ending = epoch.signal;
    const signal = callerSignal
      ? AbortSignal.any([callerSignal, ending])
      : ending;
    try {
      return await send(
        repeatable && input instanceof Request ? input.clone() : input,
        { ...init, signal },
      );
    } catch (error) {
      if (!ending.aborted || callerSignal?.aborted || !repeatable) throw error;
      // The barrier stopped this read before its answer could set a cookie.
      // Ask again once it opens, with the cookie that holds then.
    }
  }
}

/**
 * Axios runs on XHR: its request waits for the barrier and is aborted with
 * it. Returns the signal the request must carry.
 */
export async function sessionBarrierSignal(
  callerSignal?: AbortSignal,
): Promise<AbortSignal> {
  listen();
  await waitUntilOpen(callerSignal);
  return callerSignal
    ? AbortSignal.any([callerSignal, epoch.signal])
    : epoch.signal;
}

/** Test seam: forget the state of a previous test. */
export function resetSessionBarrierForTests() {
  closedBy = null;
  reopenAfterDeletion = true;
  epoch = new AbortController();
  waiters = [];
  clearTimeout(otherTabTimer);
  listeners.clear();
  channel?.close();
  channel = null;
}
