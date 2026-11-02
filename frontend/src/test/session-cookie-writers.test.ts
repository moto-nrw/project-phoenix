import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import {
  INVENTORY_ROOTS,
  cookieWrites,
  inventoryFiles,
  sessionCookieNameSources,
  type CookieWriteKind,
} from "./session-cookie-writers";

// #5444: the regex inventory of #5403 missed aliases, destructuring and
// JavaScript files. Each probe is a session cookie writer that bypasses
// gateSessionCookieWrites: the old inventory passed it, the syntax tree
// inventory rejects it.

/**
 * The regex inventory before #5444, kept as the baseline: the syntax tree
 * inventory flags at least every file this one flags.
 */
function legacyCookieWriter(file: string, source: string): boolean {
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

const kinds = (file: string, source: string) =>
  [...new Set(cookieWrites(file, source).map((write) => write.kind))].sort();

const JAR = `import { cookies } from "next/headers";\n`;
const SERVER = `import { NextResponse } from "next/server";\n`;
const WRITE = `"parent.session-token", "sealed"`;

type Probe = [name: string, file: string, source: string, CookieWriteKind[]];

const BYPASSES: Probe[] = [
  [
    "an alias of the cookie jar",
    "route.ts",
    `${JAR}export async function POST() {
  const store = await cookies();
  store.set(${WRITE});
  return new Response(null);
}`,
    ["next-cookie-jar"],
  ],
  [
    "a cookie jar assigned after its declaration",
    "route.ts",
    `${JAR}export async function POST() {
  let c;
  c = await cookies();
  c.delete("parent.session-token");
}`,
    ["next-cookie-jar"],
  ],
  [
    "a destructured jar method",
    "route.ts",
    `${JAR}export async function POST() {
  const { set } = await cookies();
  set(${WRITE});
}`,
    ["next-cookie-jar"],
  ],
  [
    "a renamed destructured jar method",
    "route.ts",
    `${JAR}export async function POST() {
  const { delete: drop } = await cookies();
  drop("parent.session-token");
}`,
    ["next-cookie-jar"],
  ],
  [
    "a jar method by string key",
    "route.ts",
    `${JAR}export async function POST() {
  (await cookies())["set"](${WRITE});
}`,
    ["next-cookie-jar"],
  ],
  [
    "the jar handed to a helper",
    "route.ts",
    `${JAR}import { writeSession } from "./write";
export async function POST() {
  writeSession(await cookies());
}`,
    ["next-cookie-jar"],
  ],
  [
    "the jar returned from a helper",
    "jar.ts",
    `${JAR}export async function jar() {
  return cookies();
}`,
    ["next-cookie-jar"],
  ],
  [
    "a renamed cookies import",
    "route.ts",
    `import { cookies as requestCookies } from "next/headers";
export async function POST() {
  (await requestCookies()).set(${WRITE});
}`,
    ["next-cookie-jar"],
  ],
  [
    "the jar promise resolved with then",
    "route.ts",
    `${JAR}export function POST() {
  return cookies().then((c) => c.set(${WRITE}));
}`,
    ["next-cookie-jar"],
  ],
  [
    "cookies of a dynamic import of next/headers",
    "route.ts",
    `export async function POST() {
  const { cookies: read } = await import("next/headers");
  (await read()).set(${WRITE});
}`,
    ["next-cookie-jar"],
  ],
  [
    "the cookies function re-exported",
    "headers.ts",
    `export { cookies } from "next/headers";`,
    ["module-export"],
  ],
  [
    "the cookies function exported under another name",
    "headers.ts",
    `${JAR}export const jar = cookies;`,
    ["module-export"],
  ],
  [
    "an alias of a response's cookie list",
    "route.ts",
    `${SERVER}export function GET() {
  const response = NextResponse.json({});
  const list = response.cookies;
  list.set(${WRITE});
  return response;
}`,
    ["cookie-list"],
  ],
  [
    "a response's cookie list destructured",
    "route.ts",
    `${SERVER}export function GET() {
  const response = NextResponse.json({});
  const { cookies } = response;
  cookies.set(${WRITE});
  return response;
}`,
    ["cookie-list"],
  ],
  [
    "a renamed destructured cookie list method",
    "route.ts",
    `${SERVER}export function GET() {
  const response = NextResponse.json({});
  const { set: put } = response.cookies;
  put(${WRITE});
  return response;
}`,
    ["cookie-list"],
  ],
  [
    "an optional call on a cookie list",
    "route.ts",
    `${SERVER}export function GET() {
  const response = NextResponse.json({});
  response.cookies?.set(${WRITE});
  return response;
}`,
    ["cookie-list"],
  ],
  [
    "a cookie list by string key",
    "route.ts",
    `${SERVER}export function GET() {
  const response = NextResponse.json({});
  response["cookies"].set(${WRITE});
  return response;
}`,
    ["cookie-list"],
  ],
  [
    "a cookie list destructured in a parameter",
    "write.ts",
    `import type { NextResponse } from "next/server";
export function write({ cookies }: NextResponse) {
  cookies.set(${WRITE});
}`,
    ["cookie-list"],
  ],
  [
    "Next's ResponseCookies over raw headers",
    "route.ts",
    `import { ResponseCookies } from "next/server";
export function GET() {
  const headers = new Headers();
  new ResponseCookies(headers).set(${WRITE});
  return new Response(null, { headers });
}`,
    ["cookie-list"],
  ],
  [
    "the browser's cookieStore through window",
    "client.ts",
    `export async function write() {
  await window.cookieStore.set(${WRITE});
}`,
    ["cookie-list"],
  ],
  [
    "the browser's global cookieStore",
    "client.ts",
    `export async function drop() {
  await cookieStore.delete("parent.session-token");
}`,
    ["cookie-list"],
  ],
  [
    "document.cookie",
    "client.ts",
    `export function write() {
  document.cookie = "parent.session-token=sealed; path=/";
}`,
    ["document-cookie"],
  ],
  [
    "an alias of document",
    "client.ts",
    `export function write() {
  const page = document;
  page.cookie = "parent.session-token=sealed; path=/";
}`,
    ["document-cookie"],
  ],
  [
    "document through window by string key",
    "client.ts",
    `export function write() {
  window["document"].cookie = "parent.session-token=sealed; path=/";
}`,
    ["document-cookie"],
  ],
  [
    "document destructured from window",
    "client.ts",
    `export function write() {
  const { document: page } = window;
  Object.assign(page, { cookie: "parent.session-token=sealed" });
}`,
    ["document-cookie"],
  ],
  [
    "document handed to Reflect",
    "client.ts",
    `export function write() {
  Reflect.set(document, "cookie", "parent.session-token=sealed");
}`,
    ["document-cookie"],
  ],
  ...(["route.js", "route.mjs", "route.cjs", "client.jsx"] as const).map(
    (file): Probe => [
      `a writer in ${file}`,
      file,
      `export function GET(request) {
  const response = new Response(null);
  response.cookies.set(${WRITE});
  return response;
}`,
      ["cookie-list"],
    ],
  ),
  [
    "a Set-Cookie header name spelled in parts",
    "route.ts",
    `export function GET() {
  const response = new Response(null);
  response.headers.append("Set-" + "Cookie", "parent.session-token=sealed");
  return response;
}`,
    ["cookie-header"],
  ],
  [
    "a Set-Cookie header name in a template",
    "route.ts",
    `export function GET() {
  const response = new Response(null);
  response.headers.append(\`set-\${"cookie"}\`, "parent.session-token=sealed");
  return response;
}`,
    ["cookie-header"],
  ],
  [
    "a writer between strings that look like a block comment",
    "route.ts",
    `${SERVER}export function GET() {
  const response = NextResponse.json({});
  const glob = "/*"; response.cookies.set(${WRITE}); const end = "*/";
  return response;
}`,
    ["cookie-list"],
  ],
  [
    "a writer after a string with two slashes",
    "route.ts",
    `${SERVER}export function GET() {
  const response = NextResponse.json({});
  const path = 'a//b'; response.cookies.set(${WRITE});
  return response;
}`,
    ["cookie-list"],
  ],
];

/** Writers the regex inventory caught; the syntax tree inventory must too. */
const CAUGHT: Probe[] = [
  [
    "a jar method on cookies()",
    "route.ts",
    `${JAR}export async function POST() {
  (await cookies()).set(${WRITE});
}`,
    ["next-cookie-jar"],
  ],
  [
    "a jar named cookieStore",
    "route.ts",
    `${JAR}export async function POST() {
  const cookieStore = await cookies();
  cookieStore.delete("parent.session-token");
}`,
    ["next-cookie-jar"],
  ],
  [
    "a response's cookie list",
    "route.ts",
    `${SERVER}export function GET() {
  const response = NextResponse.json({});
  response.cookies.set(${WRITE});
  response.cookies.delete("parent.session-token");
  return response;
}`,
    ["cookie-list"],
  ],
  [
    "a Set-Cookie header",
    "route.ts",
    `export function GET() {
  const response = new Response(null);
  response.headers.append("Set-Cookie", "parent.session-token=sealed");
  return response;
}`,
    ["cookie-header"],
  ],
  [
    "a renamed destructured header method",
    "route.ts",
    `export function GET() {
  const response = new Response(null);
  const { append: add } = response.headers;
  add("set-cookie", "parent.session-token=sealed");
  return response;
}`,
    ["cookie-header"],
  ],
  [
    "a Set-Cookie header on headers() of next/headers",
    "route.ts",
    `import { headers } from "next/headers";
export async function POST() {
  (await headers()).set("set-cookie", "parent.session-token=sealed");
}`,
    ["cookie-header"],
  ],
  [
    "a Set-Cookie header in a response's init",
    "route.ts",
    `export function GET() {
  return new Response(null, {
    headers: { "Set-Cookie": "parent.session-token=sealed" },
  });
}`,
    ["cookie-header"],
  ],
  [
    "Next's middleware Set-Cookie header",
    "proxy.ts",
    `${SERVER}export function proxy() {
  return NextResponse.next({
    headers: { "x-middleware-set-cookie": "parent.session-token=sealed" },
  });
}`,
    ["cookie-header"],
  ],
];

const READERS: [name: string, file: string, source: string][] = [
  [
    "a jar read on cookies()",
    "page.tsx",
    `${JAR}export default async function Page() {
  return (await cookies()).get("locale")?.value;
}`,
  ],
  [
    "reads through an alias of the jar",
    "page.tsx",
    `${JAR}export default async function Page() {
  const store = await cookies();
  return [store.get("a")?.value, store.getAll().length, store.has("b"), store.size];
}`,
  ],
  [
    "a destructured jar read",
    "page.tsx",
    `${JAR}export default async function Page() {
  const { get } = await cookies();
  return get("locale")?.value;
}`,
  ],
  [
    "a request's cookie read",
    "route.ts",
    `import type { NextRequest } from "next/server";
export function GET(request: NextRequest) {
  return request.cookies.get("demo")?.value;
}`,
  ],
  [
    "a destructured request cookie list read",
    "route.ts",
    `import type { NextRequest } from "next/server";
export function GET(request: NextRequest) {
  const { cookies } = request;
  return cookies.get("demo")?.value;
}`,
  ],
  [
    "the session cookie options of an Auth.js config",
    "snapshot.ts",
    `import { parentAuthConfig } from "./parent-config";
export const name = parentAuthConfig.cookies.sessionToken.name;`,
  ],
  [
    "a cookie list cleared on an error report",
    "sentry.ts",
    `export function scrub(event: { request?: { cookies?: object } }) {
  if (event.request?.cookies) event.request.cookies = {};
}`,
  ],
  [
    "a document.cookie read",
    "client.ts",
    `export const raw = () => document.cookie.split("; ");`,
  ],
  [
    "a Set-Cookie list read",
    "route.ts",
    `export const all = (response: Response) => response.headers.getSetCookie();`,
  ],
  [
    "writers named only in comments",
    "route.ts",
    `// response.cookies.set(name, value)
/* document.cookie = "parent.session-token=sealed" */
export {};`,
  ],
  [
    "set on maps and search parameters",
    "route.ts",
    `export function build() {
  const params = new URLSearchParams();
  params.set("cookie", "1");
  return new Map().set("a", params);
}`,
  ],
  [
    "a parameter shadowing the cookies import",
    "helper.ts",
    `${JAR}export function put(cookies: Map<string, string>) {
  cookies.set("a", "b");
}`,
  ],
  [
    "a local variable named cookieStore",
    "client.ts",
    `export function remember() {
  const cookieStore = new Map<string, string>();
  cookieStore.set("a", "b");
  return cookieStore;
}`,
  ],
  [
    "ResponseCookies as a type",
    "types.ts",
    `import type { ResponseCookies } from "next/server";
export type Jar = ResponseCookies;`,
  ],
];

describe("session cookie writers the regex inventory missed (#5444)", () => {
  it.each(BYPASSES)("rejects %s", (_, file, source, expected) => {
    expect(legacyCookieWriter(file, source)).toBe(false);
    expect(kinds(file, source)).toEqual(expected);
  });

  it("rejects a Set-Cookie header name that leaves its module", () => {
    // A listed writer exporting the name would let an unlisted module write
    // the header without spelling it.
    const importer = `import { SET_COOKIE } from "~/server/auth/route-handler";
export function GET() {
  const response = new Response(null);
  response.headers.append(SET_COOKIE, "parent.session-token=sealed");
  return response;
}`;
    expect(legacyCookieWriter("route.ts", importer)).toBe(false);
    expect(
      kinds("route-handler.ts", `export const SET_COOKIE = "Set-Cookie";`),
    ).toEqual(["cookie-header", "module-export"]);
    expect(
      kinds(
        "route-handler.ts",
        `const SET_COOKIE = "Set-Cookie";\nexport { SET_COOKIE };`,
      ),
    ).toEqual(["cookie-header", "module-export"]);
  });
});

describe("session cookie writers the regex inventory caught", () => {
  it.each(CAUGHT)("still rejects %s", (_, file, source, expected) => {
    expect(legacyCookieWriter(file, source)).toBe(true);
    expect(kinds(file, source)).toEqual(expected);
  });
});

describe("cookie readers", () => {
  it.each(READERS)("accepts %s", (_, file, source) => {
    expect(legacyCookieWriter(file, source)).toBe(false);
    expect(cookieWrites(file, source)).toEqual([]);
  });
});

describe("cookieWrites", () => {
  it("flags every file of the inventory the regex inventory flagged", () => {
    const root = process.cwd();
    const missed = inventoryFiles(root, INVENTORY_ROOTS).filter((file) => {
      const source = readFileSync(path.join(root, file), "utf8");
      return (
        legacyCookieWriter(file, source) && !cookieWrites(file, source).length
      );
    });
    expect(missed).toEqual([]);
  });

  it("names the line and the code of each write", () => {
    expect(
      cookieWrites(
        "route.ts",
        `${JAR}export async function POST() {
  const store = await cookies();
  store.set(${WRITE});
  store.get("a");
}`,
      ),
    ).toEqual([{ kind: "next-cookie-jar", line: 4, code: "store.set" }]);
  });
});

describe("sessionCookieNameSources", () => {
  const lines = (source: string) =>
    sessionCookieNameSources("config.ts", source).map(({ line }) => line);

  it("finds every way to name a session cookie", () => {
    expect(
      lines(`export const a = { name: "parent.session-token" };
export const b = (prefix: string) => \`\${prefix}.session-token\`;
export const c = /session-token(?:\\.\\d+)?$/;
export const d = (config: Config) => config.cookies.sessionToken.name;
export const e = ({ sessionToken }: Config["cookies"]) => sessionToken;
export const f = "parent.session" + "-token";
export const g = (config: Config) => config.cookies["sessionToken"];`),
    ).toEqual([1, 2, 3, 4, 5, 6, 7]);
  });

  it("ignores comments and the name check", () => {
    expect(
      lines(`// parent.session-token
/* traeger.session-token */
import { isSessionCookieName } from "./session-cookie-writes";
export const session = (name: string) => isSessionCookieName(name);`),
    ).toEqual([]);
  });
});
