import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import NextAuth from "next-auth";
import { NextRequest } from "next/server";
import { withServerSessionMutation } from "~/server/auth/session-mutation";
import { createPortalAuthConfig } from "~/server/auth/portal-config";
import { PORTALS } from "~/server/auth/portals";
const tenantAuthConfig = createPortalAuthConfig(PORTALS.tenant);
const operatorAuthConfig = createPortalAuthConfig(PORTALS.operator);
const parentAuthConfig = createPortalAuthConfig(PORTALS.parent);
const schoolAuthConfig = createPortalAuthConfig(PORTALS.school);
const catererAuthConfig = createPortalAuthConfig(PORTALS.caterer);
const traegerAuthConfig = createPortalAuthConfig(PORTALS.traeger);

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
  ["caterer", catererAuthConfig, "caterer-credentials", "/api/caterer/auth"],
  ["org", traegerAuthConfig, "traeger-credentials", "/api/traeger/auth"],
] as const)(
  "%s public Credentials boundary",
  (portal, config, provider, basePath) => {
    async function submitForm(fields: Record<string, string>) {
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
            "x-moto-server-session-mutation": "true",
          },
          body: new URLSearchParams({ csrfToken: csrf.csrfToken, ...fields }),
        }),
      );
    }

    const submit = (
      token: string,
      refreshToken = "valid-refresh",
      extra: Record<string, string> = {},
    ) => submitForm({ internalRefresh: "true", token, refreshToken, ...extra });

    // Only the parents' login page signs in with a password through Auth.js;
    // every other portal logs in through its BFF login route (review L7), so
    // a password posted to its callback is no second login entry.
    it(
      portal === "parent"
        ? "takes a password sign-in to the parent login"
        : "refuses a password sign-in without asking the backend",
      async () => {
        backend.mockImplementation(async () =>
          Response.json(
            { status: "error", code: "identity.invalid_credentials" },
            { status: 401 },
          ),
        );
        const response = await submitForm({
          email: "local@example.test",
          password: "synthetic-password",
        });
        expect(response.headers.get("location")).toContain(
          "error=CredentialsSignin",
        );
        expect(response.headers.getSetCookie().join(";")).not.toContain(
          "session-token=",
        );
        if (portal === "parent") {
          expect(backend.mock.calls.map(([url]) => String(url))).toEqual([
            "http://backend.test/parent/auth/login",
          ]);
        } else {
          expect(backend).not.toHaveBeenCalled();
        }
      },
    );

    const extraCredentials: Record<string, string>[] = [
      {},
      {
        email: "local@example.test",
        password: "synthetic-password",
        serverSessionMutation: "true",
      },
    ];
    it.each(extraCredentials)(
      "refuses public token seeding even with valid CSRF, valid tokens and extra credentials %j",
      async (extra) => {
        backend.mockImplementation(async () =>
          Response.json({
            id: 42,
            exp: Math.floor(Date.now() / 1000) + 900,
            scope: portal === "tenant" ? "" : portal,
            tenant_id: 7,
          }),
        );
        const response = await submit("valid-access", "valid-refresh", extra);
        expect(response.headers.get("location")).toContain(
          "error=CredentialsSignin",
        );
        expect(response.headers.getSetCookie().join(";")).not.toContain(
          "session-token=",
        );
        expect(backend).not.toHaveBeenCalled();
      },
    );

    it("does not leak an active trusted context into a concurrent public request", async () => {
      let finish: () => void = () => undefined;
      const activeMutation = withServerSessionMutation(
        () =>
          new Promise<void>((resolve) => {
            finish = resolve;
          }),
      );
      try {
        const response = await submit("valid-access");
        expect(response.headers.get("location")).toContain(
          "error=CredentialsSignin",
        );
        expect(response.headers.getSetCookie().join(";")).not.toContain(
          "session-token=",
        );
        expect(backend).not.toHaveBeenCalled();
      } finally {
        finish();
        await activeMutation;
      }
    });

    it.each(["fabricated", "tampered", "expired", "wrong-portal"])(
      "still rejects %s tokens inside the trusted server context",
      async (kind) => {
        backend.mockResolvedValue(new Response(null, { status: 401 }));
        const token = `eyJhbGciOiJIUzI1NiJ9.${Buffer.from(JSON.stringify({ id: 42, exp: 4102444800 })).toString("base64url")}.${kind}`;
        const response = await withServerSessionMutation(() => submit(token));
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
      const response = await withServerSessionMutation(() =>
        submit("valid-access", "arbitrary-refresh"),
      );
      expect(response.headers.getSetCookie().join(";")).not.toContain(
        "session-token=",
      );
      expect(
        JSON.parse(String(backend.mock.calls[0]?.[1]?.body)),
      ).toMatchObject({ refresh_token: "arbitrary-refresh", portal });
    });

    it("keeps backend validation mandatory inside the trusted server context", async () => {
      backend.mockResolvedValue(
        Response.json({
          id: 42,
          exp: Math.floor(Date.now() / 1000) + 900,
          scope: portal === "tenant" ? "" : portal,
          tenant_id: 7,
          sub: "local@example.test",
        }),
      );
      const response = await withServerSessionMutation(() =>
        submit("valid-access"),
      );
      expect(response.headers.getSetCookie().join(";")).toContain(
        "session-token=",
      );
      expect(backend.mock.calls[0]?.[0]).toBe(
        "http://backend.test/auth/session/validate",
      );
    });
  },
);
