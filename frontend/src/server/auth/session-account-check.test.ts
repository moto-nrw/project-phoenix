import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { NextAuthRequest, Session } from "next-auth";
import { NextRequest } from "next/server";
import { SESSION_ACCOUNT_HEADER } from "~/lib/session-account";

// Every portal's route wrapper runs through its real response-aware Auth.js
// handler here, so a wrapper that bypasses the account check fails (#5183).
vi.unmock("~/server/auth/tenant-route");
vi.unmock("~/server/auth/operator-route");
vi.unmock("~/server/auth/parent-route");
vi.unmock("~/server/auth/traeger-route");

const { mockFetch } = vi.hoisted(() => ({ mockFetch: vi.fn() }));

// The cookie holds account 2 of school 7: in another tab, account 2 signed
// in after this tab had loaded its page as account 1.
vi.mock("next-auth", () => {
  const session = {
    user: { id: "2", token: "access-token", tenantId: 7 },
    expires: "2099-01-01T00:00:00.000Z",
  } as unknown as Session;
  type RouteHandler = (
    request: NextAuthRequest,
    context?: unknown,
  ) => Promise<Response>;
  const auth = (handler?: RouteHandler) => {
    if (!handler) return Promise.resolve(session);
    return async (request: NextAuthRequest, context?: unknown) => {
      request.auth = session;
      const response = await handler(request, context);
      return new Response(response.body, response);
    };
  };
  return {
    default: () => ({ auth, handlers: {}, signIn: vi.fn() }),
  };
});

vi.stubGlobal("fetch", mockFetch);

// schule-a is the session's school 7, schule-b school 8 (#5130).
vi.mock("~/lib/tenant-resolve.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/tenant-resolve.server")>()),
  resolveTenantCached: async (slug: string) => ({
    tenant_id: slug === "schule-b" ? 8 : 7,
  }),
}));

type Route = (
  request: NextRequest,
  context: { params: Promise<Record<string, string>> },
) => Promise<Response>;

type Portal = {
  name: string;
  host: string;
  read: () => Promise<Route>;
  write: () => Promise<Route>;
};

const portals: Portal[] = [
  {
    name: "tenant",
    host: "schule-a.localhost:3000",
    read: async () =>
      (await import("~/lib/route-proxy.server")).proxyGet("/api/x"),
    write: async () =>
      (await import("~/lib/route-proxy.server")).proxyPost("/api/x"),
  },
  {
    name: "operator",
    host: "operator.localhost:3000",
    read: async () =>
      (await import("~/lib/operator/route-wrapper.server")).proxyGet("/api/x"),
    write: async () =>
      (await import("~/lib/operator/route-wrapper.server")).proxyPost("/api/x"),
  },
  {
    name: "parent",
    host: "eltern.localhost:3000",
    read: async () =>
      (await import("~/lib/parent/route-wrapper.server")).proxyGet("/api/x"),
    write: async () =>
      (await import("~/lib/parent/route-wrapper.server")).proxyPost("/api/x"),
  },
  {
    name: "school",
    host: "schule.localhost:3000",
    read: async () =>
      (await import("~/lib/school/route-wrapper.server")).proxyGet("/api/x"),
    write: async () =>
      (await import("~/lib/school/route-wrapper.server")).proxyPost("/api/x"),
  },
  {
    name: "caterer",
    host: "caterer.localhost:3000",
    read: async () =>
      (await import("~/lib/caterer/route-wrapper.server")).proxyGet("/api/x"),
    write: async () =>
      (await import("~/lib/caterer/route-wrapper.server")).proxyPost("/api/x"),
  },
  {
    name: "traeger",
    host: "traeger.localhost:3000",
    read: async () =>
      (await import("~/lib/traeger/route-wrapper.server")).proxyGet("/api/x"),
    write: async () =>
      (await import("~/lib/traeger/route-wrapper.server")).proxyPost("/api/x"),
  },
];

function call(
  route: Route,
  {
    host,
    account,
    method = "GET",
    path = "/api/example",
  }: {
    host: string;
    account?: string;
    method?: "GET" | "POST";
    path?: string;
  },
) {
  const headers: Record<string, string> = {
    host,
    "x-moto-original-host": host,
    "content-type": "application/json",
  };
  if (account !== undefined) headers[SESSION_ACCOUNT_HEADER] = account;
  return route(
    new NextRequest(`http://${host}${path}`, {
      method,
      headers,
      ...(method === "POST" ? { body: JSON.stringify({ note: "x" }) } : {}),
    }),
    { params: Promise.resolve({}) },
  );
}

function backendOk(): Response {
  return new Response(JSON.stringify({ status: "success", data: { id: 1 } }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

beforeEach(() => {
  mockFetch.mockReset();
  mockFetch.mockImplementation(async () => backendOk());
  vi.stubEnv("TENANT_DOMAIN", "localhost");
});

afterEach(() => {
  vi.unstubAllEnvs();
});

const cases = portals.map((portal) => [portal.name, portal] as const);

describe("another account signed in in another tab (#5183)", () => {
  it.each(cases)(
    "%s: refuses a read of the old tab before the backend call",
    async (_name, portal) => {
      const response = await call(await portal.read(), {
        host: portal.host,
        account: "1",
      });

      expect(response.status).toBe(409);
      expect(await response.json()).toMatchObject({
        status: 409,
        code: "identity.session_account_mismatch",
      });
      expect(response.headers.get("Cache-Control")).toBe("private, no-store");
      expect(mockFetch).not.toHaveBeenCalled();
    },
  );

  it.each(cases)(
    "%s: refuses a write of the old tab before the backend call",
    async (_name, portal) => {
      const response = await call(await portal.write(), {
        host: portal.host,
        account: "1",
        method: "POST",
      });

      expect(response.status).toBe(409);
      expect(await response.json()).toMatchObject({
        code: "identity.session_account_mismatch",
      });
      expect(mockFetch).not.toHaveBeenCalled();
    },
  );

  it.each(cases)(
    "%s: answers a tab that names the cookie's account, also after it signed in again",
    async (_name, portal) => {
      const read = await call(await portal.read(), {
        host: portal.host,
        account: "2",
      });
      const write = await call(await portal.write(), {
        host: portal.host,
        account: "2",
        method: "POST",
      });

      expect(read.status).toBe(200);
      expect(write.status).toBe(200);
      expect(mockFetch).toHaveBeenCalledTimes(2);
    },
  );

  it.each(cases)(
    "%s: lets a request without the header through (navigation, stream, images)",
    async (_name, portal) => {
      const response = await call(await portal.read(), { host: portal.host });

      expect(response.status).toBe(200);
      expect(mockFetch).toHaveBeenCalledTimes(1);
    },
  );
});

describe("routes that must work with another account in the cookie", () => {
  it.each(["/api/auth/logout", "/api/auth/token", "/api/logs"])(
    "tenant %s stays reachable",
    async (path) => {
      const route = (await import("~/lib/route-proxy.server")).proxyPost(
        "/api/x",
      );

      const response = await call(route, {
        host: "schule-a.localhost:3000",
        account: "1",
        method: "POST",
        path,
      });

      expect(response.status).toBe(200);
      expect(mockFetch).toHaveBeenCalledTimes(1);
    },
  );

  it.each([
    ["operator", "operator.localhost:3000", "/api/operator/auth/logout"],
    ["parent", "eltern.localhost:3000", "/api/parent/auth/logout"],
    ["school", "schule.localhost:3000", "/api/school/auth/logout"],
    ["caterer", "caterer.localhost:3000", "/api/caterer/auth/logout"],
    ["traeger", "traeger.localhost:3000", "/api/traeger/auth/logout"],
  ] as const)("%s logout stays reachable", async (name, host, path) => {
    const portal = portals.find((entry) => entry.name === name)!;

    const response = await call(await portal.write(), {
      host,
      account: "1",
      method: "POST",
      path,
    });

    expect(response.status).toBe(200);
  });
});

it.each(["/api/auth/switch-tenant", "/api/auth/staff-preview/end"])(
  "%s cannot act on a different cookie account",
  async (path) => {
    const route = (await import("~/lib/route-proxy.server")).proxyPost(
      "/api/x",
    );
    const response = await call(route, {
      host: "schule-a.localhost:3000",
      account: "1",
      method: "POST",
      path,
    });
    expect(response.status).toBe(409);
    expect(await response.json()).toMatchObject({
      code: "identity.session_account_mismatch",
    });
    expect(mockFetch).not.toHaveBeenCalled();
    const sameAccount = await call(route, {
      host: "schule-a.localhost:3000",
      account: "2",
      method: "POST",
      path,
    });
    expect(sameAccount.status).toBe(200);
  },
);

it.each([
  ["schule-a.localhost:3000", "~/app/api/sse/events/route"],
  ["eltern.localhost:3000", "~/app/api/parent/sse/events/route"],
  ["schule.localhost:3000", "~/app/api/school/sse/events/route"],
])(
  "%s rejects the stale tab before opening an upstream stream",
  async (host, module) => {
    const { GET } = (await import(module)) as { GET: Route };
    const response = await call(GET, { host, account: "1" });
    expect(response.status).toBe(409);
    expect(await response.json()).toMatchObject({
      code: "identity.session_account_mismatch",
    });
    expect(mockFetch).not.toHaveBeenCalled();
    const sameAccount = await call(GET, { host, account: "2" });
    expect(sameAccount.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    await sameAccount.body?.cancel();
  },
);

describe("school and account checks of the tenant portal", () => {
  it("keeps the school mismatch for the same account on another school", async () => {
    const route = (await import("~/lib/route-proxy.server")).proxyGet("/api/x");

    const response = await call(route, {
      host: "schule-b.localhost:3000",
      account: "2",
    });

    expect(response.status).toBe(409);
    expect(await response.json()).toMatchObject({
      code: "identity.session_tenant_mismatch",
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
