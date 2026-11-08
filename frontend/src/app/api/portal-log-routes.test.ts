import { validateSessionToken } from "~/server/auth/token-validation";
vi.mock("~/server/auth/token-validation", () => ({
  validateSessionToken: vi.fn(),
}));
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { ServerSession } from "~/server/auth/session-types";

type Auth = () => Promise<ServerSession | null>;

const { sessions, mockTenantAuth } = vi.hoisted(() => ({
  sessions: {
    operator: vi.fn<Auth>(),
    school: vi.fn<Auth>(),
    traeger: vi.fn<Auth>(),
  },
  mockTenantAuth: vi.fn<Auth>(),
}));

vi.mock("~/server/auth/operator", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("operator", {
    auth: sessions.operator,
  }),
);
vi.mock("~/server/auth/school", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("school", {
    auth: sessions.school,
  }),
);
vi.mock("~/server/auth/traeger", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("traeger", {
    auth: sessions.traeger,
  }),
);
vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockTenantAuth,
  }),
);

// Before, these portals' client logs went to the tenant /api/logs: the proxy
// refused it on the school and Träger hosts, and on the operator host it
// arrived only when a tenant cookie happened to be present.
describe.each([
  {
    portal: "operator",
    host: "operator.localhost:3000",
    validation: "platform",
    route: () => import("./operator/logs/route"),
  },
  {
    portal: "school",
    host: "schule.localhost:3000",
    validation: "school",
    route: () => import("./school/logs/route"),
  },
  {
    portal: "traeger",
    host: "traeger.localhost:3000",
    validation: "org",
    route: () => import("./traeger/logs/route"),
  },
] as const)("POST /api/$portal/logs", ({ portal, host, validation, route }) => {
  const request = (body: unknown) =>
    new NextRequest(`http://${host}/api/${portal}/logs`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

  beforeEach(() => {
    vi.clearAllMocks();
    mockTenantAuth.mockResolvedValue(null);
    sessions[portal].mockResolvedValue({
      user: { id: "55", token: `${portal}-access` },
      expires: "2099-01-01",
    });
    vi.mocked(validateSessionToken).mockResolvedValue({
      id: "55",
      exp: 4102444800,
      scope: validation,
    });
  });

  it("accepts the portal's session without a tenant cookie", async () => {
    const output = vi.spyOn(console, "log").mockImplementation(() => {});
    const { POST } = await route();

    const response = await POST(
      request({ entries: [{ level: "error", msg: "portal_probe_failed" }] }),
    );

    expect(response.status).toBe(200);
    expect(validateSessionToken).toHaveBeenCalledWith(
      `${portal}-access`,
      validation,
    );
    expect(mockTenantAuth).not.toHaveBeenCalled();
    expect(JSON.parse(String(output.mock.calls[0]?.[0]))).toMatchObject({
      msg: "portal_probe_failed",
      portal,
      user_id: "55",
      provenance: "client_log",
    });
    output.mockRestore();
  });

  it("rejects a request without the portal's session", async () => {
    sessions[portal].mockResolvedValueOnce(null);
    mockTenantAuth.mockResolvedValue({
      user: { id: "85", token: "tenant-access" },
      expires: "2099-01-01",
    });
    const { POST } = await route();

    expect((await POST(request({ entries: [] }))).status).toBe(401);
    expect(validateSessionToken).not.toHaveBeenCalled();
  });
});
