import {
  afterAll,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { encode } from "next-auth/jwt";
import { NextRequest } from "next/server";
import { env } from "~/env";
import { gateSessionCookieWrites } from "~/server/auth/session-cookie-writes";

// #5399, server invariant: a response writes a session cookie only for a
// changed session whose refresh-token family Identity & Access confirms live
// while the response is built. Whenever its request started and however late
// its answer arrives, an unchanged, an ended or a revoked session sets none.

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

const backend = vi.fn<typeof fetch>();
/** The answer of Identity & Access on the session's family. */
let familyStatus = 200;

function familyChecks() {
  return backend.mock.calls.filter(([url]) =>
    String(url).endsWith("/auth/session/validate"),
  );
}

/** A JWT-shaped token; the BFF reads its claims without a signature. */
function jwt(claims: Record<string, unknown>) {
  const part = (value: unknown) =>
    Buffer.from(JSON.stringify(value)).toString("base64url");
  return `${part({ alg: "HS256" })}.${part(claims)}.signature`;
}

beforeAll(() => {
  // Auth.js reads AUTH_SECRET before NEXTAUTH_SECRET.
  vi.stubEnv("AUTH_SECRET", env.NEXTAUTH_SECRET);
});
afterAll(() => {
  vi.unstubAllEnvs();
});
beforeEach(() => {
  familyStatus = 200;
  backend.mockReset();
  backend.mockImplementation(async (url) => {
    const target = String(url);
    if (target.endsWith("/auth/session/validate"))
      return Response.json({ id: 7 }, { status: familyStatus });
    if (target.endsWith("/auth/refresh")) {
      const now = Math.floor(Date.now() / 1000);
      return Response.json({
        access_token: jwt({ id: 7, scope: "parent", exp: now + 900 }),
        refresh_token: jwt({ id: 7, exp: now + 86_400 }),
      });
    }
    return new Response(null, { status: 404 });
  });
  vi.stubGlobal("fetch", backend);
});

function sealed(token: Record<string, unknown>, salt = NAME) {
  return encode({ token, secret: env.NEXTAUTH_SECRET, salt });
}

function request(cookie: string) {
  return new Request("http://parents.localhost:3000/api/parent/children", {
    headers: { cookie },
  });
}

function response(...setCookies: string[]) {
  const answer = new Response(null);
  for (const cookie of setCookies) answer.headers.append("Set-Cookie", cookie);
  return answer;
}

function written(answer: Response, name = NAME) {
  return answer.headers
    .getSetCookie()
    .filter((cookie) => cookie.split("=")[0]?.replace(/\.\d+$/, "") === name);
}

describe("gateSessionCookieWrites", () => {
  it("drops a re-encoded copy of the presented session, other cookies stay", async () => {
    const before = await sealed(SESSION);
    const after = await sealed(SESSION);
    expect(after).not.toBe(before);

    const answer = await gateSessionCookieWrites(
      request(`${NAME}=${before}`),
      response(
        `${NAME}=${after}; Path=/; HttpOnly; SameSite=Lax`,
        "other=1; Path=/",
      ),
    );

    expect(written(answer)).toEqual([]);
    expect(answer.headers.getSetCookie()).toEqual(["other=1; Path=/"]);
    expect(familyChecks()).toEqual([]);
  });

  it("keeps a rotated session of a live family", async () => {
    const answer = await gateSessionCookieWrites(
      request(`${NAME}=${await sealed(SESSION)}`),
      response(
        `${NAME}=${await sealed({ ...SESSION, token: "access-2" })}; Path=/`,
      ),
    );

    expect(written(answer)).toHaveLength(1);
    expect(familyChecks()).toHaveLength(1);
  });

  it.each([
    [401, "revoked"],
    [503, "not readable"],
  ])("drops a rotated session whose family answers %s (%s)", async (status) => {
    familyStatus = status;

    const answer = await gateSessionCookieWrites(
      request(`${NAME}=${await sealed(SESSION)}`),
      response(
        `${NAME}=${await sealed({ ...SESSION, token: "access-2" })}; Path=/`,
        "other=1; Path=/",
      ),
    );

    expect(written(answer)).toEqual([]);
    expect(answer.headers.getSetCookie()).toEqual(["other=1; Path=/"]);
  });

  it("drops an ended session without asking, keeps a deletion", async () => {
    const presented = request(`${NAME}=${await sealed(SESSION)}`);
    const ended = await gateSessionCookieWrites(
      presented,
      response(
        `${NAME}=${await sealed({ ...SESSION, error: "RefreshTokenError" })}; Path=/`,
      ),
    );
    const deleted = await gateSessionCookieWrites(
      presented,
      response(`${NAME}=; Path=/; Max-Age=0`),
    );

    expect(written(ended)).toEqual([]);
    expect(written(deleted)).toEqual([`${NAME}=; Path=/; Max-Age=0`]);
    expect(familyChecks()).toEqual([]);
  });

  it("keeps a sign-in of a live family the request did not present", async () => {
    const answer = await gateSessionCookieWrites(
      request("other=1"),
      response(`${NAME}=${await sealed(SESSION)}; Path=/`),
    );

    expect(written(answer)).toHaveLength(1);
    expect(familyChecks()).toHaveLength(1);
  });

  it("keeps a cookie it cannot read: it carries no session", async () => {
    const answer = await gateSessionCookieWrites(
      request(`${NAME}=not-a-session`),
      response(`${NAME}=also-not-a-session; Path=/`),
    );

    expect(written(answer)).toHaveLength(1);
  });

  it("compares chunked sessions as a whole", async () => {
    const before = await sealed(SESSION);
    const after = await sealed(SESSION);
    const cut = (value: string) => [value.slice(0, 40), value.slice(40)];
    const [b0, b1] = cut(before);
    const [a0, a1] = cut(after);

    const answer = await gateSessionCookieWrites(
      request(`${NAME}.1=${b1}; ${NAME}.0=${b0}`),
      response(`${NAME}.0=${a0}; Path=/`, `${NAME}.1=${a1}; Path=/`),
    );

    expect(written(answer)).toEqual([]);
  });

  it.each(["shrink", "unchunk", "chunk"])(
    "validates replacement writes alongside %s cleanup",
    async (shape) => {
      const before = await sealed(SESSION);
      for (const status of [200, 401, 503]) {
        backend.mockClear();
        familyStatus = status;
        const after = await sealed({ ...SESSION, token: "access-2" });
        const deletions =
          shape === "chunk"
            ? [`${NAME}=; Path=/; Max-Age=0`]
            : [
                `${NAME}.0=; Path=/; Max-Age=0`,
                `${NAME}.1=; Path=/; Max-Age=0`,
              ];
        const replacements =
          shape === "unchunk"
            ? [`${NAME}=${after}; Path=/`]
            : shape === "shrink"
              ? [`${NAME}.0=${after}; Path=/`]
              : [
                  `${NAME}.0=${after.slice(0, 40)}; Path=/`,
                  `${NAME}.1=${after.slice(40)}; Path=/`,
                ];
        const answer = await gateSessionCookieWrites(
          request(
            shape === "chunk"
              ? `${NAME}=${before}`
              : `${NAME}.0=${before.slice(0, 40)}; ${NAME}.1=${before.slice(40)}`,
          ),
          response(...deletions, ...replacements, "other=1; Path=/"),
        );

        expect(familyChecks()).toHaveLength(1);
        expect(answer.headers.getSetCookie()).toEqual([
          ...deletions,
          ...(status === 200 ? replacements : []),
          "other=1; Path=/",
        ]);
      }
    },
  );

  it("drops an unchanged replacement but keeps obsolete chunk deletions", async () => {
    const before = await sealed(SESSION);
    const deletion = `${NAME}.1=; Path=/; Max-Age=0`;
    const answer = await gateSessionCookieWrites(
      request(
        `${NAME}.0=${before.slice(0, 40)}; ${NAME}.1=${before.slice(40)}`,
      ),
      response(deletion, `${NAME}.0=${await sealed(SESSION)}; Path=/`),
    );

    expect(answer.headers.getSetCookie()).toEqual([deletion]);
    expect(familyChecks()).toEqual([]);
  });

  it("asks once per access token", async () => {
    const answer = await gateSessionCookieWrites(
      request(`${NAME}=${await sealed(SESSION)}`),
      response(
        `${NAME}=${await sealed({ ...SESSION, name: "Erika B." })}; Path=/`,
        `${NAME}=${await sealed({ ...SESSION, name: "Erika C." })}; Path=/`,
      ),
    );

    expect(written(answer)).toHaveLength(2);
    expect(familyChecks()).toHaveLength(1);
  });
});

describe("BFF route of a portal (#5399)", () => {
  async function route() {
    const { withParentAuth } = await import("~/server/auth/parent");
    return withParentAuth(async (authRequest) =>
      Response.json({ account: authRequest.auth?.user?.id ?? null }),
    );
  }

  it("answers a route that only read the session without its cookie", async () => {
    const answer = await (
      await route()
    )(
      new NextRequest("http://parents.localhost:3000/api/parent/children", {
        headers: { cookie: `${NAME}=${await sealed(SESSION)}` },
      }),
    );

    expect(await answer.json()).toEqual({ account: "7" });
    expect(written(answer)).toEqual([]);
  });

  // The request renewed its session before the logout revoked the family;
  // its answer is built after it. No cookie of the revoked family leaves.
  it.each([
    [200, 1],
    [401, 0],
  ])(
    "writes a renewed session only while its family is live (%s)",
    async (status, cookies) => {
      familyStatus = status;
      const due = {
        ...SESSION,
        refreshToken: `refresh-due-${status}`,
        tokenExpiry: Date.now() + 1_000,
      };

      const answer = await (
        await route()
      )(
        new NextRequest("http://parents.localhost:3000/api/parent/children", {
          headers: { cookie: `${NAME}=${await sealed(due)}` },
        }),
      );

      expect(
        backend.mock.calls.some(([url]) =>
          String(url).endsWith("/auth/refresh"),
        ),
      ).toBe(true);
      expect(written(answer)).toHaveLength(cookies);
    },
  );
});
