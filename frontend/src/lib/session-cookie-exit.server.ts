import "server-only";
import { ServerResponse } from "node:http";
import { createLogger } from "~/lib/logger";

/**
 * The runtime lock at the exit of the Node server (#5520): every response of
 * this server, whatever produced it (route handler, Auth.js route, server
 * action, page, proxy, a backend response handed through), writes its status
 * line and headers through ServerResponse#writeHead, explicitly or on its
 * first write. There a Set-Cookie line with a session cookie name leaves only
 * if gateSessionCookieWrites approved it for this response, or if it deletes
 * the cookie. Any other is dropped and logged by cookie name, never value.
 */

const logger = createLogger({ component: "SessionCookieExit" });

/** Auth.js' session cookie of any portal, chunks included. */
export function isSessionCookieName(name: string): boolean {
  return /session-token(?:\.\d+)?$/.test(name);
}

/** How long an approval waits for its response to be written. */
const APPROVAL_LIFETIME_MS = 60_000;

const STATE = Symbol.for("moto.session-cookie-exit");
const LOCKED = Symbol.for("moto.session-cookie-exit.locked");

interface ExitState {
  /** Approved Set-Cookie lines and until when they may leave. */
  approved: Map<string, number>;
}

/**
 * The approvals live on globalThis: the route bundles and the
 * instrumentation bundle can each carry their own copy of this module.
 */
function exitState(): ExitState {
  const holder = globalThis as typeof globalThis & { [STATE]?: ExitState };
  holder[STATE] ??= { approved: new Map() };
  return holder[STATE];
}

/**
 * Records the session cookie writes gateSessionCookieWrites lets go. Each
 * approval admits its line once. A session cookie value is a fresh JWE, so a
 * line approved for one response does not occur in another.
 */
export function approveSessionCookieWrites(setCookies: readonly string[]) {
  const { approved } = exitState();
  const now = Date.now();
  for (const [line, until] of approved) {
    if (until > now) break;
    approved.delete(line);
  }
  for (const setCookie of setCookies) {
    const line = setCookie.trim();
    approved.delete(line);
    approved.set(line, now + APPROVAL_LIFETIME_MS);
  }
}

/** RFC 6265 5.2.2: a Max-Age the browser applies, else null. */
function maxAge(raw: string): number | null {
  return /^-?\d+$/.test(raw) ? Number(raw) : null;
}

/** The form Next and Auth.js write (Date#toUTCString); anything else is no deletion. */
const HTTP_DATE =
  /^(Mon|Tue|Wed|Thu|Fri|Sat|Sun), \d{2} (Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) \d{4} \d{2}:\d{2}:\d{2} GMT$/;

/** RFC 6265 5.1.1 bounds, restricted to the HTTP date form above. */
function cookieExpiry(raw: string): number | null {
  if (!HTTP_DATE.test(raw)) return null;
  const date = new Date(raw);
  // Date parsing accepts years before 1601 and normalizes invalid dates and
  // times. Neither is evidence of a deletion. The weekday is not significant.
  if (
    date.getUTCFullYear() < 1601 ||
    date.toUTCString().slice(5) !== raw.slice(5)
  )
    return null;
  return date.getTime();
}

/**
 * Whether a browser removes the cookie on this line (or keeps it empty).
 * Read as the browser does, so a contradiction cannot pass as a deletion:
 * the last valid Max-Age wins over Expires; an Expires counts only in the
 * HTTP date form. When in doubt, it is a write.
 */
function deletesCookie(value: string, attributes: string[]): boolean {
  if (value === "") return true;
  let age: number | null = null;
  let expires: number | null = null;
  for (const attribute of attributes) {
    const separator = attribute.indexOf("=");
    if (separator < 0) continue;
    const key = attribute.slice(0, separator).trim().toLowerCase();
    const raw = attribute.slice(separator + 1).trim();
    if (key === "max-age") age = maxAge(raw) ?? age;
    if (key === "expires") expires = cookieExpiry(raw) ?? expires;
  }
  if (age !== null) return age <= 0;
  return expires !== null && expires <= Date.now();
}

/** The session cookie a line writes, or null for a deletion or another cookie. */
function sessionCookieWrite(setCookie: string): string | null {
  const [pair = "", ...attributes] = setCookie.split(";");
  const separator = pair.indexOf("=");
  if (separator <= 0) return null;
  const name = pair.slice(0, separator).trim();
  if (!isSessionCookieName(name)) return null;
  const value = pair.slice(separator + 1).trim();
  return deletesCookie(value, attributes) ? null : name;
}

/**
 * The Set-Cookie lines that may leave: every other cookie, every deletion,
 * and session cookie writes approved by gateSessionCookieWrites. Each
 * approval is used up here.
 */
function admitSetCookies(setCookies: readonly string[]): string[] {
  const { approved } = exitState();
  const now = Date.now();
  return setCookies.filter((setCookie) => {
    const name = sessionCookieWrite(setCookie);
    if (name === null) return true;
    const line = setCookie.trim();
    const until = approved.get(line);
    approved.delete(line);
    if (until !== undefined && until > now) return true;
    logger.error("session_cookie_write_refused_at_exit", { cookie: name });
    return false;
  });
}

type HeaderValue = number | string | readonly string[];

const isSetCookie = (name: unknown) =>
  typeof name === "string" && name.toLowerCase() === "set-cookie";

/**
 * Merge only Set-Cookie into the progressive headers before checking it once.
 * Like Node's writeHead, object keys replace cached values (last key wins),
 * while explicit array pairs replace cached values but retain duplicates.
 * Leave all other headers to Node unchanged.
 */
function mergeCookieHeaders(
  response: ServerResponse,
  headers: unknown,
): unknown {
  if (Array.isArray(headers)) {
    let first = true;
    const merge = (name: string, value: HeaderValue) => {
      if (!isSetCookie(name)) return false;
      if (first) response.setHeader(name, value);
      else
        response.appendHeader(
          name,
          typeof value === "number" ? String(value) : value,
        );
      first = false;
      return true;
    };
    if (headers.length && Array.isArray(headers[0]))
      return (headers as [string, HeaderValue][])
        .filter(([name, value]) => !merge(name, value))
        .flat();
    const remaining: unknown[] = [];
    for (let i = 0; i < headers.length; i += 2) {
      const name = headers[i] as string;
      const value = headers[i + 1] as HeaderValue;
      if (!merge(name, value)) remaining.push(name, value);
    }
    return remaining;
  }
  if (headers && typeof headers === "object") {
    const remaining = { ...(headers as Record<string, unknown>) };
    for (const name of Object.keys(remaining)) {
      if (!isSetCookie(name)) continue;
      response.setHeader(name, remaining[name] as HeaderValue);
      delete remaining[name];
    }
    return remaining;
  }
  return headers;
}

type WriteHead = ServerResponse["writeHead"] & { [LOCKED]?: true };

/**
 * Installs the lock on the server's responses. instrumentation.ts calls it
 * once in the Node runtime, before Next handles a request. Node writes every
 * response head through writeHead, also implicitly on the first write or
 * end, so no response leaves past it.
 */
export function installSessionCookieExit(
  prototype: ServerResponse = ServerResponse.prototype,
): void {
  const writeHead = prototype.writeHead as WriteHead;
  if (writeHead[LOCKED]) return;
  const locked = function (this: ServerResponse, ...args: unknown[]) {
    if (!this.headersSent) {
      const index = typeof args[1] === "string" ? 2 : 1;
      if (args.length > index)
        args[index] = mergeCookieHeaders(this, args[index]);
      const own = this.getHeader("set-cookie");
      if (own !== undefined && typeof own !== "number") {
        const lines = typeof own === "string" ? [own] : own;
        const admitted = admitSetCookies(lines);
        if (admitted.length !== lines.length) {
          if (admitted.length) this.setHeader("set-cookie", admitted);
          else this.removeHeader("set-cookie");
        }
      }
    }
    return Reflect.apply(writeHead, this, args) as ServerResponse;
  } as WriteHead;
  locked[LOCKED] = true;
  prototype.writeHead = locked;
  logger.info("session_cookie_exit_locked");
}
