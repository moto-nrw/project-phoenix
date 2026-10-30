import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { RequestCookies } from "next/dist/server/web/spec-extension/cookies";
import {
  MutableRequestCookiesAdapter,
  type ResponseCookies,
} from "next/dist/server/web/spec-extension/adapters/request-cookies";

const state = vi.hoisted(() => ({ jar: null as ResponseCookies | null }));
vi.mock("next/headers", () => ({
  cookies: async () => state.jar,
  headers: async () =>
    new Headers({
      host: "localhost:3000",
      referer: "http://localhost:3000/home?error=previous",
      cookie: "",
    }),
}));
vi.mock("~/server/auth/tenant-host-check", () => ({
  sessionTenantMismatchResponse: async () => null,
}));
vi.mock("~/server/auth/tenant-config", async () => {
  const { default: Credentials } =
    await import("next-auth/providers/credentials");
  return {
    tenantAuthConfig: {
      secret: "synthetic-local-session-setter-test-secret",
      trustHost: true,
      session: { strategy: "jwt" },
      providers: [
        Credentials({
          authorize: (credentials) =>
            credentials.token === "valid-access" &&
            credentials.refreshToken === "valid-refresh"
              ? { id: "42" }
              : null,
        }),
      ],
    },
  };
});
const { withTenantAuth, setTenantSession } = await import("~/server/auth");
beforeEach(() => {
  vi.stubEnv("AUTH_URL", "http://localhost:3000/api/auth");
  state.jar = MutableRequestCookiesAdapter.wrap(
    new RequestCookies(new Headers()),
  );
});
afterEach(() => vi.unstubAllEnvs());

describe("tenant session setter with real Auth.js credentials", () => {
  it("does not mistake an error query in the referrer for a failed sign-in", async () => {
    const route = withTenantAuth(async () => {
      await setTenantSession("valid-access", "valid-refresh");
      return Response.json({ success: true });
    });
    const response = await route(
      new NextRequest("http://localhost:3000/api/auth/switch-tenant"),
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ success: true });
    expect(response.headers.getSetCookie().join(";")).toContain(
      "session-token=",
    );
  });
  it("still rejects invalid credentials rather than accepting an error redirect", async () => {
    const route = withTenantAuth(async () => {
      await expect(
        setTenantSession("invalid-access", "invalid-refresh"),
      ).rejects.toThrow();
      return Response.json({ refused: true });
    });
    const response = await route(
      new NextRequest("http://localhost:3000/api/auth/switch-tenant"),
    );
    expect(await response.json()).toEqual({ refused: true });
    expect(response.headers.getSetCookie().join(";")).not.toContain(
      "session-token=",
    );
  });
});
