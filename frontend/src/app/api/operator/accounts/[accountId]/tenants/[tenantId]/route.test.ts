import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/operator", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("operator", {
    operatorAuth: mockAuth,
    uncachedOperatorAuth: vi.fn(),
    withOperatorAuth: (handler: unknown) => handler,
  }),
);

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
    // The per-route invalidJsonMessage option is gone; the operator portal's
    // shared invalid-JSON answer carries its detail text.
    expect(await response.json()).toMatchObject({
      status: 400,
      detail: "Invalid JSON request body",
      code: "general.input",
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
