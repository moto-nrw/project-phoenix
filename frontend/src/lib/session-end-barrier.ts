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
 * held while the tab leaves. If the logout fails, `release` sends them.
 */

/** Dispatched on window; `detail.ending` is true at the start, false on release. */
export const SESSION_END_EVENT = "phoenix:session-end";

const SIGN_OUT_PATHS = new Set(["/api/auth/csrf", "/api/auth/signout"]);
const SETTLE_TIMEOUT_MS = 5000;

type Phase = "open" | "holding" | "cookie-deleted";

let installed = false;
let phase: Phase = "open";
let held: (() => void)[] = [];
const inFlight = new Set<Promise<void>>();

/** Whether a logout of this tab is under way. */
export function isSessionEnding(): boolean {
  return phase !== "open";
}

function requestUrl(input: RequestInfo | URL): URL {
  const raw = input instanceof Request ? input.url : input.toString();
  return new URL(raw, window.location.href);
}

/** Wraps window.fetch once; requests started before it are not awaited. */
export function installSessionEndBarrier(): void {
  if (installed || typeof window === "undefined") return;
  installed = true;
  const originalFetch = window.fetch.bind(window);

  window.fetch = (input, init) => {
    const url = requestUrl(input);
    if (url.origin !== window.location.origin)
      return originalFetch(input, init);
    if (phase === "holding" && !SIGN_OUT_PATHS.has(url.pathname)) {
      return new Promise<Response>((resolve, reject) => {
        held.push(() => {
          window.fetch(input, init).then(resolve, reject);
        });
      });
    }

    const response = originalFetch(input, init);
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
          if (phase === "holding") phase = "cookie-deleted";
        },
        () => undefined,
      );
    }
    return response;
  };
}

function release(): void {
  if (phase === "open") return;
  phase = "open";
  const queue = held;
  held = [];
  window.dispatchEvent(
    new CustomEvent(SESSION_END_EVENT, { detail: { ending: false } }),
  );
  for (const send of queue) send();
}

/**
 * Holds new requests, waits up to five seconds for those in flight and returns
 * `release` for a failed logout. Call it right before deleting the cookie.
 * After the timeout a request still open may restore the cookie; the next
 * page then still sees the old session, as before this barrier existed.
 */
export async function holdRequestsUntilSessionEnds(): Promise<() => void> {
  installSessionEndBarrier();
  phase = "holding";
  window.dispatchEvent(
    new CustomEvent(SESSION_END_EVENT, { detail: { ending: true } }),
  );
  let timer: ReturnType<typeof setTimeout> | undefined;
  await Promise.race([
    Promise.allSettled(inFlight),
    new Promise((resolve) => {
      timer = setTimeout(resolve, SETTLE_TIMEOUT_MS);
    }),
  ]);
  clearTimeout(timer);
  return release;
}
