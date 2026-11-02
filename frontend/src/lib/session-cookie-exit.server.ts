import "server-only";
import { AsyncLocalStorage } from "node:async_hooks";
import { Server, ServerResponse } from "node:http";
import { Server as HttpsServer } from "node:https";
import { env } from "~/env";
import { createLogger } from "~/lib/logger";

/**
 * The runtime lock at the exit of the Node server (#5520): every response of
 * this server, whatever produced it (route handler, Auth.js route, server
 * action, page, proxy, a backend response handed through), writes its status
 * line and headers through ServerResponse#writeHead (or its writeHeader
 * alias), explicitly or on its first write. There a Set-Cookie line with a
 * session cookie name leaves only if gateSessionCookieWrites approved it
 * while handling this very request, or if it deletes the cookie. Any other
 * is dropped and logged by cookie name, never value.
 *
 * An approval belongs to one request: the server opens a context per request
 * (AsyncLocalStorage, entered where Node emits the request) and ties it to
 * the request's ServerResponse. It holds as long as that response exists and
 * no longer, with no time limit and nothing left behind.
 */

const logger = createLogger({ component: "SessionCookieExit" });

/** The session cookie of each portal; their Auth.js configs use these. */
export const PORTAL_SESSION_COOKIE_NAMES = {
  caterer: "caterer.session-token",
  operator: "operator.session-token",
  parent: "parent.session-token",
  school: "school.session-token",
  traeger: "traeger.session-token",
} as const;

/** Auth.js' default name, the tenant portal's without a TENANT_DOMAIN. */
const AUTHJS_SESSION_COOKIE = "authjs.session-token";

/** The tenant portal's session cookie for TENANT_DOMAIN (tenant-config.ts). */
export function tenantSessionCookieName(tenantDomain: string | undefined) {
  return !tenantDomain || tenantDomain === "localhost"
    ? AUTHJS_SESSION_COOKIE
    : `${tenantDomain.replace(/\./g, "-")}.session-token`;
}

/** Every configured session cookie name of this deployment. */
function sessionCookieBases(): string[] {
  return [
    ...Object.values(PORTAL_SESSION_COOKIE_NAMES),
    tenantSessionCookieName(env.TENANT_DOMAIN),
    AUTHJS_SESSION_COOKIE,
  ];
}

/** The browser's cookie name prefixes, in any case. */
const COOKIE_PREFIX = /^__(?:secure|host)-/i;

/**
 * Every cookie a session reader could take for a session cookie (#5520).
 * Auth.js (SessionStore, also behind getToken in the layout snapshots) takes
 * every cookie whose name starts with its configured name as a chunk:
 * `parent.session-token.evil`, `…_shadow`, `….0extra` and `….0.1` read as
 * the session. So does this, for every configured name, also behind a
 * `__Secure-`/`__Host-` prefix in any case: a superset of what any reader
 * takes, and no other cookie (`parent-session-token`, `session-token`).
 */
export function isSessionCookieName(name: string): boolean {
  const bare = name.replace(COOKIE_PREFIX, "");
  return sessionCookieBases().some((base) => bare.startsWith(base));
}

/**
 * The names Auth.js writes: a configured name, with Auth.js' own
 * `__Secure-`/`__Host-` prefix, or one of its chunks `<name>.<n>`. Only
 * these can be approved; every other session cookie name leaves the server
 * only as a deletion.
 */
export function isWrittenSessionCookieName(name: string): boolean {
  const bare = name.replace(/^__(?:Secure|Host)-/, "");
  return sessionCookieBases().some(
    (base) =>
      bare === base ||
      (bare.startsWith(`${base}.`) &&
        /^\d+$/.test(bare.slice(base.length + 1))),
  );
}

const STATE = Symbol.for("moto.session-cookie-exit");
const LOCKED = Symbol.for("moto.session-cookie-exit.locked");

interface RequestApprovals {
  /** Set-Cookie lines gateSessionCookieWrites let go for this request. */
  lines: Set<string>;
}

interface ExitState {
  /** The request being handled, entered when the server emits it. */
  request: AsyncLocalStorage<RequestApprovals>;
  /** The approvals of each response, released with the response. */
  responses: WeakMap<ServerResponse, RequestApprovals>;
}

/**
 * The state lives on globalThis: the route bundles and the instrumentation
 * bundle can each carry their own copy of this module.
 */
function exitState(): ExitState {
  const holder = globalThis as typeof globalThis & { [STATE]?: ExitState };
  holder[STATE] ??= {
    request: new AsyncLocalStorage(),
    responses: new WeakMap(),
  };
  return holder[STATE];
}

type Locked<F> = F & { [LOCKED]?: true };

/** Whether this process's HTTP servers and responses carry the lock. */
export function isSessionCookieExitLocked(): boolean {
  return Boolean(
    (ServerResponse.prototype.writeHead as Locked<unknown>)[LOCKED] &&
    Reflect.get(ServerResponse.prototype, "writeHeader") ===
      ServerResponse.prototype.writeHead &&
    (Server.prototype.emit as Locked<unknown>)[LOCKED] &&
    (HttpsServer.prototype.emit as Locked<unknown>)[LOCKED],
  );
}

/**
 * Records the session cookie writes gateSessionCookieWrites lets go, for the
 * response of the request it runs in. Without the lock nothing could hold a
 * bypass back, so the gate fails loudly instead (#5520). Outside a request
 * (no response to write) there is nothing to approve.
 */
export function approveSessionCookieWrites(setCookies: readonly string[]) {
  if (!isSessionCookieExitLocked())
    throw new Error(
      "The session cookie exit lock is not installed (#5520): instrumentation.ts did not run",
    );
  const approvals = exitState().request.getStore();
  if (!approvals) return;
  for (const setCookie of setCookies) approvals.lines.add(setCookie.trim());
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
 * Whether the line can only delete the cookie: an empty value, and a
 * lifetime that ends in the past by every reading. At most one Max-Age and
 * one Expires (names in any case), at least one of them, a Max-Age of plain
 * digits that is not positive, an Expires that is a valid date not later
 * than now. A repeated, contradicting or unreadable lifetime, or any value,
 * makes it a write that needs an approval.
 */
function deletesCookie(value: string, attributes: string[]): boolean {
  if (value !== "") return false;
  const ages: string[] = [];
  const expiries: string[] = [];
  for (const attribute of attributes) {
    const separator = attribute.indexOf("=");
    const key = (separator < 0 ? attribute : attribute.slice(0, separator))
      .trim()
      .toLowerCase();
    const raw = separator < 0 ? "" : attribute.slice(separator + 1).trim();
    if (key === "max-age") ages.push(raw);
    if (key === "expires") expiries.push(raw);
  }
  if (ages.length > 1 || expiries.length > 1) return false;
  const [age] = ages;
  const [expires] = expiries;
  if (age === undefined && expires === undefined) return false;
  if (age !== undefined && !(/^-?\d+$/.test(age) && Number(age) <= 0))
    return false;
  if (expires !== undefined) {
    const at = cookieExpiry(expires);
    if (at === null || at > Date.now()) return false;
  }
  return true;
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
 * The Set-Cookie lines a response may send: every other cookie, every
 * deletion, and session cookie writes approved while its request ran.
 */
function admitSetCookies(
  response: ServerResponse,
  setCookies: readonly unknown[],
): unknown[] {
  const approved = exitState().responses.get(response)?.lines;
  return setCookies.filter((setCookie) => {
    // Node takes any value for a header, numbers included.
    const line = String(setCookie);
    const name = sessionCookieWrite(line);
    if (name === null || approved?.has(line.trim())) return true;
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

type WriteHead = Locked<ServerResponse["writeHead"]>;
type Emit = Locked<Server["emit"]>;

/** Opens the approval context of every request a server emits. */
function lockEmit(prototype: Server | HttpsServer) {
  const emit = prototype.emit as Emit;
  if (emit[LOCKED]) return;
  const locked = function (
    this: Server,
    event: string | symbol,
    ...args: unknown[]
  ) {
    const response = args[1];
    const { request, responses } = exitState();
    if (response instanceof ServerResponse && !responses.has(response)) {
      const approvals: RequestApprovals = { lines: new Set() };
      responses.set(response, approvals);
      return request.run(approvals, () =>
        Reflect.apply(emit, this, [event, ...args]),
      ) as boolean;
    }
    return Reflect.apply(emit, this, [event, ...args]) as boolean;
  } as Emit;
  locked[LOCKED] = true;
  prototype.emit = locked;
}

/** Merges writeHead's Set-Cookie into the response, then admits it once. */
function filterHead(response: ServerResponse, args: unknown[], index: number) {
  if (args.length > index)
    args[index] = mergeCookieHeaders(response, args[index]);
  const own = response.getHeader("set-cookie");
  if (own === undefined || typeof own === "number") return;
  const lines: unknown[] = typeof own === "string" ? [own] : own;
  const admitted = admitSetCookies(response, lines);
  if (admitted.length === lines.length) return;
  if (admitted.length) response.setHeader("set-cookie", admitted as string[]);
  else response.removeHeader("set-cookie");
}

/** Takes every Set-Cookie out of the head, without throwing. */
function dropSetCookies(
  response: ServerResponse,
  args: unknown[],
  index: number,
  given: unknown,
) {
  try {
    response.removeHeader("set-cookie");
  } catch {
    // A response that refuses removeHeader already sent its head.
  }
  if (args.length <= index) return;
  try {
    if (Array.isArray(given)) {
      args[index] =
        given.length && Array.isArray(given[0])
          ? (given as unknown[][]).filter(([name]) => !isSetCookie(name))
          : given.filter(
              (_, i) => !isSetCookie(given[i % 2 === 0 ? i : i - 1]),
            );
    } else if (given && typeof given === "object") {
      args[index] = Object.fromEntries(
        Object.entries(given).filter(([name]) => !isSetCookie(name)),
      );
    } else args[index] = given;
  } catch {
    // Unreadable headers do not leave with this head.
    args[index] = undefined;
  }
}

/** Filters every response head at writeHead and its legacy writeHeader alias. */
function lockWriteHead() {
  const writeHead = ServerResponse.prototype.writeHead as WriteHead;
  if (writeHead[LOCKED]) {
    Reflect.set(ServerResponse.prototype, "writeHeader", writeHead);
    return;
  }
  const locked = function (this: ServerResponse, ...args: unknown[]) {
    if (!this.headersSent) {
      const index = typeof args[1] === "string" ? 2 : 1;
      const given = args[index];
      try {
        filterHead(this, args, index);
      } catch (error) {
        // The exit never throws for its own sake: whatever it cannot read
        // leaves without any Set-Cookie line (fail-closed), and Node judges
        // the rest of the head as without the lock.
        logger.error("session_cookie_exit_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        dropSetCookies(this, args, index, given);
      }
    }
    return Reflect.apply(writeHead, this, args) as ServerResponse;
  } as WriteHead;
  locked[LOCKED] = true;
  ServerResponse.prototype.writeHead = locked;
  // Node exposes this legacy alias at runtime, but @types/node omits it.
  Reflect.set(ServerResponse.prototype, "writeHeader", locked);
}

/**
 * Installs the lock on every HTTP(S) server and response of this process.
 * instrumentation.ts calls it once in the Node runtime, before Next handles
 * a request. Node writes every response head through writeHead or its
 * writeHeader alias, also implicitly on the first write or end, so no
 * response leaves past it.
 */
export function installSessionCookieExit(): void {
  if (isSessionCookieExitLocked()) return;
  lockEmit(Server.prototype);
  lockEmit(HttpsServer.prototype);
  lockWriteHead();
  logger.info("session_cookie_exit_locked");
}
