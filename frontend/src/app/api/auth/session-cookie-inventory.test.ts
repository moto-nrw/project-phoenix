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
// The checkout does not change during the run: every file is stripped once,
// not once per pattern searched across all files.
const stripped = new Map<string, string>();
/** Source without comments: a comment that names Set-Cookie writes nothing. */
const code = (file: string) => {
  let source = stripped.get(file);
  if (source === undefined) {
    source = readFileSync(path.join(ROOT, file), "utf8")
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .replace(/(^|[^:"'`])\/\/.*$/gm, "$1");
    stripped.set(file, source);
  }
  return source;
};
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

/** The one place that creates an Auth.js instance. */
const AUTH_FACTORY = "src/server/auth/portal-auth.ts";

/** The six portals' entry modules, each with its own Auth.js instance. */
const AUTH_INSTANCES: Record<string, string> = {
  "src/server/auth/caterer.ts": "caterer",
  "src/server/auth/index.ts": "tenant",
  "src/server/auth/operator.ts": "operator",
  "src/server/auth/parent.ts": "parent",
  "src/server/auth/school.ts": "school",
  "src/server/auth/traeger.ts": "traeger",
};

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
    expect(matching(/\bNextAuth\(/)).toEqual([AUTH_FACTORY]);
    const factory = code(AUTH_FACTORY);
    expect(factory.match(/\bNextAuth\(/g)).toHaveLength(1);
    const instance = /const (\w+) = NextAuth\(/.exec(factory)?.[1];
    expect(instance).toBeDefined();
    const uses = (member: string) => [
      ...factory.matchAll(new RegExp(`\\b${instance}\\.${member}\\b`, "g")),
    ];
    // The instance never leaves the factory whole; of its members only the
    // handlers (wrapped by the Auth.js routes) are handed out.
    expect(factory).not.toMatch(
      new RegExp(`(\\.\\.\\.|return |: |\\()${instance}\\b(?!\\.)`),
    );
    expect(
      [
        ...new Set(
          [...factory.matchAll(new RegExp(`\\b${instance}\\.(\\w+)`, "g"))].map(
            ([, member]) => member,
          ),
        ),
      ].sort(),
    ).toEqual(["auth", "handlers", "signIn", "unstable_update"]);
    // The raw auth only feeds the finalizer; its handler overload writes
    // cookies to its own response.
    expect(uses("auth")).toHaveLength(1);
    expect(factory).toMatch(
      new RegExp(`createResponseAwareAuth\\(\\s*${instance}\\.auth,`),
    );
    // Auth.js' server helpers write Next's cookie jar; only mutateSession
    // may call them, and it hands their cookies to the exit.
    expect(factory).not.toMatch(/\bsignOut\b/);
    for (const member of ["signIn", "unstable_update"]) {
      const calls = [
        ...factory.matchAll(new RegExp(`\\b${instance}\\.${member}\\(`, "g")),
      ];
      expect(calls, member).toHaveLength(1);
      for (const call of calls)
        expect(
          factory.slice(Math.max(0, call.index - 200), call.index),
          member,
        ).toMatch(/mutateSession\(/);
    }
    // One instance per portal, each in its own entry module.
    expect(
      matching(/\bcreatePortalAuth\(/).filter((file) => file !== AUTH_FACTORY),
    ).toEqual(Object.keys(AUTH_INSTANCES).sort());
    for (const [file, portal] of Object.entries(AUTH_INSTANCES))
      expect(code(file).match(/\bcreatePortalAuth\([^)]*\)/g), file).toEqual([
        `createPortalAuth(PORTALS.${portal})`,
      ]);
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
      // writes (#5520); it writes none. Its exports read whether the lock is
      // installed on ServerResponse#writeHead (module-export, checked below);
      // none of them hands a writer out.
      "src/lib/session-cookie-exit.server.ts": {
        kinds: ["cookie-header", "module-export"],
        proof: /const admitted = admitSetCookies\(response, lines\);/,
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
      // Test support only: a reused Vitest worker returns document to its
      // first state after every test file and empties the happy-dom cookie
      // jar, as a fresh worker would start. It sets no cookie.
      "src/test/setup-worker-reuse.ts": {
        kinds: ["document-cookie"],
        proof: /clearCookies: \(\) => container\.clearCookies\(\),/,
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
    // Only the lock at the exit, whose exports read its own installation.
    expect(writersOf("module-export")).toEqual([
      "src/lib/session-cookie-exit.server.ts",
    ]);
    const lock = code("src/lib/session-cookie-exit.server.ts");
    expect(
      [
        ...lock.matchAll(/^export (?:async function|function|const) (\w+)/gm),
      ].map(([, name]) => name),
    ).toEqual([
      "PORTAL_SESSION_COOKIE_NAMES",
      "tenantSessionCookieName",
      "isSessionCookieName",
      "isWrittenSessionCookieName",
      "isSessionCookieExitLocked",
      "approveSessionCookieWrites",
      "installSessionCookieExit",
    ]);
    expect(lock).not.toMatch(/^export (?!function|const)/m);
    // The portal table takes every session cookie name from the lock, the
    // one source of what the lock and the readers match; the six Auth.js
    // configs and the layout snapshot read it from the table.
    expect(
      matching(/\b(?:PORTAL_SESSION_COOKIE_NAMES|tenantSessionCookieName)\b/),
    ).toEqual([
      // Auth.js' default name, for the tenant cookie on localhost.
      "src/lib/portal-session-snapshot.server.ts",
      "src/lib/session-cookie-exit.server.ts",
      "src/server/auth/portals.ts",
    ]);
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
      // The six portals' session cookie names (the portal table takes them
      // from here), isSessionCookieName and isWrittenSessionCookieName, #5520.
      "src/lib/session-cookie-exit.server.ts",
      // The portal table derives each portal's callback and CSRF cookie
      // names from its session cookie name.
      "src/server/auth/portals.ts",
      // The gate's own grouping of session cookie chunks, unchanged.
      "src/server/auth/session-cookie-writes.ts",
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
      // The portal table: names from the lock for the Auth.js configs and
      // the layout snapshot, neither of which writes a cookie.
      "src/server/auth/portals.ts",
      "src/server/auth/route-handler.ts", // listed writer
      "src/server/auth/session-cookie-writes.ts", // the exit
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
    // Production (instrumentation.ts) and the shared test setup, which runs
    // every test against the locked server as production does.
    expect(matching(/\binstallSessionCookieExit\(/)).toEqual([
      "src/instrumentation.ts",
      "src/lib/session-cookie-exit.server.ts",
      "src/test/setup-common.ts",
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
