import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import NextAuth from "next-auth";
import { NextRequest } from "next/server";
import { tenantAuthConfig } from "~/server/auth/tenant-config";
import { operatorAuthConfig } from "~/server/auth/operator-config";
import { parentAuthConfig } from "~/server/auth/parent-config";
import { schoolAuthConfig } from "~/server/auth/school-config";

vi.mock("~/env", () => ({
  env: {
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "168h",
    NEXTAUTH_SECRET: "local-test-session-secret-not-for-production",
    TENANT_DOMAIN: "localhost",
  },
}));

const backend = vi.fn();
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});
beforeEach(() => {
  vi.stubEnv("API_URL", "http://backend.test");
  vi.stubGlobal("fetch", backend);
  backend.mockReset();
});

describe.each([
  ["tenant", tenantAuthConfig, "credentials", "/api/auth"],
  [
    "platform",
    operatorAuthConfig,
    "operator-credentials",
    "/api/operator/auth",
  ],
  ["parent", parentAuthConfig, "parent-credentials", "/api/parent/auth"],
  ["school", schoolAuthConfig, "school-credentials", "/api/school/auth"],
] as const)(
  "%s public Credentials boundary",
  (portal, config, provider, basePath) => {
    async function submit(token: string, refreshToken = "valid-refresh") {
      const { handlers } = NextAuth({
        ...config,
        basePath,
        secret: "local-test-session-secret-not-for-production",
        trustHost: true,
      });
      const csrfResponse = await handlers.GET(
        new NextRequest(`http://localhost${basePath}/csrf`),
      );
      const csrf = (await csrfResponse.json()) as { csrfToken: string };
      const cookies = csrfResponse.headers
        .getSetCookie()
        .map((cookie) => cookie.split(";")[0])
        .join("; ");
      return handlers.POST(
        new NextRequest(`http://localhost${basePath}/callback/${provider}`, {
          method: "POST",
          headers: {
            "Content-Type": "application/x-www-form-urlencoded",
            cookie: cookies,
          },
          body: new URLSearchParams({
            csrfToken: csrf.csrfToken,
            internalRefresh: "true",
            token,
            refreshToken,
          }),
        }),
      );
    }

    it.each(["fabricated", "tampered", "expired", "wrong-portal"])(
      "rejects %s tokens without a session cookie",
      async (kind) => {
        backend.mockResolvedValue(new Response(null, { status: 401 }));
        const token = `eyJhbGciOiJIUzI1NiJ9.${Buffer.from(JSON.stringify({ id: 42, exp: 4102444800 })).toString("base64url")}.${kind}`;
        const response = await submit(token);
        expect(backend).toHaveBeenCalledOnce();
        expect(response.headers.getSetCookie().join(";")).not.toContain(
          "session-token=",
        );
        expect(response.headers.get("location")).toContain(
          "error=CredentialsSignin",
        );
      },
    );

    it("rejects an arbitrary refresh token", async () => {
      backend.mockResolvedValue(new Response(null, { status: 401 }));
      const response = await submit("valid-access", "arbitrary-refresh");
      expect(response.headers.getSetCookie().join(";")).not.toContain(
        "session-token=",
      );
      expect(
        JSON.parse(String(backend.mock.calls[0]?.[1]?.body)),
      ).toMatchObject({ refresh_token: "arbitrary-refresh", portal });
    });

    it("creates a session only from backend-verified claims", async () => {
      backend.mockResolvedValue(
        Response.json({
          id: 42,
          exp: Math.floor(Date.now() / 1000) + 900,
          scope: portal === "tenant" ? "" : portal,
          tenant_id: 7,
          sub: "local@example.test",
        }),
      );
      const response = await submit("valid-access");
      expect(response.headers.getSetCookie().join(";")).toContain(
        "session-token=",
      );
      expect(backend.mock.calls[0]?.[0]).toBe(
        "http://backend.test/auth/session/validate",
      );
    });
  },
);
