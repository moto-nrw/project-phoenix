import "server-only";
import { decode } from "next-auth/jwt";
import { env } from "~/env";
import {
  portalOfScope,
  sessionFamilyState,
  type SessionFamilyState,
} from "./token-validation";

/**
 * Session cookie writes of every portal's session route and BFF routes
 * (#5399). Auth.js re-encodes the session cookie on every session read, and a
 * response that arrives after a logout or a school switch deleted or
 * replaced the cookie wrote the old session back. See gateSessionCookieWrites.
 */
/** Claims Auth.js sets on every encode; they change on every write. */
const ENCODE_CLAIMS = new Set(["iat", "exp", "jti"]);

interface CookieWrite {
  name: string;
  value: string;
  deletes: boolean;
}

function parseSetCookie(header: string): CookieWrite | null {
  const [pair, ...attributes] = header.split(";");
  const separator = pair?.indexOf("=") ?? -1;
  if (!pair || separator <= 0) return null;
  const name = pair.slice(0, separator).trim();
  const value = pair.slice(separator + 1).trim();
  const deletes =
    value === "" ||
    attributes.some((attribute) => {
      const [key, raw] = attribute.split("=", 2).map((part) => part.trim());
      if (key?.toLowerCase() === "max-age") return Number(raw) <= 0;
      if (key?.toLowerCase() === "expires")
        return Date.parse(raw ?? "") <= Date.now();
      return false;
    });
  return { name, value, deletes };
}

/** `name` or one of its chunks `name.0`, `name.1`, … */
function chunkIndex(cookie: string, base: string): number | null {
  if (cookie === base) return 0;
  if (!cookie.startsWith(`${base}.`)) return null;
  const suffix = cookie.slice(base.length + 1);
  return /^\d+$/.test(suffix) ? Number(suffix) : null;
}

function baseName(cookie: string): string {
  return cookie.replace(/\.\d+$/, "");
}

/** Auth.js' SessionStore joins the chunks in index order. */
function joinChunks(cookies: Iterable<[string, string]>, base: string) {
  const chunks: [number, string][] = [];
  for (const [name, value] of cookies) {
    const index = chunkIndex(name, base);
    if (index !== null) chunks.push([index, value]);
  }
  if (!chunks.length) return null;
  return chunks
    .sort(([a], [b]) => a - b)
    .map(([, value]) => value)
    .join("");
}

function requestCookies(request: Request): [string, string][] {
  const header = request.headers.get("cookie");
  if (!header) return [];
  return header.split(";").flatMap((part) => {
    const separator = part.indexOf("=");
    if (separator <= 0) return [];
    return [
      [part.slice(0, separator).trim(), part.slice(separator + 1).trim()],
    ];
  });
}

function stable(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(stable);
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>)
        .filter(
          ([key, entry]) => !ENCODE_CLAIMS.has(key) && entry !== undefined,
        )
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([key, entry]) => [key, stable(entry)]),
    );
  }
  return value;
}

async function readSession(token: string, salt: string) {
  try {
    return (await decode({
      token,
      salt,
      secret: env.NEXTAUTH_SECRET,
    })) as Record<string, unknown> | null;
  } catch {
    // Not a session Auth.js can read back either: it carries no session.
    return null;
  }
}

/** The family state per access token, asked once per response. */
export type FamilyStates = Map<string, Promise<SessionFamilyState>>;

export function familyStateOf(
  states: FamilyStates,
  session: Record<string, unknown>,
): Promise<SessionFamilyState> {
  const token = typeof session.token === "string" ? session.token : "";
  let state = states.get(token);
  if (!state) {
    state = sessionFamilyState(token, portalOfScope(session.scope));
    states.set(token, state);
  }
  return state;
}

/** Whether a written session may reach the browser (#5399). */
async function mayWrite(
  written: Record<string, unknown> | null,
  presented: Record<string, unknown> | null,
  states: FamilyStates,
): Promise<boolean> {
  // Auth.js cannot read it back either; it carries no session.
  if (!written) return true;
  // A mere re-encoding of the presented session.
  if (
    presented &&
    JSON.stringify(stable(written)) === JSON.stringify(stable(presented))
  )
    return false;
  // An ended session: the response body reports the end; the cookie stays
  // as the browser holds it, possibly already deleted or replaced.
  if (written.error || typeof written.token !== "string" || !written.token)
    return false;
  // A new or renewed session only while its family is live, asked now.
  return (await familyStateOf(states, written)) === "live";
}

/**
 * The server invariant of #5399: a response writes a session cookie only for
 * a session that changed (sign-in, rotation, update()) and whose
 * refresh-token family Identity & Access confirms live while the response is
 * built. A re-encoded unchanged session, an ended one and one of a revoked or
 * unreadable family set nothing, whenever the request started and however
 * late its answer arrives. Deletions and every other cookie pass unchanged.
 */
export async function gateSessionCookieWrites(
  request: Request,
  response: Response,
  states: FamilyStates = new Map(),
): Promise<Response> {
  const setCookies = response.headers.getSetCookie();
  const writes = setCookies.map(parseSetCookie);
  const bases = new Set(
    writes
      .filter((write): write is CookieWrite => write !== null)
      .map((write) => baseName(write.name))
      .filter((base) => base.endsWith("session-token")),
  );
  if (!bases.size) return response;

  const presented = requestCookies(request);
  const refused = new Set<string>();
  for (const base of bases) {
    const own = writes.filter(
      (write): write is CookieWrite =>
        write !== null && chunkIndex(write.name, base) !== null,
    );
    // Auth.js cleans obsolete chunks alongside replacements. Only the final
    // non-deleted values form the session to validate; cleanup is not a bypass.
    const latest = new Map<string, string>();
    for (const write of own) {
      if (write.deletes) latest.delete(write.name);
      else latest.set(write.name, write.value);
    }
    if (!latest.size) continue;
    const written = joinChunks(latest, base);
    const before = joinChunks(presented, base);
    const [next, previous] = await Promise.all([
      written ? readSession(written, base) : null,
      before ? readSession(before, base) : null,
    ]);
    if (!(await mayWrite(next, previous, states))) refused.add(base);
  }
  if (!refused.size) return response;

  response.headers.delete("Set-Cookie");
  setCookies.forEach((cookie, index) => {
    const write = writes[index];
    if (!write || write.deletes || !refused.has(baseName(write.name)))
      response.headers.append("Set-Cookie", cookie);
  });
  return response;
}
