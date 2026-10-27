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

// The session route of every portal answers the browser (#5005): the access
// token, the refresh token and the recovery proof stay in the encrypted
// session cookie. The routes are imported as deployed, so a portal whose
// route skips withBrowserSession fails here.

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://backend.test",
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "168h",
    NEXTAUTH_URL: "http://localhost:3000",
    NEXTAUTH_SECRET: "local-test-session-secret-not-for-production",
    TENANT_DOMAIN: "localhost",
    NODE_ENV: "test",
  },
}));

const SECRET = "local-test-session-secret-not-for-production";
const ACCESS_TOKEN = "eyJhbGciOiJIUzI1NiJ9.access-token-must-stay-server.sig";
const REFRESH_TOKEN = "refresh-token-must-stay-server";
const RECOVERY_PROOF = "recovery-proof-must-stay-server";

const backend = vi.fn();

beforeAll(() => {
  vi.stubEnv("AUTH_SECRET", SECRET);
});
afterAll(() => {
  vi.unstubAllEnvs();
});
beforeEach(() => {
  vi.stubGlobal("fetch", backend);
  backend.mockReset();
});

type Route = (request: NextRequest) => Promise<Response>;
interface PortalRoute {
  GET: Route;
  POST: Route;
}

const PORTALS: ReadonlyArray<{
  portal: string;
  basePath: string;
  scope: string;
  route: () => Promise<PortalRoute>;
  cookieName: () => Promise<string>;
}> = [
  {
    portal: "tenant",
    basePath: "/api/auth",
    scope: "",
    route: () => import("./[...nextauth]/route") as Promise<PortalRoute>,
    // Auth.js default name: TENANT_DOMAIN localhost sets no cookie option.
    cookieName: async () => "authjs.session-token",
  },
  {
    portal: "operator",
    basePath: "/api/operator/auth",
    scope: "platform",
    route: () =>
      import("../operator/auth/[...nextauth]/route") as Promise<PortalRoute>,
    cookieName: async () =>
      (await import("~/server/auth/operator-config")).operatorAuthConfig.cookies
        .sessionToken.name,
  },
  {
    portal: "parent",
    basePath: "/api/parent/auth",
    scope: "parent",
    route: () =>
      import("../parent/auth/[...nextauth]/route") as Promise<PortalRoute>,
    cookieName: async () =>
      (await import("~/server/auth/parent-config")).parentAuthConfig.cookies
        .sessionToken.name,
  },
  {
    portal: "school",
    basePath: "/api/school/auth",
    scope: "school",
    route: () =>
      import("../school/auth/[...nextauth]/route") as Promise<PortalRoute>,
    cookieName: async () =>
      (await import("~/server/auth/school-config")).schoolAuthConfig.cookies
        .sessionToken.name,
  },
  {
    portal: "caterer",
    basePath: "/api/caterer/auth",
    scope: "caterer",
    route: () =>
      import("../caterer/auth/[...nextauth]/route") as Promise<PortalRoute>,
    cookieName: async () =>
      (await import("~/server/auth/caterer-config")).catererAuthConfig.cookies
        .sessionToken.name,
  },
  {
    portal: "traeger",
    basePath: "/api/traeger/auth",
    scope: "org",
    route: () =>
      import("../traeger/auth/[...nextauth]/route") as Promise<PortalRoute>,
    cookieName: async () =>
      (await import("~/server/auth/traeger-config")).traegerAuthConfig.cookies
        .sessionToken.name,
  },
];

async function sessionCookie(cookieName: string, scope: string) {
  const value = await encode({
    token: {
      id: "7",
      name: "Erika Beispiel",
      email: "erika@example.com",
      token: ACCESS_TOKEN,
      refreshToken: REFRESH_TOKEN,
      refreshRecoveryProof: RECOVERY_PROOF,
      roles: ["admin"],
      permissions: ["admin:*"],
      firstName: "Erika",
      isAdmin: true,
      tenantId: 3,
      orgId: 2,
      scope,
      tokenExpiry: Date.now() + 10 * 60_000,
      refreshTokenExpiry: Date.now() + 24 * 60 * 60_000,
    },
    secret: SECRET,
    salt: cookieName,
  });
  return `${cookieName}=${value}`;
}

function expectNoSecrets(body: string) {
  expect(body).not.toContain(ACCESS_TOKEN);
  expect(body).not.toContain(REFRESH_TOKEN);
  expect(body).not.toContain(RECOVERY_PROOF);
  const session = JSON.parse(body) as { user: Record<string, unknown> };
  expect(session.user).not.toHaveProperty("token");
  expect(session.user).not.toHaveProperty("refreshToken");
  expect(session.user).not.toHaveProperty("refreshRecoveryProof");
}

describe.each(PORTALS)(
  "$portal session route (#5005)",
  ({ basePath, scope, route, cookieName }) => {
    it("answers GET with the session but without any token", async () => {
      const { GET } = await route();
      const cookie = await sessionCookie(await cookieName(), scope);

      const response = await GET(
        new NextRequest(`http://localhost:3000${basePath}/session`, {
          headers: { cookie },
        }),
      );

      expect(response.status).toBe(200);
      // Auth.js marks the session answer itself; the copy keeps it.
      expect(response.headers.get("Cache-Control")).toContain("no-store");
      const body = await response.text();
      expectNoSecrets(body);
      expect(JSON.parse(body)).toMatchObject({
        user: { id: "7", email: "erika@example.com", scope },
      });
      expect(backend).not.toHaveBeenCalled();
    });

    it("answers the update() POST without any token", async () => {
      const { GET, POST } = await route();
      const csrfResponse = await GET(
        new NextRequest(`http://localhost:3000${basePath}/csrf`),
      );
      const { csrfToken } = (await csrfResponse.json()) as {
        csrfToken: string;
      };
      const csrfCookie = csrfResponse.headers
        .getSetCookie()
        .map((value) => value.split(";")[0])
        .join("; ");
      const name = await cookieName();
      const cookie = `${await sessionCookie(name, scope)}; ${csrfCookie}`;

      const response = await POST(
        new NextRequest(`http://localhost:3000${basePath}/session`, {
          method: "POST",
          headers: { cookie, "Content-Type": "application/json" },
          body: JSON.stringify({ csrfToken, data: { name: "Erika B." } }),
        }),
      );

      expect(response.status).toBe(200);
      const body = await response.text();
      expectNoSecrets(body);
      expect(JSON.parse(body)).toMatchObject({
        user: { id: "7", name: "Erika B." },
      });
      // The rotated session cookie still reaches the browser.
      expect(
        response.headers
          .getSetCookie()
          .some((value) => value.startsWith(`${name}=`)),
      ).toBe(true);
    });
  },
);
