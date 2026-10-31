import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import NextAuth, { type NextAuthConfig } from "next-auth";
import Credentials from "next-auth/providers/credentials";
import { encode } from "next-auth/jwt";
import { NextRequest } from "next/server";
import { RequestCookies } from "next/dist/server/web/spec-extension/cookies";
import {
  appendMutableCookies,
  MutableRequestCookiesAdapter,
  type ResponseCookies,
} from "next/dist/server/web/spec-extension/adapters/request-cookies";
import { env } from "~/env";
import { createResponseAwareAuth } from "~/server/auth/route-handler";

// #5399, re-review of #5403: every session cookie leaves through
// gateSessionCookieWrites, also the ones Auth.js' server helpers put into
// Next's cookie jar and the ones of Auth.js' own sign-in callbacks. A family
// Identity & Access answers as revoked gets no cookie on either path.

const state = vi.hoisted(() => ({ jar: null as ResponseCookies | null }));
vi.mock("next/headers", () => ({
  cookies: async () => state.jar,
  headers: async () =>
    new Headers({
      host: "localhost:3000",
      cookie: (state.jar?.getAll() ?? [])
        .map((c) => `${c.name}=${c.value}`)
        .join("; "),
    }),
}));

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

const now = () => Math.floor(Date.now() / 1000);

beforeEach(() => {
  familyStatus = 200;
  backend.mockReset();
  backend.mockImplementation(async (url) => {
    const target = String(url);
    if (target.endsWith("/auth/session/validate"))
      return Response.json({ id: 7 }, { status: familyStatus });
    if (target.endsWith("/parent/auth/login"))
      return Response.json({
        status: "success",
        data: {
          access_token: jwt({
            id: 7,
            sub: "erika@example.test",
            scope: "parent",
            exp: now() + 900,
          }),
          refresh_token: jwt({ id: 7, exp: now() + 86_400 }),
        },
      });
    return new Response(null, { status: 404 });
  });
  vi.stubGlobal("fetch", backend);
  // Auth.js reads AUTH_SECRET before NEXTAUTH_SECRET.
  vi.stubEnv("AUTH_SECRET", env.NEXTAUTH_SECRET);
  vi.stubEnv("AUTH_URL", "http://localhost:3000/api/auth");
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

const NAME = "authjs.session-token";

function sessionCookies(setCookies: string[], name = NAME) {
  return setCookies.filter(
    (cookie) => cookie.split("=")[0]?.replace(/\.\d+$/, "") === name,
  );
}

describe("session replacement through Next's cookie jar (gap 1)", () => {
  const config: NextAuthConfig = {
    secret: env.NEXTAUTH_SECRET,
    trustHost: true,
    session: { strategy: "jwt" },
    cookies: {
      sessionToken: {
        name: NAME,
        options: { httpOnly: true, sameSite: "lax", path: "/" },
      },
    },
    providers: [
      Credentials({
        authorize: (credentials) => ({
          id: "7",
          token: String(credentials.token),
          refreshToken: String(credentials.refreshToken),
        }),
      }),
    ],
    callbacks: {
      jwt: ({ token, user }) => {
        if (user) {
          const issued = user as { token: string; refreshToken: string };
          token.id = "7";
          token.token = issued.token;
          token.refreshToken = issued.refreshToken;
          token.scope = "parent";
        }
        return token;
      },
    },
  };

  async function presented() {
    const value = await encode({
      token: {
        id: "7",
        token: jwt({ id: 7, exp: now() + 600 }),
        refreshToken: "refresh-1",
        scope: "parent",
      },
      secret: env.NEXTAUTH_SECRET,
      salt: NAME,
    });
    return `${NAME}=${value}`;
  }

  /** What Next sends: the route's answer, then its cookie jar appended. */
  function asNextSends(response: Response) {
    const headers = new Headers(response.headers);
    appendMutableCookies(headers, state.jar!);
    return headers.getSetCookie();
  }

  // A preview start or a session replacement that runs while the logout
  // revokes the family: the gate refuses the replacement, and the copy left
  // in Next's jar must not reach the browser either.
  it.each([
    [401, 0],
    [200, 1],
  ])(
    "lets the replacement reach the browser only past the gate (%s)",
    async (status, cookies) => {
      const instance = NextAuth(config);
      const helpers = createResponseAwareAuth(instance.auth, "parent");
      const route = helpers.withAuthResponse(async () => {
        await helpers.mutateSession(
          () =>
            instance.signIn("credentials", {
              redirect: false,
              token: jwt({ id: 7, exp: now() + 900 }),
              refreshToken: "refresh-2",
            }),
          () => true,
        );
        return Response.json({ ok: true });
      });
      const cookie = await presented();
      state.jar = MutableRequestCookiesAdapter.wrap(
        new RequestCookies(new Headers({ cookie })),
      );
      familyStatus = status;

      const response = await route(
        new NextRequest("http://localhost:3000/api/preview", {
          method: "POST",
          headers: { cookie },
        }),
      );

      expect(familyChecks()).toHaveLength(1);
      expect(sessionCookies(asNextSends(response))).toHaveLength(cookies);
    },
  );
});

describe("Auth.js sign-in callback of the parent portal (gap 2)", () => {
  const PARENT = "parent.session-token";
  const base = "http://parents.localhost:3000/api/parent/auth";

  async function signIn() {
    const { GET, POST } = await import("../parent/auth/[...nextauth]/route");
    const csrfResponse = await GET(new NextRequest(`${base}/csrf`));
    const { csrfToken } = (await csrfResponse.json()) as { csrfToken: string };
    const cookie = csrfResponse.headers
      .getSetCookie()
      .map((value) => value.split(";")[0])
      .join("; ");
    return POST(
      new NextRequest(`${base}/callback/parent-credentials`, {
        method: "POST",
        headers: {
          cookie,
          "Content-Type": "application/x-www-form-urlencoded",
        },
        body: new URLSearchParams({
          csrfToken,
          email: "erika@example.test",
          password: "synthetic-local-password",
          callbackUrl: "http://parents.localhost:3000/",
        }),
      }),
    );
  }

  it.each([
    [401, 0],
    [200, 1],
  ])(
    "writes the new session only past the gate (%s)",
    async (status, cookies) => {
      familyStatus = status;

      const response = await signIn();

      expect(
        backend.mock.calls.some(([url]) =>
          String(url).endsWith("/parent/auth/login"),
        ),
      ).toBe(true);
      expect(familyChecks()).toHaveLength(1);
      expect(
        sessionCookies(response.headers.getSetCookie(), PARENT),
      ).toHaveLength(cookies);
    },
  );
});
