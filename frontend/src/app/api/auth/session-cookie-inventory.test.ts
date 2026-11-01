import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { randomUUID } from "node:crypto";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { encode } from "next-auth/jwt";
import { env } from "~/env";
import { withBrowserSession } from "~/server/auth/browser-session";
import { isSessionCookieName } from "~/server/auth/session-cookie-writes";
import { DEMO_TOKEN_COOKIE } from "../demo/access/forward";
import { operatorInvitationCookieName } from "~/lib/operator/operator-invitation-session.server";

// #5399, re-review of #5403: every session cookie of every portal leaves
// through one exit, gateSessionCookieWrites. This inventory names every way
// a session cookie can be written and fails when a new one passes the exit.

const ROOT = process.cwd();
const SRC = path.join(ROOT, "src");

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const absolute = path.join(dir, entry);
    if (statSync(absolute).isDirectory()) return sourceFiles(absolute);
    if (!/\.(ts|tsx)$/.test(entry) || /\.(test|spec)\.tsx?$/.test(entry))
      return [];
    return [absolute];
  });
}

const relative = (file: string) => path.relative(ROOT, file);
/** Source without comments: a comment that names Set-Cookie writes nothing. */
const code = (file: string) =>
  readFileSync(path.join(ROOT, file), "utf8")
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|[^:"'`])\/\/.*$/gm, "$1");
const ALL = sourceFiles(SRC).map(relative);
const matching = (pattern: RegExp) =>
  ALL.filter((file) => pattern.test(code(file))).sort();

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
    // Everyone else only reads the jar.
    for (const file of matching(/\bcookies\(\)/))
      expect(code(file), file).not.toMatch(
        /cookies\(\)\)?\.(set|delete)\(|cookieStore\.(set|delete)\(|jar\.(set|delete)\(/,
      );
  });

  it("lets only listed code write a cookie, and no session cookie", () => {
    const writers = {
      // The exit itself.
      "src/server/auth/session-cookie-writes.ts": null,
      // finishResponse composes the route's cookies, then calls the exit.
      "src/server/auth/route-handler.ts": null,
      // The backend's own cookies (trusted device), without session cookies.
      "src/lib/auth-proxy.server.ts": /withoutSessionCookies\(/,
      "src/lib/login-route.server.ts": /withoutSessionCookies\(/,
      // Cookies of their own, never a session cookie (checked below).
      "src/app/api/demo/access/sessions/route.ts": /DEMO_TOKEN_COOKIE,/,
      "src/app/api/operator/auth/invitations/accept/route.ts":
        /operatorInvitationCookieName\(/,
      "src/app/api/operator/auth/invitations/session/route.ts":
        /operatorInvitationCookieName\(/,
    } as const;
    expect(
      matching(
        /set-cookie|\.cookies\.(set|delete)\(|\.(set|append)\(\s*["'`]set-cookie/i,
      ),
    ).toEqual(Object.keys(writers).sort());
    for (const [file, pattern] of Object.entries(writers))
      if (pattern) expect(code(file), file).toMatch(pattern);
    expect(isSessionCookieName(DEMO_TOKEN_COOKIE)).toBe(false);
    expect(
      isSessionCookieName(operatorInvitationCookieName(randomUUID())),
    ).toBe(false);
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
