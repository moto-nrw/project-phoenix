import "server-only";
import { decode } from "next-auth/jwt";
import { env } from "~/env";

/**
 * Auth.js re-encodes the session cookie on every session read: the session
 * route (poll, update()) and every BFF route through `auth(handler)` answer
 * with a fresh Set-Cookie even when nothing changed. Such an answer that
 * arrives after a logout or a school switch deleted or replaced the cookie
 * wrote the old session back (#5399). A response therefore keeps its
 * session Set-Cookie only when the session itself changed: sign-in,
 * rotation, update() or its end. The cookie's lifetime only moves with a
 * change then; it equals the refresh lifetime, which moves with a rotation.
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

async function sessionContent(token: string, salt: string) {
  try {
    const payload = await decode({
      token,
      salt,
      secret: env.NEXTAUTH_SECRET,
    });
    return payload ? JSON.stringify(stable(payload)) : null;
  } catch {
    // Unreadable: no proof that nothing changed.
    return null;
  }
}

/**
 * Removes the session Set-Cookie of a response that only re-encodes the
 * session the request presented. Every other cookie, a deletion, a cookie
 * the request did not carry or one that cannot be read stays: those are
 * changes or cannot be shown to be none.
 */
export async function dropUnchangedSessionCookie(
  request: Request,
  response: Response,
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
  const unchanged = new Set<string>();
  for (const base of bases) {
    const own = writes.filter(
      (write): write is CookieWrite =>
        write !== null && chunkIndex(write.name, base) !== null,
    );
    if (own.some((write) => write.deletes)) continue;
    // The last write of a name wins in the browser.
    const latest = new Map(own.map((write) => [write.name, write.value]));
    const written = joinChunks(latest, base);
    const before = joinChunks(presented, base);
    if (!written || !before) continue;
    const [previous, next] = await Promise.all([
      sessionContent(before, base),
      sessionContent(written, base),
    ]);
    if (previous !== null && previous === next) unchanged.add(base);
  }
  if (!unchanged.size) return response;

  response.headers.delete("Set-Cookie");
  setCookies.forEach((cookie, index) => {
    const write = writes[index];
    if (!write || !unchanged.has(baseName(write.name)))
      response.headers.append("Set-Cookie", cookie);
  });
  return response;
}
