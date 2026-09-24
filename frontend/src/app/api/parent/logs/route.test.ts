import { validateSessionToken } from "~/server/auth/token-validation";
vi.mock("~/server/auth/token-validation", () => ({
  validateSessionToken: vi.fn(),
}));
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

const { mockParentAuth, mockTenantAuth } = vi.hoisted(() => ({
  mockParentAuth: vi.fn<() => Promise<Session | null>>(),
  mockTenantAuth: vi.fn<() => Promise<Session | null>>(),
}));

vi.mock("~/server/auth/parent", () => ({ parentAuth: mockParentAuth }));
vi.mock("~/server/auth", () => ({ auth: mockTenantAuth }));

const { POST } = await import("./route");

function createRequest(body: unknown): NextRequest {
  return new NextRequest("http://eltern.localhost:3000/api/parent/logs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

// Parents-portal errors were invisible: the tenant-only /api/logs rejected
// the parent session with 401, so no client log from a guardian ever arrived.
describe("POST /api/parent/logs", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(validateSessionToken).mockResolvedValue({
      id: "321",
      exp: 4102444800,
      scope: "parent",
    });
    mockParentAuth.mockResolvedValue({
      user: { id: "321", token: "parent-access" },
      expires: "2099-01-01",
    });
    mockTenantAuth.mockResolvedValue(null);
  });

  it("accepts a parent session and verifies it as a parent-scope token", async () => {
    const output = vi.spyOn(console, "log").mockImplementation(() => {});

    const response = await POST(
      createRequest({
        entries: [{ level: "error", msg: "parent_news_poll_answer_failed" }],
      }),
    );

    expect(response.status).toBe(200);
    expect(validateSessionToken).toHaveBeenCalledWith(
      "parent-access",
      "parent",
    );
    expect(mockTenantAuth).not.toHaveBeenCalled();
    expect(JSON.parse(String(output.mock.calls[0]?.[0]))).toMatchObject({
      msg: "parent_news_poll_answer_failed",
      portal: "parent",
      user_id: "321",
      context: "client",
      provenance: "client_log",
      via_api: true,
    });
    output.mockRestore();
  });

  it("rejects a request without a parent session", async () => {
    mockParentAuth.mockResolvedValueOnce(null);
    mockTenantAuth.mockResolvedValue({
      user: { id: "85", token: "tenant-access" },
      expires: "2099-01-01",
    });

    const response = await POST(createRequest({ entries: [] }));

    expect(response.status).toBe(401);
    expect(validateSessionToken).not.toHaveBeenCalled();
  });

  it("rejects a token the backend does not accept as parent scope", async () => {
    vi.mocked(validateSessionToken).mockResolvedValueOnce(null);

    expect((await POST(createRequest({ entries: [] }))).status).toBe(401);
  });

  it("rejects an invalid batch without logging", async () => {
    const output = vi.spyOn(console, "log").mockImplementation(() => {});

    const response = await POST(
      createRequest({ entries: [{ level: "audit", msg: "x" }] }),
    );

    expect(response.status).toBe(400);
    expect(output).not.toHaveBeenCalled();
    output.mockRestore();
  });
});
