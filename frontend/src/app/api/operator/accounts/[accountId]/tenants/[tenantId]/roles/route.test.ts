import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockUncachedAuth,
}));

vi.mock("~/server/auth/operator-route", () => ({
  withOperatorAuth: (handler: unknown) => handler,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://server:8080",
}));

import { GET } from "./route";

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("/api/operator/accounts/[accountId]/tenants/[tenantId]/roles", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "operator-token" } });
    vi.stubGlobal("fetch", mockFetch);
  });

  it("forwards the assignable-role request with the operator token", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ data: [] }));

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/operator/accounts/42/tenants/7/roles",
      ),
      { params: Promise.resolve({ accountId: "42", tenantId: "7" }) },
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/operator/accounts/42/tenants/7/roles",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer operator-token",
        }),
      }),
    );
  });

  it("rejects missing route parameters without contacting the backend", async () => {
    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/operator/accounts/42/tenants/7/roles",
      ),
      { params: Promise.resolve({ accountId: "42" }) },
    );

    expect(response.status).toBe(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
