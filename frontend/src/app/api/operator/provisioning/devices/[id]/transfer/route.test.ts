import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
}));
vi.mock("~/server/auth", () => ({ auth: mockAuth }));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));
global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

describe("POST /api/operator/provisioning/devices/[id]/transfer", () => {
  beforeEach(() => vi.clearAllMocks());

  it("forwards an authenticated transfer", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: { id: 99 } }),
    });
    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/devices/17/transfer",
      { method: "POST", body: JSON.stringify({ target_school_id: 23 }) },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "17" }) };

    const response = await POST(request, context);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/devices/17/transfer",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("rejects unauthenticated requests", async () => {
    mockAuth.mockResolvedValue(null);
    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/devices/17/transfer",
      { method: "POST", body: JSON.stringify({ target_school_id: 23 }) },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "17" }) };

    expect((await POST(request, context)).status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
