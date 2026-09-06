import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const sessions = vi.hoisted(() => ({
  tenant: vi.fn(),
  parent: vi.fn(),
  school: vi.fn(),
}));
vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_OPERATOR_HOSTNAME: "operator.localhost:3000",
    NEXT_PUBLIC_PARENTS_HOSTNAME: "parents.localhost:3000",
    NEXT_PUBLIC_SCHOOL_HOSTNAME: "schule.localhost:3000",
  },
}));
vi.mock("~/server/auth", () => ({
  withTenantAuth:
    (handler: (req: unknown) => Promise<Response>) =>
    async (request: NextRequest) => {
      request.clone(); // NextAuth reconstructs the request; its body must remain unread.
      return handler({ auth: sessions.tenant() });
    },
}));
vi.mock("~/server/auth/parent", () => ({
  withParentAuth:
    (handler: (req: unknown) => Promise<Response>) =>
    async (request: NextRequest) => {
      request.clone();
      return handler({ auth: sessions.parent() });
    },
}));
vi.mock("~/server/auth/school", () => ({
  withSchoolAuth:
    (handler: (req: unknown) => Promise<Response>) =>
    async (request: NextRequest) => {
      request.clone();
      return handler({ auth: sessions.school() });
    },
}));

const { POST } = await import("./route");

function request(host: string, origin = `http://${host}`) {
  return new NextRequest(`http://${host}/api/invitations/accept`, {
    method: "POST",
    headers: { host, origin, "Content-Type": "application/json" },
    body: JSON.stringify({
      token: "invitation",
      existingAccount: true,
      account_id: "forged",
      owner_access_token: "forged",
    }),
  });
}

describe("invitation owner session boundary", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessions.tenant.mockReturnValue({ user: { token: "tenant-signed-token" } });
    sessions.parent.mockReturnValue({ user: { token: "parent-signed-token" } });
    sessions.school.mockReturnValue({ user: { token: "school-signed-token" } });
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response("{}", {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );
  });

  it.each([
    ["school-b.localhost:3000", "tenant-signed-token"],
    ["parents.localhost:3000", "parent-signed-token"],
    ["schule.localhost:3000", "school-signed-token"],
  ])("forwards only the matching portal session on %s", async (host, token) => {
    const response = await POST(request(host));
    expect(response.status).toBe(201);
    expect(fetch).toHaveBeenCalledWith(
      expect.stringContaining("/auth/invitations/invitation/accept"),
      expect.objectContaining({
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      }),
    );
    const calls = vi.mocked(fetch).mock.calls;
    expect(calls[0]?.[1]?.body).not.toContain("forged");
    expect(
      sessions.tenant.mock.calls.length +
        sessions.parent.mock.calls.length +
        sessions.school.mock.calls.length,
    ).toBe(1);
  });

  it("does not forward client proof or another portal's cookie when signed out", async () => {
    sessions.parent.mockReturnValue(null);
    const response = await POST(request("parents.localhost:3000"));
    expect(response.status).toBe(401);
    expect(fetch).not.toHaveBeenCalled();
    expect(sessions.tenant).not.toHaveBeenCalled();
  });

  it.each([
    ["operator.localhost:3000", "http://operator.localhost:3000"],
    ["school-b.localhost:3000", "https://attacker.example"],
  ])("rejects operator or cross-origin acceptance", async (host, origin) => {
    expect((await POST(request(host, origin))).status).toBe(403);
    expect(fetch).not.toHaveBeenCalled();
  });
});
