import { afterEach, describe, expect, it, vi } from "vitest";
import NextAuth from "next-auth";
import { encode } from "next-auth/jwt";
import { NextRequest } from "next/server";

const { testEnv } = vi.hoisted(() => ({
  testEnv: {
    TENANT_DOMAIN: undefined as string | undefined,
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "1h",
    NEXTAUTH_SECRET: "synthetic-cookie-attributes-test-secret",
  },
}));
vi.mock("~/env", () => ({ env: testEnv }));

// Independent expectations: never derive the protection from PORTALS.
const portals = [
  ["tenant", "lax", "schools-example-test.session-token"],
  ["operator", "lax", "operator.session-token"],
  ["parent", "strict", "parent.session-token"],
  ["school", "strict", "school.session-token"],
  ["caterer", "strict", "caterer.session-token"],
  ["traeger", "strict", "traeger.session-token"],
] as const;

afterEach(() => vi.unstubAllEnvs());

describe.each(["production", "development"] as const)(
  "%s session cookie protection",
  (nodeEnv) => {
    describe.each(["schools.example.test", "localhost", undefined])(
      "TENANT_DOMAIN=%s",
      (domain) => {
        it.each(portals)(
          "%s keeps its cookie attributes",
          async (key, sameSite, name) => {
            vi.stubEnv("NODE_ENV", nodeEnv);
            testEnv.TENANT_DOMAIN = domain;
            vi.resetModules();
            const { PORTALS } = await import("~/server/auth/portals");
            const { createPortalAuthConfig } =
              await import("~/server/auth/portal-config");
            const config = createPortalAuthConfig(PORTALS[key]);
            const secure = nodeEnv === "production";
            const defaults =
              key === "tenant" && domain !== "schools.example.test";
            const cookieName = defaults
              ? `${secure ? "__Secure-" : ""}authjs.session-token`
              : name;
            const cookieDomain =
              key === "tenant" && !defaults
                ? ".schools.example.test"
                : undefined;

            if (defaults) {
              // Auth.js supplies the host-only tenant cookie on localhost or
              // without TENANT_DOMAIN; exercise those defaults on the wire below.
              expect(config.cookies).toEqual({});
            } else {
              expect(config.cookies?.sessionToken).toEqual({
                name: cookieName,
                options: {
                  httpOnly: true,
                  sameSite,
                  secure,
                  path: "/",
                  ...(cookieDomain && { domain: cookieDomain }),
                },
              });
            }

            const origin = `${secure ? "https" : "http"}://localhost:3000`;
            vi.stubEnv("AUTH_URL", `${origin}${config.basePath}`);
            const token = await encode({
              secret: testEnv.NEXTAUTH_SECRET,
              salt: cookieName,
              token: { sub: "7", email: "cookie-test@example.test" },
            });
            // Only the callbacks are isolated: real Auth.js resolves the
            // generated cookie config, reads the session and serializes it.
            const { handlers } = NextAuth({
              ...config,
              callbacks: { jwt: ({ token }) => token },
            });
            const response = await handlers.GET(
              new NextRequest(`${origin}${config.basePath}/session`, {
                headers: { cookie: `${cookieName}=${token}` },
              }),
            );
            expect(response.status).toBe(200);
            expect(await response.json()).toMatchObject({
              user: { email: "cookie-test@example.test" },
            });
            const sessionCookies = response.headers
              .getSetCookie()
              .filter((line) => line.startsWith(`${cookieName}=`));
            expect(sessionCookies).toHaveLength(1);
            const attributes = sessionCookies[0]!.split("; ").slice(1);
            expect(attributes).toContain(
              `SameSite=${sameSite === "strict" ? "Strict" : "Lax"}`,
            );
            expect(attributes).toContain("HttpOnly");
            expect(attributes).toContain("Path=/");
            expect(attributes.includes("Secure")).toBe(secure);
            expect(
              attributes.filter((attribute) => attribute.startsWith("Domain=")),
            ).toEqual(cookieDomain ? [`Domain=${cookieDomain}`] : []);
          },
        );
      },
    );
  },
);
