import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { encode } from "next-auth/jwt";
import { NextRequest } from "next/server";
import { env } from "~/env";
import { dropUnchangedSessionCookie } from "~/server/auth/session-cookie-writes";

// #5399: a response that only re-reads the session must not carry its
// cookie. Arriving after a logout or a school switch, it would write the old
// session back; a change (sign-in, rotation, update(), end) keeps it.

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

beforeAll(() => {
  // Auth.js reads AUTH_SECRET before NEXTAUTH_SECRET.
  vi.stubEnv("AUTH_SECRET", env.NEXTAUTH_SECRET);
});
afterAll(() => {
  vi.unstubAllEnvs();
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

describe("dropUnchangedSessionCookie", () => {
  it("drops a re-encoded copy of the presented session, other cookies stay", async () => {
    const before = await sealed(SESSION);
    const after = await sealed(SESSION);
    expect(after).not.toBe(before);

    const answer = await dropUnchangedSessionCookie(
      request(`${NAME}=${before}`),
      response(
        `${NAME}=${after}; Path=/; HttpOnly; SameSite=Lax`,
        "other=1; Path=/",
      ),
    );

    expect(written(answer)).toEqual([]);
    expect(answer.headers.getSetCookie()).toEqual(["other=1; Path=/"]);
  });

  it("keeps a rotated session", async () => {
    const answer = await dropUnchangedSessionCookie(
      request(`${NAME}=${await sealed(SESSION)}`),
      response(
        `${NAME}=${await sealed({ ...SESSION, token: "access-2" })}; Path=/`,
      ),
    );

    expect(written(answer)).toHaveLength(1);
  });

  it("keeps an ended session and a deletion", async () => {
    const presented = request(`${NAME}=${await sealed(SESSION)}`);
    const ended = await dropUnchangedSessionCookie(
      presented,
      response(
        `${NAME}=${await sealed({ ...SESSION, error: "RefreshTokenError" })}; Path=/`,
      ),
    );
    const deleted = await dropUnchangedSessionCookie(
      presented,
      response(`${NAME}=; Path=/; Max-Age=0`),
    );

    expect(written(ended)).toHaveLength(1);
    expect(written(deleted)).toEqual([`${NAME}=; Path=/; Max-Age=0`]);
  });

  it("keeps a session the request did not present (sign-in)", async () => {
    const answer = await dropUnchangedSessionCookie(
      request("other=1"),
      response(`${NAME}=${await sealed(SESSION)}; Path=/`),
    );

    expect(written(answer)).toHaveLength(1);
  });

  it("keeps a cookie it cannot read", async () => {
    const answer = await dropUnchangedSessionCookie(
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

    const answer = await dropUnchangedSessionCookie(
      request(`${NAME}.1=${b1}; ${NAME}.0=${b0}`),
      response(`${NAME}.0=${a0}; Path=/`, `${NAME}.1=${a1}; Path=/`),
    );

    expect(written(answer)).toEqual([]);
  });

  it("keeps a session of another portal's cookie name apart", async () => {
    const tenant = "authjs.session-token";
    const answer = await dropUnchangedSessionCookie(
      request(`${NAME}=${await sealed(SESSION)}`),
      response(`${tenant}=${await sealed(SESSION, tenant)}; Path=/`),
    );

    expect(written(answer, tenant)).toHaveLength(1);
  });
});

describe("BFF route of a portal (#5399)", () => {
  it("answers a route that only read the session without its cookie", async () => {
    const { withParentAuth } = await import("~/server/auth/parent");
    const route = withParentAuth(async (authRequest) =>
      Response.json({ account: authRequest.auth?.user?.id ?? null }),
    );

    const answer = await route(
      new NextRequest("http://parents.localhost:3000/api/parent/children", {
        headers: { cookie: `${NAME}=${await sealed(SESSION)}` },
      }),
    );

    expect(await answer.json()).toEqual({ account: "7" });
    expect(written(answer)).toEqual([]);
  });
});
