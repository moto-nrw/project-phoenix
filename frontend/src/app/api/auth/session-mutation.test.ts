import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import NextAuth, { type NextAuthConfig } from "next-auth";
import Credentials from "next-auth/providers/credentials";
import { NextRequest } from "next/server";
import { RequestCookies } from "next/dist/server/web/spec-extension/cookies";
import {
  appendMutableCookies,
  MutableRequestCookiesAdapter,
  type ResponseCookies,
} from "next/dist/server/web/spec-extension/adapters/request-cookies";
import { createResponseAwareAuth } from "~/server/auth/route-handler";

const state = vi.hoisted(() => ({ jar: null as ResponseCookies | null }));
vi.mock("next/headers", () => ({
  cookies: async () => state.jar,
  headers: async () =>
    new Headers({
      host: "localhost:3000",
      cookie: state
        .jar!.getAll()
        .map((c) => `${c.name}=${c.value}`)
        .join("; "),
    }),
}));

const config: NextAuthConfig = {
  secret: "synthetic-local-session-test-secret",
  trustHost: true,
  session: { strategy: "jwt" },
  cookies: {
    sessionToken: {
      name: "authjs.session-token",
      options: { httpOnly: true, sameSite: "lax", path: "/" },
    },
  },
  providers: [
    Credentials({
      authorize: (credentials) =>
        credentials.value === "refused"
          ? null
          : {
              id: String(credentials.value),
              name: String(credentials.padding ?? ""),
            },
    }),
  ],
  callbacks: {
    jwt: ({ token, user, trigger, session }) => {
      if (user) {
        token.id = user.id;
        token.name = user.name;
      }
      if (trigger === "update") {
        token.name = session.name;
      }
      return token;
    },
    session: ({ session, token }) => {
      session.user.id = String(token.id);
      session.user.name = token.name;
      return session;
    },
  },
};

function cookieHeader(response: Response, initial = "") {
  const browser = new Map(
    initial
      .split("; ")
      .filter(Boolean)
      .map((value) => {
        const i = value.indexOf("=");
        return [value.slice(0, i), value.slice(i + 1)];
      }),
  );
  for (const cookie of response.headers.getSetCookie()) {
    const part = cookie.split(";")[0]!;
    const i = part.indexOf("=");
    if (/Max-Age=0/i.test(cookie)) browser.delete(part.slice(0, i));
    else browser.set(part.slice(0, i), part.slice(i + 1));
  }
  return [...browser].map(([key, value]) => `${key}=${value}`).join("; ");
}
function incoming(cookie = "") {
  state.jar = MutableRequestCookiesAdapter.wrap(
    new RequestCookies(new Headers({ cookie })),
  );
  return new NextRequest("http://localhost:3000/api/auth/switch-tenant", {
    headers: { cookie },
  });
}

beforeEach(() => {
  vi.stubEnv("AUTH_URL", "http://localhost:3000/api/auth");
});
afterEach(() => vi.unstubAllEnvs());

describe("real Auth.js server actions under the response wrapper", () => {
  it.each([
    [0, 12000],
    [12000, 0],
    [12000, 14000],
  ])(
    "keeps only the new session after rotation, padding %s -> %s",
    async (oldPadding, newPadding) => {
      const instance = NextAuth(config);
      const helpers = createResponseAwareAuth(instance.auth, "tenant");
      async function signIn(value: string, padding: number) {
        return helpers.mutateSession(
          () =>
            instance.signIn("credentials", {
              redirect: false,
              value,
              padding: "x".repeat(padding),
            }),
          () => true,
        );
      }
      const seed = helpers.withAuthResponse(async () => {
        await signIn("old", oldPadding);
        return Response.json({ success: true });
      });
      const original = cookieHeader(await seed(incoming()));
      const transition = helpers.withAuthResponse(async () => {
        await signIn("new", newPadding);
        return Response.json({ success: true });
      });
      const response = await transition(incoming(original));
      expect(await response.json()).toEqual({ success: true });
      expect(
        response.headers
          .getSetCookie()
          .every((cookie) => /HttpOnly/i.test(cookie)),
      ).toBe(true);
      appendMutableCookies(response.headers, state.jar!);
      const browser = cookieHeader(response, original);
      const read = helpers.withAuthResponse(async () =>
        Response.json(await helpers.auth()),
      );
      const session = await (await read(incoming(browser))).json();
      expect(session.user.id).toBe("new");
      expect(session.user.name).toHaveLength(newPadding);
      // A subsequent request must not resurrect an obsolete chunk.
      const repeated = await (await read(incoming(browser))).json();
      expect(repeated.user.id).toBe("new");
    },
  );

  it("persists unstable_update after the wrapper rotates the old cookie", async () => {
    const instance = NextAuth(config);
    const helpers = createResponseAwareAuth(instance.auth, "tenant");
    const seed = helpers.withAuthResponse(async () => {
      await helpers.mutateSession(
        () =>
          instance.signIn("credentials", { redirect: false, value: "admin" }),
        () => true,
      );
      return Response.json({ success: true });
    });
    const initial = cookieHeader(await seed(incoming()));
    const update = helpers.withAuthResponse(async () => {
      await helpers.mutateSession(
        () =>
          instance.unstable_update({
            user: { name: "preview" },
            name: "preview",
          } as Parameters<typeof instance.unstable_update>[0]),
        (result) => result?.user.name === "preview",
      );
      return Response.json({ success: true });
    });
    const response = await update(incoming(initial));
    const read = helpers.withAuthResponse(async () =>
      Response.json(await helpers.auth()),
    );
    const session = await (
      await read(incoming(cookieHeader(response, initial)))
    ).json();
    expect(session.user.name).toBe("preview");
  });

  it("does not replace the source cookie when Credentials refuses the new session", async () => {
    const instance = NextAuth(config);
    const helpers = createResponseAwareAuth(instance.auth, "tenant");
    const seed = helpers.withAuthResponse(async () => {
      await helpers.mutateSession(
        () => instance.signIn("credentials", { redirect: false, value: "old" }),
        () => true,
      );
      return Response.json({ success: true });
    });
    const initial = cookieHeader(await seed(incoming()));
    const transition = helpers.withAuthResponse(async () => {
      await expect(
        helpers.mutateSession(
          () =>
            instance.signIn("credentials", {
              redirect: false,
              value: "refused",
            }),
          () => true,
        ),
      ).rejects.toThrow();
      return Response.json({ failed: true }, { status: 500 });
    });
    const response = await transition(incoming(initial));
    const read = helpers.withAuthResponse(async () =>
      Response.json(await helpers.auth()),
    );
    expect(
      (await (await read(incoming(cookieHeader(response, initial)))).json())
        .user.id,
    ).toBe("old");
  });
});
