import { readFileSync } from "node:fs";
import path from "node:path";
import { beforeAll, describe, expect, it } from "vitest";
import {
  INVENTORY_ROOTS,
  cookieWrites,
  inventoryFiles,
  inventoryOf,
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
    "a cookie list under a constant key",
    "route.ts",
    `import { NextResponse, type NextRequest } from "next/server";
export function GET(request: NextRequest) {
  const key = "cookies";
  const response = NextResponse.json({ n: request[key].getAll().length });
  response[key].set(${WRITE});
  return response;
}`,
    ["cookie-list"],
  ],
  [
    "a cookie list under a template literal key",
    "route.ts",
    `${SERVER}const part = "cook";
export function GET() {
  const response = NextResponse.json({});
  response[\`\${part}ies\`].set(${WRITE});
  return response;
}`,
    ["cookie-list"],
  ],
  [
    "a response member under a variable key",
    "write.ts",
    `import type { NextResponse } from "next/server";
export function write(target: NextResponse, key: string) {
  target[key].set(${WRITE});
}`,
    ["computed-member"],
  ],
  [
    "a member of a route's response under a computed key",
    "route.ts",
    `${SERVER}import { pick } from "./pick";
export function GET() {
  const answer = NextResponse.json({});
  answer[pick()].set(${WRITE});
  return answer;
}`,
    ["computed-member"],
  ],
  [
    "a member of an untyped request under a variable key",
    "route.js",
    `export function POST(request, key) {
  return request[key];
}`,
    ["computed-member"],
  ],
  [
    "a member of headers() under a variable key",
    "route.ts",
    `import { headers } from "next/headers";
export async function POST(key: string) {
  const list = await headers();
  list[key]("x", "y");
}`,
    ["computed-member"],
  ],
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
    "a request's cookie list read under a constant key",
    "route.ts",
    `import type { NextRequest } from "next/server";
const key = "cookies";
export function GET(request: NextRequest) {
  return request[key].getAll().length;
}`,
  ],
  [
    "computed keys on plain data",
    "route.ts",
    `export function pick(params: Record<string, string>, key: string, index: number) {
  const parts = key.split("/");
  return [params[key], parts[index + 1]];
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

describe("the regex inventory as a baseline", () => {
  const missed: string[] = [];
  let read = 0;
  // Parsing every source file is setup, not a check; under a full parallel
  // run it takes longer than one test's budget.
  beforeAll(() => {
    const root = process.cwd();
    for (const file of inventoryFiles(root, INVENTORY_ROOTS)) {
      const source = readFileSync(path.join(root, file), "utf8");
      read++;
      if (
        legacyCookieWriter(file, source) &&
        !cookieWrites(file, source).length
      )
        missed.push(file);
    }
  }, 120_000);

  it("flags every file of the inventory the regex inventory flagged", () => {
    expect(read).toBeGreaterThan(1000);
    expect(missed).toEqual([]);
  });
});

describe("cookieWrites", () => {
  it.each([
    "`Set-Cookie${string}`",
    "`${string}Set-Cookie${string}`",
    "`${string}Set-Cookie`",
  ])("handles the template literal type %s safely", (literal) => {
    expect(kinds("header.ts", `export type Header = ${literal};`)).toEqual([
      "cookie-header",
    ]);
  });

  it.each([
    "`Set-Cookie`",
    "`Set-Cookie${suffix}`",
    "`${prefix}Set-Cookie${suffix}`",
    "`${prefix}Set-Cookie`",
  ])("tracks the template header name %s through exports", (literal) => {
    expect(kinds("route.ts", `headers.set(${literal}, value);`)).toEqual([
      "cookie-header",
    ]);
    expect(kinds("header.ts", `export const name = ${literal};`)).toEqual([
      "cookie-header",
      "module-export",
    ]);
  });

  it.each(["module", "function"])(
    "tracks a block-contained var alias in %s scope",
    (scope) => {
      const block = `if (true) {
  var store = await cookies();
  store.set(${WRITE});
}`;
      const source =
        scope === "module" ? block : `async function write() { ${block} }`;
      expect(kinds("route.ts", `${JAR}${source}`)).toEqual(["next-cookie-jar"]);
    },
  );

  it("respects a block-scoped declaration shadowing a var alias", () => {
    expect(
      cookieWrites(
        "route.ts",
        `${JAR}async function read() {
  if (true) {
    var store = await cookies();
    store.get("locale");
    {
      const store = new Map();
      store.set("a", "b");
    }
  }
}`,
      ),
    ).toEqual([]);
  });

  it.each(["module", "function"])(
    "tracks var redeclarations of the same binding in %s scope",
    (scope) => {
      for (const body of [
        `var jar = await cookies(); { var jar; jar.set(${WRITE}); }`,
        `{ var jar = await cookies(); } var jar; jar.set(${WRITE});`,
        `var jar = await cookies(); for (var jar; false;) { jar.set(${WRITE}); }`,
      ]) {
        const source = `${JAR}${scope === "module" ? body : `async function write() { ${body} }`}`;
        expect(legacyCookieWriter("route.ts", source)).toBe(true);
        expect(kinds("route.ts", source)).toEqual(["next-cookie-jar"]);
      }
    },
  );

  it("respects a function-scoped var shadowing an outer var alias", () => {
    expect(
      cookieWrites(
        "route.ts",
        `${JAR}async function read() {
  var store = await cookies();
  store.get("locale");
  function remember() {
    var store = new Map();
    store.set("a", "b");
  }
}`,
      ),
    ).toEqual([]);
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

describe("writes to read-only members", () => {
  it.each([
    [
      "a jar",
      `${JAR}const store = await cookies();`,
      "store",
      "get",
      "next-cookie-jar",
    ],
    [
      "a response list",
      `const list = response.cookies;`,
      "list",
      "getAll",
      "cookie-list",
    ],
    ["cookieStore", "", "cookieStore", "has", "cookie-list"],
    [
      "cookie options",
      `const list = config.cookies;`,
      "list",
      "sessionToken",
      "cookie-list",
    ],
  ] as const)(
    "tracks assignments and deletions on %s",
    (_, setup, target, member, kind) => {
      for (const operation of [
        `${target}.${member} = writeCookie; ${target}.${member}(name, value);`,
        `delete ${target}["${member}"];`,
        `({ method: ${target}["${member}"] } = replacement);`,
      ]) {
        expect(kinds("writer.ts", `${setup}\n${operation}`)).toEqual([kind]);
      }
    },
  );
});

describe("exported cookie names", () => {
  // A module exporting a Set-Cookie or session cookie name lets an unlisted
  // importer write the cookie without spelling its name.
  const exported = (source: string) =>
    inventoryOf("names.ts", source).exportedCookieNames.map(({ line }) => line);

  it.each([
    ["an object", `export const names = { header: "Set-Cookie" };`],
    ["an array", `export const names = ["Set-Cookie"];`],
    [
      "a function's return",
      `export function header() {\n  return "Set-Cookie";\n}`,
    ],
    [
      "an arrow function's return",
      `export const name = () => "parent.session-token";`,
    ],
    [
      "a re-export of a local binding",
      `const names = { header: "Set-Cookie" };\nexport { names as cookieNames };`,
    ],
    ["a default export", `export default { cookie: "parent.session-token" };`],
    [
      "a constant the export references",
      `const NAME = "parent.session-token";\nexport const config = { cookie: NAME };`,
    ],
    [
      "a helper the export calls",
      `function name() {\n  return "Set-" + "Cookie";\n}\nexport const header = () => name();`,
    ],
    [
      "a destructured object binding",
      `const { name } = { name: "Set-Cookie" }; export { name };`,
    ],
    [
      "a renamed nested binding",
      `const { nested: { name: header } } = { nested: { name: "Set-Cookie" } }; export { header };`,
    ],
    [
      "an array binding through an alias",
      `const names = ["parent.session-token"]; const [name] = names; export default name;`,
    ],
    [
      "a default binding value",
      `const { name = "Set-Cookie" } = {}; export { name };`,
    ],
    [
      "a nested default binding value",
      `const { nested: { name } = { name: "Set-Cookie" } } = {}; module.exports = name;`,
    ],
    ["CommonJS", `module.exports = { header: "Set-Cookie" };`],
    ["a session cookie pattern", `export const pattern = /session-token$/;`],
  ])("rejects %s", (_, source) => {
    expect(exported(source)).not.toEqual([]);
  });

  it("names the line of the export", () => {
    expect(
      exported(`const NAME = "parent.session-token";
export const other = 1;
export const config = { cookie: NAME };`),
    ).toEqual([3]);
  });

  it.each([
    ["another header", `export const names = { header: "Content-Type" };`],
    [
      "a name the exports never reach",
      `const NAME = "parent.session-token";\nexport const other = () => 1;`,
    ],
    ["a type", `export type Header = "Set-Cookie";`],
  ])("accepts %s", (_, source) => {
    expect(exported(source)).toEqual([]);
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
