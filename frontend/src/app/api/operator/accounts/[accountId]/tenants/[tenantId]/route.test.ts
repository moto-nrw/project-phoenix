import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: vi.fn(),
}));

vi.mock("~/server/auth/operator-route", () => ({
  withOperatorAuth: (handler: unknown) => handler,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://server:8080",
}));

import { PUT } from "./route";

describe("/api/operator/accounts/[accountId]/tenants/[tenantId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "operator-token" } });
    vi.stubGlobal("fetch", mockFetch);
  });

  it("rejects malformed role-change JSON without contacting the backend", async () => {
    const response = await PUT(
      new NextRequest(
        "http://localhost:3000/api/operator/accounts/42/tenants/7",
        {
          method: "PUT",
          body: "{",
          headers: { "Content-Type": "application/json" },
        },
      ),
      { params: Promise.resolve({ accountId: "42", tenantId: "7" }) },
    );

    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({ error: "Invalid JSON body" });
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
