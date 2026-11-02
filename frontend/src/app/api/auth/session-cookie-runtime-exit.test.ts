import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { createServer, get, ServerResponse, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { encode, getToken } from "next-auth/jwt";
import { NextResponse } from "next/server";
import {
  NodeNextRequest,
  NodeNextResponse,
} from "next/dist/server/base-http/node";
import { sendResponse } from "next/dist/server/send-response";
import { env } from "~/env";
import {
  installSessionCookieExit,
  isSessionCookieExitLocked,
  isSessionCookieName,
  isWrittenSessionCookieName,
} from "~/lib/session-cookie-exit.server";
import { catererAuthConfig } from "~/server/auth/caterer-config";
import { operatorAuthConfig } from "~/server/auth/operator-config";
import { parentAuthConfig } from "~/server/auth/parent-config";
import { schoolAuthConfig } from "~/server/auth/school-config";
import { traegerAuthConfig } from "~/server/auth/traeger-config";
import { setTestClock } from "~/test/clock";
import { randomUUID } from "node:crypto";
import { LOCALE_COOKIE_NAME } from "~/i18n/locales";
import { operatorInvitationCookieName } from "~/lib/operator/operator-invitation-session.server";
import { DEMO_TOKEN_COOKIE } from "../demo/access/forward";
import { gateSessionCookieWrites } from "~/server/auth/session-cookie-writes";

// #5520: invariant I1 of #5399 at run time. Whatever code builds a response,
// a session cookie leaves the Node server only when gateSessionCookieWrites
// approved it while handling the same request; a deletion always leaves. The
// routes below run on a real Node server with the lock instrumentation.ts
// installs (setup-common.ts installs it for every test, as in production),
// and Next's own sendResponse writes their responses, as it does for every
// route handler.

const NAME = "parent.session-token";
const SESSION = {
  id: "7",
  name: "Erika Beispiel",
  token: "access-1",
  refreshToken: "refresh-1",
  scope: "parent",
  tokenExpiry: Date.now() + 10 * 60_000,
  refreshTokenExpiry: Date.now() + 24 * 60 * 60_000,
};
const DELETION = `${NAME}=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=0`;

const backend = vi.fn<typeof fetch>();
/** The answer of Identity & Access on the session's family. */
let familyStatus = 200;

function sealed(token: Record<string, unknown> = SESSION) {
  return encode({ token, secret: env.NEXTAUTH_SECRET, salt: NAME });
}

/** The request of a browser without a session cookie. */
const browserRequest = () =>
  new Request("http://parents.localhost:3000/api/parent/auth/session");

/** One route per path, answered through Next's sendResponse. */
let routes = new Map<string, () => Promise<Response>>();
// Node exposes this legacy alias at runtime, but @types/node omits it.
type RawResponse = ServerResponse & {
  writeHeader: ServerResponse["writeHead"];
};
/** Responses whose head a route writes itself, past sendResponse. */
let rawRoutes = new Map<string, (res: RawResponse) => unknown>();

installSessionCookieExit();

const server: Server = createServer((req, res) => {
  const path = req.url ?? "";
  const raw = rawRoutes.get(path);
  if (raw) return void raw(res as RawResponse);
  const route = routes.get(path);
  if (!route) return res.writeHead(404).end();
  void route().then((response) =>
    sendResponse(new NodeNextRequest(req), new NodeNextResponse(res), response),
  );
});

/** The gate's approval of `line`, in the request it runs in. */
async function approve(line: string) {
  const answer = new Response(null, { headers: { "Set-Cookie": line } });
  const gated = await gateSessionCookieWrites(browserRequest(), answer);
  expect(gated.headers.getSetCookie()).toEqual([line]);
}

/** The Set-Cookie lines the browser receives for a path. */
function setCookiesAt(path: string): Promise<string[]> {
  const { port } = server.address() as AddressInfo;
  return new Promise((resolve, reject) => {
    get({ host: "127.0.0.1", port, path }, (res) => {
      res.resume();
      res.on("end", () => resolve(res.headers["set-cookie"] ?? []));
    }).on("error", reject);
  });
}

const sessionCookies = (lines: string[]) =>
  lines.filter((line) => line.startsWith(`${NAME}=`));

beforeAll(async () => {
  vi.stubEnv("AUTH_SECRET", env.NEXTAUTH_SECRET);
  await new Promise<void>((resolve) =>
    server.listen(0, "127.0.0.1", () => resolve()),
  );
});
afterAll(async () => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  await new Promise((resolve) => server.close(resolve));
});
beforeEach(() => {
  routes = new Map();
  rawRoutes = new Map();
  familyStatus = 200;
  backend.mockReset();
  backend.mockImplementation(async (url) =>
    String(url).endsWith("/auth/session/validate")
      ? Response.json({ id: 7 }, { status: familyStatus })
      : new Response(null, { status: 404 }),
  );
  vi.stubGlobal("fetch", backend);
});

describe("the lock at the server's exit", () => {
  it("drops a session cookie a route writes past the gate, other cookies leave", async () => {
    const value = await sealed();
    routes.set("/bypass", async () => {
      const response = NextResponse.json({ ok: true });
      const key = ["coo", "kies"].join("") as "cookies";
      response[key].set(NAME, value, { path: "/", httpOnly: true });
      response.headers.append("Set-Cookie", "other=1; Path=/");
      return response;
    });

    const received = await setCookiesAt("/bypass");

    expect(received).toEqual(["other=1; Path=/"]);
    expect(backend).not.toHaveBeenCalled();
  });

  it("drops a session cookie of a backend response handed through unfiltered (#5517)", async () => {
    const value = await sealed();
    routes.set("/forward", async () => {
      const upstream = new Response("{}", {
        headers: [
          ["Content-Type", "application/json"],
          ["Set-Cookie", `${NAME}=${value}; Path=/; HttpOnly`],
          ["Set-Cookie", `${NAME}.0=${value}; Path=/; HttpOnly`],
          ["Set-Cookie", "trusted_device=abc; Path=/; HttpOnly"],
        ],
      });
      // private-no-store.ts copies an immutable fetch result like this.
      return new Response(upstream.body, upstream);
    });

    const received = await setCookiesAt("/forward");

    expect(received).toEqual(["trusted_device=abc; Path=/; HttpOnly"]);
  });

  it("lets a session cookie the gate approved arrive unchanged", async () => {
    const line = `${NAME}=${await sealed()}; Path=/; HttpOnly; SameSite=Lax`;
    routes.set("/sign-in", async () => {
      const answer = new Response(null);
      answer.headers.append("Set-Cookie", line);
      return gateSessionCookieWrites(browserRequest(), answer);
    });

    const received = await setCookiesAt("/sign-in");

    expect(received).toEqual([line]);
  });

  it("admits an approval once: the same line in a later response is dropped", async () => {
    const line = `${NAME}=${await sealed()}; Path=/; HttpOnly; SameSite=Lax`;
    routes.set("/sign-in", async () => {
      const answer = new Response(null);
      answer.headers.append("Set-Cookie", line);
      return gateSessionCookieWrites(browserRequest(), answer);
    });
    routes.set("/replay", async () => {
      const answer = new Response(null);
      answer.headers.append("Set-Cookie", line);
      return answer;
    });

    expect(await setCookiesAt("/sign-in")).toEqual([line]);
    expect(sessionCookies(await setCookiesAt("/replay"))).toEqual([]);
  });

  it("delivers no cookie of a family the gate found revoked", async () => {
    familyStatus = 401;
    const line = `${NAME}=${await sealed()}; Path=/; HttpOnly`;
    routes.set("/late", async () => {
      const answer = new Response(null);
      answer.headers.append("Set-Cookie", line);
      return gateSessionCookieWrites(browserRequest(), answer);
    });

    expect(sessionCookies(await setCookiesAt("/late"))).toEqual([]);
  });

  it("lets every deletion arrive, also past the gate", async () => {
    const cleared = NextResponse.json({ ok: true });
    cleared.cookies.delete(NAME);
    const [nextDeletion = ""] = cleared.headers.getSetCookie();
    routes.set("/sign-out", async () => {
      const answer = new Response(null);
      answer.headers.append("Set-Cookie", DELETION);
      answer.headers.append("Set-Cookie", `${NAME}.1=; Path=/; Max-Age=0`);
      answer.headers.append("Set-Cookie", nextDeletion);
      return answer;
    });

    expect(await setCookiesAt("/sign-out")).toEqual([
      DELETION,
      `${NAME}.1=; Path=/; Max-Age=0`,
      nextDeletion,
    ]);
  });

  it("drops writes that carry a value or a future lifetime next to a deletion", async () => {
    const value = await sealed();
    routes.set("/disguised", async () => {
      const answer = new Response(null);
      // The browser keeps these for an hour or as a session cookie.
      answer.headers.append(
        "Set-Cookie",
        `${NAME}=${value}; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=3600`,
      );
      answer.headers.append(
        "Set-Cookie",
        `${NAME}.0=${value}; Max-Age=0; Max-Age=3600`,
      );
      answer.headers.append("Set-Cookie", `${NAME}.1=${value}; Expires=0`);
      return answer;
    });

    expect(await setCookiesAt("/disguised")).toEqual([]);
  });

  it.each(["object", "pairs", "case-alias"])(
    "filters only effective headers of writeHead (%s)",
    async (form) => {
      const line = `${NAME}=${await sealed()}; Path=/; HttpOnly`;
      rawRoutes.set("/merged", async (res) => {
        await approve(line);
        res.setHeader("Set-Cookie", line);
        const headers =
          form === "pairs"
            ? ["Set-Cookie", line, "Set-Cookie", "other=1"]
            : form === "case-alias"
              ? { "Set-Cookie": line, "set-cookie": [line, "other=1"] }
              : { "Set-Cookie": [line, "other=1"] };
        res.writeHead(200, "OK", headers).end();
      });
      rawRoutes.set("/replay", (res) => {
        res.setHeader("Set-Cookie", line);
        res.end();
      });

      expect(await setCookiesAt("/merged")).toEqual([line, "other=1"]);
      expect(await setCookiesAt("/replay")).toEqual([]);
    },
  );

  it.each(["writeHead", "writeHeader"] as const)(
    "filters unapproved cookies through %s while preserving approved writes and deletions",
    async (method) => {
      const line = `${NAME}=${await sealed()}; Path=/; HttpOnly`;
      rawRoutes.set("/unapproved-cached", (res) => {
        res.setHeader("Set-Cookie", [line, DELETION, "other=1"]);
        res[method](200).end();
      });
      rawRoutes.set("/unapproved-explicit", (res) => {
        res[method](200, "OK", {
          "Set-Cookie": [line, DELETION, "other=1"],
        }).end();
      });
      rawRoutes.set("/approved", async (res) => {
        await approve(line);
        res[method](200, { "Set-Cookie": [line, DELETION, "other=1"] }).end();
      });

      expect(await setCookiesAt("/unapproved-cached")).toEqual([
        DELETION,
        "other=1",
      ]);
      expect(await setCookiesAt("/unapproved-explicit")).toEqual([
        DELETION,
        "other=1",
      ]);
      expect(await setCookiesAt("/approved")).toEqual([
        line,
        DELETION,
        "other=1",
      ]);
    },
  );

  it("drops a header writeHead overrides and keeps the approved one", async () => {
    const line = `${NAME}=${await sealed()}; Path=/; HttpOnly`;
    rawRoutes.set("/overridden", async (res) => {
      await approve(line);
      res.setHeader("Set-Cookie", line);
      res.writeHead(200, { "Set-Cookie": "other=1" }).end();
    });
    rawRoutes.set("/approved", async (res) => {
      await approve(line);
      res.setHeader("Set-Cookie", line);
      res.end();
    });

    expect(await setCookiesAt("/overridden")).toEqual(["other=1"]);
    expect(await setCookiesAt("/approved")).toEqual([line]);
  });

  it("binds an approval to its request: parallel requests with the same line both get it", async () => {
    const line = `${NAME}=${await sealed()}; Path=/; HttpOnly`;
    let approvals = 0;
    let bothApproved: () => void = () => {};
    const barrier = new Promise<void>((resolve) => (bothApproved = resolve));
    rawRoutes.set("/parallel", async (res) => {
      await approve(line);
      if (++approvals === 2) bothApproved();
      await barrier;
      res.setHeader("Set-Cookie", line);
      res.end();
    });

    const [first, second] = await Promise.all([
      setCookiesAt("/parallel"),
      setCookiesAt("/parallel"),
    ]);

    expect(first).toEqual([line]);
    expect(second).toEqual([line]);
  });

  it("keeps an approval until its response is written, however late", async () => {
    const line = `${NAME}=${await sealed()}; Path=/; HttpOnly`;
    rawRoutes.set("/late-flush", async (res) => {
      await approve(line);
      // A stream that flushes its head long after the gate ran.
      setTestClock(new Date(Date.now() + 61 * 60_000));
      res.setHeader("Set-Cookie", line);
      res.end();
    });

    expect(await setCookiesAt("/late-flush")).toEqual([line]);
  });

  it("approves nothing outside a request", async () => {
    const line = `${NAME}=${await sealed()}; Path=/; HttpOnly`;
    await approve(line);
    rawRoutes.set("/elsewhere", (res) => {
      res.setHeader("Set-Cookie", line);
      res.end();
    });

    expect(await setCookiesAt("/elsewhere")).toEqual([]);
  });

  it("repairs an unguarded writeHeader alias without wrapping writeHead again", () => {
    const prototype = ServerResponse.prototype as RawResponse;
    const locked = prototype.writeHead;
    const alias = prototype.writeHeader;
    prototype.writeHeader = function (
      this: ServerResponse,
      ...args: unknown[]
    ) {
      return Reflect.apply(locked, this, args) as ServerResponse;
    } as typeof locked;
    try {
      expect(isSessionCookieExitLocked()).toBe(false);
      installSessionCookieExit();
      expect(isSessionCookieExitLocked()).toBe(true);
      expect(prototype.writeHeader).toBe(locked);
      expect(prototype.writeHead).toBe(locked);
      installSessionCookieExit();
      expect(prototype.writeHead).toBe(locked);
      expect(prototype.writeHeader).toBe(locked);
    } finally {
      prototype.writeHeader = alias;
    }
  });

  it("fails the gate when the lock at the exit is missing", async () => {
    const locked = ServerResponse.prototype.writeHead;
    // What a server looks like when instrumentation.ts did not run.
    ServerResponse.prototype.writeHead = function (
      this: ServerResponse,
      ...args: unknown[]
    ) {
      return Reflect.apply(locked, this, args) as ServerResponse;
    } as typeof locked;
    try {
      expect(isSessionCookieExitLocked()).toBe(false);
      await expect(
        gateSessionCookieWrites(
          browserRequest(),
          new Response(null, {
            headers: { "Set-Cookie": `${NAME}=${await sealed()}` },
          }),
        ),
      ).rejects.toThrow(/session cookie exit lock is not installed/);
    } finally {
      ServerResponse.prototype.writeHead = locked;
    }
    expect(isSessionCookieExitLocked()).toBe(true);
  });

  it.each([
    ["a repeated Max-Age", `${NAME}=VALUE; Max-Age=0; Max-Age=+3600`],
    ["a repeated Max-Age, empty", `${NAME}=; Max-Age=0; Max-Age=+3600`],
    ["names in other case", `${NAME}=; max-age=0; MAX-AGE=3600`],
    ["a Max-Age with +", `${NAME}=; Max-Age=+0`],
    [
      "a repeated Expires",
      `${NAME}=VALUE; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Expires=Thu, 01 Jan 2099 00:00:00 GMT`,
    ],
    [
      "a repeated Expires, past last",
      `${NAME}=VALUE; Expires=Thu, 01 Jan 2099 00:00:00 GMT; Expires=Thu, 01 Jan 1970 00:00:00 GMT`,
    ],
    [
      "a repeated Expires, empty",
      `${NAME}=; Expires=Thu, 01 Jan 1970 00:00:00 GMT; expires=Thu, 01 Jan 2099 00:00:00 GMT`,
    ],
    [
      "an empty value with a long life",
      `${NAME}=; Expires=Thu, 01 Jan 2099 00:00:00 GMT`,
    ],
    ["an empty value with a positive Max-Age", `${NAME}=; Max-Age=3600`],
    ["an empty value without a lifetime", `${NAME}=; Path=/`],
    [
      "a past Expires against a long Max-Age",
      `${NAME}=; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=3600`,
    ],
    ["a value with Max-Age=0", `${NAME}=VALUE; Max-Age=0`],
    [
      "a value with a past Expires",
      `${NAME}=VALUE; Expires=Thu, 01 Jan 1970 00:00:00 GMT`,
    ],
  ])("needs an approval for a disguised deletion: %s", async (_, template) => {
    const line = template.replace("VALUE", await sealed());
    rawRoutes.set("/disguised", (res) => {
      res.setHeader("Set-Cookie", [line, "other=1"]);
      res.end();
    });

    expect(await setCookiesAt("/disguised")).toEqual(["other=1"]);
  });

  it.each([
    "Mon, 01 Jan 1500 00:00:00 GMT",
    "Mon, 01 Jan 1600 00:00:00 GMT",
    "Thu, 00 Jan 1970 00:00:00 GMT",
    "Thu, 32 Jan 1970 00:00:00 GMT",
    "Thu, 01 Jan 1970 24:00:00 GMT",
    "Thu, 01 Jan 1970 00:60:00 GMT",
    "Thu, 01 Jan 1970 00:00:60 GMT",
  ])(
    "does not treat an invalid cookie date as a deletion (%s)",
    async (expires) => {
      rawRoutes.set("/invalid-expiry", (res) => {
        res.setHeader("Set-Cookie", [
          `${NAME}=; Expires=${expires}`,
          "other=1",
        ]);
        res.end();
      });

      expect(await setCookiesAt("/invalid-expiry")).toEqual(["other=1"]);
    },
  );

  it("lets deletions at the valid date bounds arrive", async () => {
    const lines = [
      `${NAME}=; Expires=Mon, 01 Jan 1601 00:00:00 GMT`,
      `${NAME}=; Expires=Thu, 01 Jan 1970 23:59:59 GMT`,
      `${NAME}=; Max-Age=-1`,
    ];
    rawRoutes.set("/valid-expiry", (res) => {
      res.setHeader("Set-Cookie", lines);
      res.end();
    });

    expect(await setCookiesAt("/valid-expiry")).toEqual(lines);
  });

  it("covers heads a response writes itself, explicitly or on its first write", async () => {
    const value = await sealed();
    rawRoutes.set("/explicit-object", (res) =>
      res
        .writeHead(200, "OK", {
          "SET-COOKIE": [`${NAME}=${value}; Path=/`, "other=1"],
        })
        .end(),
    );
    rawRoutes.set("/explicit-pairs", (res) =>
      res.writeHead(200, ["Set-Cookie", `${NAME}=${value}; Path=/`]).end(),
    );
    rawRoutes.set("/implicit", (res) => {
      res.setHeader("Set-Cookie", `${NAME}=${value}; Path=/`);
      res.end("body");
    });

    expect(await setCookiesAt("/explicit-object")).toEqual(["other=1"]);
    expect(await setCookiesAt("/explicit-pairs")).toEqual([]);
    expect(await setCookiesAt("/implicit")).toEqual([]);
  });
});

// #5520 re-check: Auth.js' SessionStore (also behind getToken) takes every
// cookie whose name starts with the configured name as a session chunk. The
// lock treats exactly those names (behind any __Secure-/__Host- prefix) as
// session cookies, and only the names Auth.js writes can be approved.
describe("session cookie names at the exit", () => {
  const TENANT_DOMAIN = "moto-app.de";
  /** Every configured session cookie name, the tenant one for TENANT_DOMAIN. */
  const CONFIGURED = [
    catererAuthConfig.cookies.sessionToken.name,
    operatorAuthConfig.cookies.sessionToken.name,
    parentAuthConfig.cookies.sessionToken.name,
    schoolAuthConfig.cookies.sessionToken.name,
    traegerAuthConfig.cookies.sessionToken.name,
    "moto-app-de.session-token",
    "authjs.session-token",
  ];
  const PREFIXES = ["", "__Secure-", "__Host-", "__secure-", "__HOST-"];
  /** Names a reader takes for the session next to the configured name. */
  const READER_SUFFIXES = [
    "",
    ".0",
    ".evil",
    "_shadow",
    ".0extra",
    ".0.1",
    "x",
    "-1",
    ".evil.session-token",
    "-x.session-token",
  ];
  const PROBES = CONFIGURED.flatMap((base) =>
    PREFIXES.flatMap((prefix) =>
      READER_SUFFIXES.map((suffix) => `${prefix}${base}${suffix}`),
    ),
  );
  let tenantDomain: string | undefined;
  beforeEach(() => {
    tenantDomain = env.TENANT_DOMAIN;
    (env as { TENANT_DOMAIN?: string }).TENANT_DOMAIN = TENANT_DOMAIN;
  });
  afterEach(() => {
    (env as { TENANT_DOMAIN?: string }).TENANT_DOMAIN = tenantDomain;
  });

  it("a reader takes a cookie whose name only starts with the session name", async () => {
    const value = await sealed();
    for (const suffix of ["", ".evil", "_shadow", ".0extra", "x", "-1"]) {
      const token = await getToken({
        req: { headers: { cookie: `${NAME}${suffix}=${value}` } },
        secret: env.NEXTAUTH_SECRET,
        salt: NAME,
        cookieName: NAME,
        secureCookie: false,
      });
      expect(token?.token, suffix).toBe(SESSION.token);
    }
  });

  it("knows every configured name, also the tenant one for TENANT_DOMAIN", async () => {
    vi.resetModules();
    vi.doMock("~/env", async () => ({
      env: { ...env, TENANT_DOMAIN },
    }));
    try {
      const { tenantAuthConfig } = await import("~/server/auth/tenant-config");
      const tenant = (
        tenantAuthConfig.cookies as { sessionToken?: { name: string } }
      ).sessionToken?.name;
      expect(tenant).toBe("moto-app-de.session-token");
    } finally {
      vi.doUnmock("~/env");
      vi.resetModules();
    }
    for (const base of CONFIGURED) {
      expect(isWrittenSessionCookieName(base), base).toBe(true);
      expect(isWrittenSessionCookieName(`${base}.0`), base).toBe(true);
      expect(isWrittenSessionCookieName(`${base}.12`), base).toBe(true);
      expect(isWrittenSessionCookieName(`__Secure-${base}`), base).toBe(true);
    }
  });

  it(`treats all ${PROBES.length} reader names as session cookies, approves only written ones`, () => {
    for (const name of PROBES) {
      expect(isSessionCookieName(name), name).toBe(true);
      const suffix = READER_SUFFIXES.find((s) => s && name.endsWith(s));
      if (suffix && suffix !== ".0")
        expect(isWrittenSessionCookieName(name), name).toBe(false);
    }
  });

  it("drops every reader name written past the gate", async () => {
    const value = await sealed();
    rawRoutes.set("/names", (res) => {
      res.setHeader("Set-Cookie", [
        ...PROBES.map((name) => `${name}=${value}; Path=/`),
        "other=1",
      ]);
      res.end();
    });

    expect(await setCookiesAt("/names")).toEqual(["other=1"]);
  });

  it.each([".evil", "_shadow", ".0extra", ".0.1", ".evil.session-token"])(
    "drops %s even when the gate let it go with a live family",
    async (suffix) => {
      const value = await sealed();
      const line = `${NAME}${suffix}=${value}; Path=/; HttpOnly`;
      routes.set("/gated", async () =>
        gateSessionCookieWrites(
          browserRequest(),
          new Response(null, { headers: { "Set-Cookie": line } }),
        ),
      );

      expect(await setCookiesAt("/gated")).toEqual([]);
    },
  );

  it("lets deletions of such names arrive", async () => {
    const lines = READER_SUFFIXES.map(
      (suffix) => `${NAME}${suffix}=; Path=/; Max-Age=0`,
    );
    rawRoutes.set("/clear", (res) => {
      res.setHeader("Set-Cookie", lines);
      res.end();
    });

    expect(await setCookiesAt("/clear")).toEqual(lines);
  });

  it("leaves foreign cookies with similar names alone", async () => {
    const value = await sealed();
    const names = [
      "parent-session-token",
      "xparent.session-token",
      "session-token",
      "PARENT.SESSION-TOKEN",
      "Parent.session-token",
      "my.session-token",
      "__Secure-session-token",
      "parent.session",
      "parent.session_token",
      "parent.sessiontoken",
      "parent.session-tok",
      "moto-app.session-token",
      "authjs.session",
    ];
    const lines = names.map((name) => `${name}=${value}; Path=/`);
    rawRoutes.set("/similar", (res) => {
      res.setHeader("Set-Cookie", lines);
      res.end();
    });

    expect(await setCookiesAt("/similar")).toEqual(lines);
  });

  it("leaves every other cookie of the app alone", async () => {
    const portals = [
      "caterer",
      "operator",
      "parent",
      "school",
      "traeger",
      "moto-app-de",
    ];
    const names = [
      ...portals.flatMap((p) => [`${p}.callback-url`, `${p}.csrf-token`]),
      // Auth.js' defaults of the tenant portal on localhost and https.
      "authjs.csrf-token",
      "__Host-authjs.csrf-token",
      "authjs.callback-url",
      "__Secure-authjs.callback-url",
      "authjs.pkce.code_verifier",
      "authjs.state",
      "authjs.nonce",
      "authjs.challenge",
      LOCALE_COOKIE_NAME,
      DEMO_TOKEN_COOKIE,
      operatorInvitationCookieName(randomUUID()),
      // The backend's trusted device (TrustedDeviceCookieName).
      "mfa_trust_device",
    ];
    const lines = names.map((name) => `${name}=1; Path=/`);
    rawRoutes.set("/app-cookies", (res) => {
      res.setHeader("Set-Cookie", lines);
      res.end();
    });

    expect(names.some(isSessionCookieName)).toBe(false);
    expect(await setCookiesAt("/app-cookies")).toEqual(lines);
  });
});
