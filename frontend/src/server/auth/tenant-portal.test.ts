import {
  afterEach,
  beforeEach,
  describe,
  expect,
  expectTypeOf,
  it,
  vi,
} from "vitest";
import { NextRequest } from "next/server";

/**
 * The tenant portal object is the only way a route reaches the tenant
 * session, so its wrapper must carry the school-host check (#5130) and no
 * unchecked wrapper may sit beside it.
 */

const { session } = vi.hoisted(() => ({
  session: {
    current: null as null | {
      expires: string;
      user: { id: string; token: string; tenantId: number };
    },
  },
}));

vi.mock("next-auth", () => ({
  default: () => ({
    auth:
      (handler: (request: unknown, context: unknown) => unknown) =>
      (request: Request & { auth?: unknown }, context: unknown) => {
        request.auth = session.current;
        return handler(request, context);
      },
    handlers: { GET: vi.fn(), POST: vi.fn() },
    signIn: vi.fn(),
    unstable_update: vi.fn(),
  }),
}));

vi.mock("~/lib/tenant-resolve.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/tenant-resolve.server")>()),
  resolveTenantCached: async (slug: string) => ({
    tenant_id: slug === "schule-a" ? 1 : 2,
  }),
}));

const { tenantPortal } = await import("./index");

const route = tenantPortal.withAuth(async () => Response.json({ ran: true }));

function call(tenantId: number) {
  session.current = {
    expires: "2099-01-01T00:00:00.000Z",
    user: { id: "1", token: "access-token", tenantId },
  };
  return route(
    new NextRequest("http://schule-a.localhost:3000/api/students", {
      headers: { host: "schule-a.localhost:3000" },
    }),
  );
}

beforeEach(() => vi.stubEnv("TENANT_DOMAIN", "localhost"));
afterEach(() => vi.unstubAllEnvs());

describe("tenantPortal", () => {
  it("runs a route for a session of the host's school", async () => {
    const response = await call(1);

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ ran: true });
  });

  it("refuses a session of another school before the route runs", async () => {
    const response = await call(2);

    expect(response.status).toBe(409);
    expect(await response.json()).toMatchObject({
      code: "identity.session_tenant_mismatch",
    });
  });

  it("offers no route wrapper but the checked one", () => {
    expectTypeOf<keyof typeof tenantPortal>().toEqualTypeOf<
      | "portal"
      | "auth"
      | "uncachedAuth"
      | "inRequestContext"
      | "withAuth"
      | "withSessionResponse"
      | "handlers"
      | "setSession"
      | "updateSession"
      | "setStaffPreviewSession"
    >();
    expect(
      Object.keys(tenantPortal).filter((key) => key.startsWith("with")),
    ).toEqual(["withAuth", "withSessionResponse"]);
  });
});
