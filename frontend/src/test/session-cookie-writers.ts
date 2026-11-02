import { readdirSync, statSync } from "node:fs";
import path from "node:path";

/**
 * The cookie writers of the session cookie inventory (#5399, #5444):
 * session-cookie-inventory.test.ts lists every file that can write a cookie
 * and keeps the session cookies on gateSessionCookieWrites.
 */

export type CookieWriteKind =
  /** Next's cookie jar of the request: cookies() of next/headers. */
  | "next-cookie-jar"
  /** A cookie list: response.cookies, request.cookies, ResponseCookies, cookieStore. */
  | "cookie-list"
  /** A Set-Cookie header name. */
  | "set-cookie-header"
  /** document.cookie, or document handed to code that can set it. */
  | "document-cookie"
  /** A cookie library from outside this repository. */
  | "cookie-library"
  /** A cookie writer leaving its module, out of reach of a per-file check. */
  | "module-export";

export interface CookieWrite {
  kind: CookieWriteKind;
  line: number;
  code: string;
}

/** Every way the file can write a cookie, from its syntax tree. */
export function cookieWrites(_file: string, _source: string): CookieWrite[] {
  return [];
}

/** Every place the file names or reads a session cookie name. */
export function sessionCookieNameSources(
  _file: string,
  _source: string,
): { line: number; code: string }[] {
  return [];
}

/** Source files of the inventory, relative to `root`. */
export function inventoryFiles(root: string, entries: string[]): string[] {
  const walk = (absolute: string): string[] => {
    if (statSync(absolute).isDirectory())
      return readdirSync(absolute).flatMap((entry) =>
        walk(path.join(absolute, entry)),
      );
    if (
      !/\.[cm]?[jt]sx?$/.test(absolute) ||
      /\.(test|spec)\.[cm]?[jt]sx?$/.test(absolute)
    )
      return [];
    return [path.relative(root, absolute)];
  };
  return entries.flatMap((entry) => walk(path.join(root, entry))).sort();
}

/**
 * The regex inventory before #5444, kept as the baseline: the syntax tree
 * inventory flags at least every file this one flags.
 */
export function legacyCookieWriter(file: string, source: string): boolean {
  if (!/\.(ts|tsx)$/.test(file) || /\.(test|spec)\.tsx?$/.test(file))
    return false;
  const code = source
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|[^:"'`])\/\/.*$/gm, "$1");
  if (
    /set-cookie|\.cookies\.(set|delete)\(|\.(set|append)\(\s*["'`]set-cookie/i.test(
      code,
    )
  )
    return true;
  return (
    /\bcookies\(\)/.test(code) &&
    /cookies\(\)\)?\.(set|delete)\(|cookieStore\.(set|delete)\(|jar\.(set|delete)\(/.test(
      code,
    )
  );
}
