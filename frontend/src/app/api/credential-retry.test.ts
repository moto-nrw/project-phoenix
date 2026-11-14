import {
  afterAll,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { NextRequest } from "next/server";

/**
 * A backend 401 on a route that carries the current password or a code must
 * not send it a second time with the renewed session: the backend counts
 * every attempt against the account lock, and the 401 may be its refusal of
 * that credential. Each route opts out of the retry; this runs the real
 * route with a session that renews to a new token and counts the sends.
 */

const SECRET = "Current-Secret-1!";

const state = vi.hoisted(() => ({ issued: 0 }));

vi.mock("next-auth", () => ({
  default: () => ({
    // Every session read renews: the wrapper's read gets access-1, the
    // renewal after a 401 access-2.
    auth:
      (handler: (request: unknown, context: unknown) => unknown) =>
      (request: Request & { auth?: unknown }, context: unknown) => {
        state.issued += 1;
        request.auth = {
          expires: "2099-01-01T00:00:00.000Z",
          user: {
            id: "1",
            token: `access-${state.issued}`,
            refreshToken: "r",
            tenantId: 7,
          },
        };
        return handler(request, context);
      },
    handlers: { GET: vi.fn(), POST: vi.fn() },
    signIn: vi.fn(),
    unstable_update: vi.fn(),
  }),
}));

vi.mock("~/lib/tenant-resolve.server", async (original) => ({
  ...(await original<typeof import("~/lib/tenant-resolve.server")>()),
  resolveTenantCached: async () => ({ tenant_id: 7 }),
}));

const backend = vi.fn<typeof fetch>();
type RoutePost = (request: NextRequest, context: unknown) => Promise<Response>;

const ROUTES: Record<string, () => Promise<{ POST: unknown }>> = {
  "auth/password": () => import("./auth/password/route"),
  "parent/me/news/[announcementId]/declaration": () =>
    import("./parent/me/news/[announcementId]/declaration/route"),
  "operator/profile/password": () =>
    import("./operator/profile/password/route"),
  "operator/profile/email-change": () =>
    import("./operator/profile/email-change/route"),
  "caterer/account/password": () => import("./caterer/account/password/route"),
  "caterer/account/mfa/disable": () =>
    import("./caterer/account/mfa/disable/route"),
  "caterer/account/mfa/confirm": () =>
    import("./caterer/account/mfa/confirm/route"),
  "parent/me/mfa/disable": () => import("./parent/me/mfa/disable/route"),
  "parent/me/mfa/confirm": () => import("./parent/me/mfa/confirm/route"),
  "parent/me/children/[studentId]/pickup-signature-statement": () =>
    import("./parent/me/children/[studentId]/pickup-signature-statement/route"),
};

async function post(route: string, host: string, body: unknown) {
  const { POST } = await ROUTES[route]!();
  const url = `http://${host}/api/${route.replace(/\[[^\]]+\]/g, "1")}`;
  const params = Object.fromEntries(
    [...route.matchAll(/\[([^\]]+)\]/g)].map(([, name]) => [name, "1"]),
  );
  return (POST as RoutePost)(
    new NextRequest(url, {
      method: "POST",
      headers: {
        host,
        origin: `http://${host}`,
        "content-type": "application/json",
      },
      body: JSON.stringify(body),
    }),
    { params: Promise.resolve(params) },
  );
}

const sendsOfSecret = () =>
  backend.mock.calls.filter(([, init]) => String(init?.body).includes(SECRET))
    .length;

beforeAll(() => {
  vi.stubGlobal("fetch", backend);
  vi.spyOn(console, "error").mockImplementation(() => {});
  vi.spyOn(console, "warn").mockImplementation(() => {});
  vi.spyOn(console, "info").mockImplementation(() => {});
});
afterAll(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});
beforeEach(() => {
  vi.stubEnv("TENANT_DOMAIN", "localhost");
  state.issued = 0;
  backend.mockReset();
  backend.mockImplementation(async () =>
    Response.json({ code: "identity.unauthorized" }, { status: 401 }),
  );
});

const password = {
  currentPassword: SECRET,
  newPassword: "New-Secret-2!",
  confirmPassword: "New-Secret-2!",
};

describe("routes that carry the current password or a code", () => {
  it.each([
    ["auth/password", "school-a.localhost:3000", password],
    [
      "parent/me/news/[announcementId]/declaration",
      "parents.localhost:3000",
      { student_id: "1", action: "agree", version_id: "1", password: SECRET },
    ],
    ["operator/profile/password", "operator.localhost:3000", password],
    [
      "operator/profile/email-change",
      "operator.localhost:3000",
      { new_email: "neu@example.test", current_password: SECRET },
    ],
    ["caterer/account/password", "caterer.localhost:3000", password],
    [
      "caterer/account/mfa/disable",
      "caterer.localhost:3000",
      { currentPassword: SECRET },
    ],
    ["caterer/account/mfa/confirm", "caterer.localhost:3000", { code: SECRET }],
    [
      "parent/me/mfa/disable",
      "parents.localhost:3000",
      { currentPassword: SECRET },
    ],
    ["parent/me/mfa/confirm", "parents.localhost:3000", { code: SECRET }],
  ])("%s sends it once after a 401", async (route, host, body) => {
    const response = await post(route, host, body);

    expect(response.status).toBe(401);
    expect(sendsOfSecret()).toBe(1);
  });

  it("a route without a credential repeats the call with the renewed session", async () => {
    const response = await post(
      "parent/me/children/[studentId]/pickup-signature-statement",
      "parents.localhost:3000",
      { note: SECRET },
    );

    // Anker: the session renewed and the call ran twice, so the harness
    // would catch a credential route that retries.
    expect(response.status).toBe(401);
    expect(sendsOfSecret()).toBe(2);
    const bearers = backend.mock.calls.map(([, init]) =>
      new Headers(init?.headers).get("authorization"),
    );
    expect(bearers).toEqual(["Bearer access-1", "Bearer access-2"]);
  });
});
