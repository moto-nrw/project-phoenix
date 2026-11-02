import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { encode } from "next-auth/jwt";
import { env } from "~/env";
import { withBrowserSession } from "~/server/auth/browser-session";
import { isSessionCookieName } from "~/server/auth/session-cookie-writes";
import { DEMO_TOKEN_COOKIE } from "../demo/access/forward";
import { operatorInvitationCookieName } from "~/lib/operator/operator-invitation-session.server";
import { LOCALE_COOKIE_NAME } from "~/i18n/locales";
import {
  INVENTORY_ROOTS,
  inventoryFiles,
  inventoryOf,
  type CookieInventory,
  type CookieWrite,
  type CookieWriteKind,
} from "~/test/session-cookie-writers";

// #5399, re-review of #5403: every session cookie of every portal leaves
// through one exit, gateSessionCookieWrites. This inventory lists the known
// ways to write a session cookie and fails when a new one it recognises
// passes the exit. #5444: the writers come from the syntax tree
// (src/test/session-cookie-writers.ts), with aliases, destructuring and
// JavaScript files. It is a tripwire for known patterns, not a guarantee;
// its limits are listed there. At run time the lock at the server's exit
// (src/lib/session-cookie-exit.server.ts, #5520) drops every session cookie write the gate
// did not approve.

const ROOT = process.cwd();
/** Source without comments: a comment that names Set-Cookie writes nothing. */
const code = (file: string) =>
  readFileSync(path.join(ROOT, file), "utf8")
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|[^:"'`])\/\/.*$/gm, "$1");
const ALL = inventoryFiles(ROOT, INVENTORY_ROOTS);
const matching = (pattern: RegExp) =>
  ALL.filter((file) => pattern.test(code(file))).sort();

/** Both inventories of every file, read once per run (see beforeAll). */
const inventories = new Map<string, CookieInventory>();
function readInventories() {
  for (const file of ALL)
    inventories.set(
      file,
      inventoryOf(file, readFileSync(path.join(ROOT, file), "utf8")),
    );
}
/** Every cookie write of the inventory, by file. */
function cookieWritesByFile(): Map<string, CookieWrite[]> {
  expect(inventories.size).toBe(ALL.length);
  return new Map(
    [...inventories]
      .filter(([, inventory]) => inventory.writes.length)
      .map(([file, inventory]) => [file, inventory.writes]),
  );
}
const writersOf = (kind: CookieWriteKind) =>
  [...cookieWritesByFile()]
    .filter(([, found]) => found.some((write) => write.kind === kind))
    .map(([file]) => file)
    .sort();
const kindsOf = (file: string) =>
  [
    ...new Set((cookieWritesByFile().get(file) ?? []).map((w) => w.kind)),
  ].sort();

/** The six portals' Auth.js routes: signin, callbacks, session, update(), … */
const AUTH_ROUTES = [
  "src/app/api/auth/[...nextauth]/route.ts",
  "src/app/api/caterer/auth/[...nextauth]/route.ts",
  "src/app/api/operator/auth/[...nextauth]/route.ts",
  "src/app/api/parent/auth/[...nextauth]/route.ts",
  "src/app/api/school/auth/[...nextauth]/route.ts",
  "src/app/api/traeger/auth/[...nextauth]/route.ts",
];

/** The six portals' Auth.js instances. */
const AUTH_INSTANCES = [
  "src/server/auth/caterer.ts",
  "src/server/auth/index.ts",
  "src/server/auth/operator.ts",
  "src/server/auth/parent.ts",
  "src/server/auth/school.ts",
  "src/server/auth/traeger.ts",
];

describe("Auth.js actions", () => {
  const NAME = "parent.session-token";
  const backend = vi.fn<typeof fetch>();

  beforeEach(() => {
    backend.mockReset();
    backend.mockImplementation(async () =>
      Response.json({ id: 7 }, { status: 401 }),
    );
    vi.stubGlobal("fetch", backend);
  });
  afterEach(() => vi.unstubAllGlobals());

  function authCoreActions() {
    const fromNextAuth = createRequire(
      createRequire(import.meta.url).resolve("next-auth"),
    );
    const core = path.dirname(fromNextAuth.resolve("@auth/core"));
    const source = readFileSync(
      path.join(core, "lib/utils/actions.js"),
      "utf8",
    );
    return [...source.matchAll(/^\s*"([a-z-]+)",$/gm)].map(
      ([, action]) => action,
    );
  }

  it("knows every action of the pinned Auth.js", () => {
    // A new action is a new way to write a session cookie: list it here
    // once withBrowserSession covers it.
    expect(authCoreActions()).toEqual([
      "providers",
      "session",
      "csrf",
      "signin",
      "signout",
      "callback",
      "verify-request",
      "error",
      "webauthn-options",
    ]);
  });

  it.each(["GET", "POST"])(
    "passes the session cookie of every action through the exit (%s)",
    async (method) => {
      const seal = (token: string) =>
        encode({
          token: { id: "7", token, refreshToken: "refresh-1" },
          secret: env.NEXTAUTH_SECRET,
          salt: NAME,
        });
      const presented = `${NAME}=${await seal("access-1")}`;
      const written = `${NAME}=${await seal("access-2")}; Path=/`;
      const route = withBrowserSession(async () => {
        const answer = new Response(null);
        answer.headers.append("Set-Cookie", written);
        return answer;
      });

      for (const action of authCoreActions()) {
        backend.mockClear();
        const path =
          action === "callback" ? "callback/parent-credentials" : action;
        const answer = await route(
          new Request(`http://parents.localhost:3000/api/parent/auth/${path}`, {
            method,
            headers: { cookie: presented },
          }),
        );

        expect(answer.headers.getSetCookie(), action).toEqual([]);
        expect(backend, action).toHaveBeenCalledOnce();
      }
    },
  );
});

describe("session cookie writers", () => {
  // Parsing every source file is setup, not a check; under a full parallel
  // run it takes longer than one test's budget.
  beforeAll(readInventories, 120_000);

  it("wraps both methods of every portal's Auth.js route", () => {
    expect(matching(/\b\w*[hH]andlers\.(GET|POST)\b/)).toEqual(AUTH_ROUTES);
    expect(ALL.filter((file) => file.includes("[...nextauth]")).sort()).toEqual(
      AUTH_ROUTES,
    );
    for (const file of AUTH_ROUTES) {
      const exports = [...code(file).matchAll(/export const (\w+)/g)].map(
        ([, name]) => name,
      );
      expect(exports, file).toEqual(["GET", "POST"]);
      for (const method of ["GET", "POST"])
        expect(code(file), file).toMatch(
          new RegExp(
            `export const ${method} = withPrivateNoStore\\(\\s*withBrowserSession\\(\\w+\\.${method}\\),?\\s*\\);`,
          ),
        );
    }
  });

  it("keeps every Auth.js instance behind the response finalizer", () => {
    expect(matching(/\bNextAuth\(/)).toEqual(AUTH_INSTANCES);
    for (const file of AUTH_INSTANCES) {
      const source = code(file);
      const raw = /auth:\s*(\w+),/.exec(source)?.[1];
      expect(raw, file).toBeDefined();
      // The raw auth only feeds the finalizer; its handler overload writes
      // cookies to its own response.
      expect(
        [...source.matchAll(new RegExp(`\\b${raw}\\b`, "g"))].length,
        file,
      ).toBe(2);
      expect(source, file).toMatch(
        new RegExp(`createResponseAwareAuth\\(${raw},`),
      );
      // Auth.js' server helpers write Next's cookie jar; only mutateSession
      // may call them, and it hands their cookies to the exit.
      expect(source, file).not.toMatch(/\bsignOut\b/);
      for (const call of source.matchAll(/\b(signIn|unstable_update)\(/g))
        expect(
          source.slice(Math.max(0, call.index - 200), call.index),
          file,
        ).toMatch(/mutateSession\(|mutate\(/);
    }
    expect(code("src/server/auth/session-writer.ts")).toMatch(
      /await mutate\(\s*\(\) =>\s*signIn\(/,
    );
    // Nobody else reaches the raw helpers.
    expect(
      matching(
        /import\s*\{[^}]*\b(signIn|unstable_update|signOut)\b[^}]*\}\s*from\s*"~\/server\/auth/,
      ),
    ).toEqual([]);
    expect(matching(/\brawAuth\(/)).toEqual([
      "src/server/auth/route-handler.ts",
    ]);
  });

  it("takes session cookies out of Next's cookie jar only at the exit", () => {
    expect(matching(/getModifiedCookieValues/)).toEqual([
      "src/server/auth/session-cookie-writes.ts",
    ]);
    expect(matching(/takeJarSessionCookies\(/)).toEqual([
      "src/server/auth/route-handler.ts",
      "src/server/auth/session-cookie-writes.ts",
    ]);
    // Everyone else only reads the jar: any other use of cookies() of
    // next/headers, through an alias, destructuring, a string key or a
    // helper, counts as a write.
    expect(writersOf("next-cookie-jar")).toEqual([
      "src/server/auth/session-cookie-writes.ts",
    ]);
  });

  it("lets only listed code write a cookie, and no session cookie", () => {
    const writers: Record<
      string,
      { kinds: CookieWriteKind[]; proof: RegExp | null }
    > = {
      // The exit itself: takes the jar's session cookies (Next's internal
      // cookie adapter) and serializes them.
      "src/server/auth/session-cookie-writes.ts": {
        kinds: [
          "cookie-header",
          "cookie-library",
          "cookie-list",
          "next-cookie-jar",
        ],
        proof: null,
      },
      // The lock at the server's exit only drops unapproved session cookie
      // writes (#5520); it writes none.
      "src/lib/session-cookie-exit.server.ts": {
        kinds: ["cookie-header"],
        proof: /const admitted = admitSetCookies\(lines\);/,
      },
      // finishResponse composes the route's cookies, then calls the exit.
      "src/server/auth/route-handler.ts": {
        kinds: ["cookie-header"],
        proof: null,
      },
      // The backend's own cookies (trusted device), without session cookies.
      "src/lib/auth-proxy.server.ts": {
        kinds: ["cookie-header"],
        proof: /withoutSessionCookies\(/,
      },
      "src/lib/login-route.server.ts": {
        kinds: ["cookie-header"],
        proof: /withoutSessionCookies\(/,
      },
      // Cookies of their own, never a session cookie (checked below).
      "src/app/api/demo/access/sessions/route.ts": {
        kinds: ["cookie-list"],
        proof: /DEMO_TOKEN_COOKIE,/,
      },
      "src/app/api/operator/auth/invitations/accept/route.ts": {
        kinds: ["cookie-list"],
        proof: /operatorInvitationCookieName\(/,
      },
      "src/app/api/operator/auth/invitations/session/route.ts": {
        kinds: ["cookie-list"],
        proof: /operatorInvitationCookieName\(/,
      },
      // The parents portal's language, in the browser (#5444: the regex
      // inventory did not see document.cookie).
      "src/i18n/locales.ts": {
        kinds: ["document-cookie"],
        proof: /document\.cookie = `\$\{LOCALE_COOKIE_NAME\}=/,
      },
    };
    expect([...cookieWritesByFile().keys()].sort()).toEqual(
      Object.keys(writers).sort(),
    );
    for (const [file, { kinds, proof }] of Object.entries(writers)) {
      expect(kindsOf(file), file).toEqual(kinds);
      if (proof) expect(code(file), file).toMatch(proof);
    }
    // A writer handing cookies() or a Set-Cookie name to another module
    // would let that module write unlisted.
    expect(writersOf("module-export")).toEqual([]);
    expect(isSessionCookieName(DEMO_TOKEN_COOKIE)).toBe(false);
    expect(
      isSessionCookieName(operatorInvitationCookieName(randomUUID())),
    ).toBe(false);
    expect(isSessionCookieName(LOCALE_COOKIE_NAME)).toBe(false);
  });

  it("knows every place that names a session cookie", () => {
    // A second tripwire for patterns the writer inventory cannot see: new
    // code that spells a session cookie name, matches it or reads Auth.js'
    // sessionToken cookie options needs a review and an entry here.
    expect(inventories.size).toBe(ALL.length);
    const sources = ALL.filter(
      (file) => inventories.get(file)?.sessionCookieNames.length,
    );
    expect(sources).toEqual([
      // The option sessionToken carries the access token, no cookie name.
      "src/lib/auth-proxy.server.ts",
      // Layout snapshots read the session cookie by its configured name.
      "src/lib/caterer-session-snapshot.server.ts",
      "src/lib/operator-session-snapshot.server.ts",
      "src/lib/parent-session-snapshot.server.ts",
      "src/lib/school-session-snapshot.server.ts",
      // isSessionCookieName, of the gate and the lock at the exit.
      "src/lib/session-cookie-exit.server.ts",
      "src/lib/tenant-session-snapshot.server.ts",
      "src/lib/traeger-session-snapshot.server.ts",
      // The six portals' Auth.js configs name their cookie.
      "src/server/auth/caterer-config.ts",
      "src/server/auth/operator-config.ts",
      "src/server/auth/parent-config.ts",
      "src/server/auth/school-config.ts",
      "src/server/auth/tenant-config.ts",
      "src/server/auth/traeger-config.ts",
      // The inventory's own pattern.
      "src/test/session-cookie-writers.ts",
    ]);
  });

  it("lets only listed modules export a Set-Cookie or session cookie name", () => {
    // An importer of such an export could write the cookie without spelling
    // its name, past the writer list above.
    expect(inventories.size).toBe(ALL.length);
    const exporters = ALL.filter(
      (file) => inventories.get(file)?.exportedCookieNames.length,
    );
    expect(exporters).toEqual([
      "src/lib/auth-proxy.server.ts", // listed writer
      "src/lib/login-route.server.ts", // listed writer
      "src/lib/session-cookie-exit.server.ts", // the lock at the exit
      "src/server/auth/caterer-config.ts", // names its portal cookie
      "src/server/auth/operator-config.ts", // names its portal cookie
      "src/server/auth/parent-config.ts", // names its portal cookie
      "src/server/auth/route-handler.ts", // listed writer
      "src/server/auth/school-config.ts", // names its portal cookie
      "src/server/auth/session-cookie-writes.ts", // the exit
      "src/server/auth/tenant-config.ts", // names its portal cookie
      "src/server/auth/traeger-config.ts", // names its portal cookie
      "src/test/session-cookie-writers.ts", // the inventory's own pattern
    ]);
  });

  it("approves session cookie writes only in the gate, locks the exit once (#5520)", () => {
    // An approval lets a session cookie pass the lock at the server's exit.
    expect(matching(/\bapproveSessionCookieWrites\(/)).toEqual([
      "src/lib/session-cookie-exit.server.ts",
      "src/server/auth/session-cookie-writes.ts",
    ]);
    expect(
      code("src/server/auth/session-cookie-writes.ts").match(
        /\bapproveSessionCookieWrites\(/g,
      ),
    ).toHaveLength(1);
    expect(matching(/\binstallSessionCookieExit\(/)).toEqual([
      "src/instrumentation.ts",
      "src/lib/session-cookie-exit.server.ts",
    ]);
  });

  it("has two callers of the exit: Auth.js' routes and every BFF route", () => {
    const callers = ALL.filter((file) =>
      /gateSessionCookieWrites\(/.test(
        code(file).replace(
          /export async function gateSessionCookieWrites\(/,
          "",
        ),
      ),
    ).sort();
    expect(callers).toEqual([
      "src/server/auth/browser-session.ts",
      "src/server/auth/route-handler.ts",
    ]);
    const finalizer = code("src/server/auth/route-handler.ts");
    // withAuthResponse and withSessionResponse both end in finishResponse,
    // and finishResponse ends in the exit.
    expect(finalizer.match(/return finishResponse\(/g)).toHaveLength(2);
    expect(finalizer).toMatch(
      /return gateSessionCookieWrites\(current\.request, response\);\s*\}/,
    );
  });
});
