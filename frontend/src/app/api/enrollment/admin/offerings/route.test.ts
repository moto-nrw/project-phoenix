import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    withTenantAuth: (handler: unknown) => handler,
  }),
);

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { PUT } from "./route";

describe("PUT /api/enrollment/admin/offerings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "access-token" } });
    mockFetch.mockResolvedValue(
      new Response(JSON.stringify({ data: { id: "13" } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
  });

  it("forwards complete-withdrawal confirmation to the backend", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/enrollment/admin/offerings",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          request_id: "12",
          child_id: "13",
          offerings: [],
          reason: "Betreuung endet",
          complete_withdrawal_confirmed: true,
        }),
      },
    );

    // Next always hands a route its context; the portal factory reads it.
    const response = await PUT(request, { params: Promise.resolve({}) });

    expect(response.status).toBe(200);
    const [url, init] = mockFetch.mock.calls[0] ?? [];
    expect(url).toBe(
      "http://localhost:8080/api/enrollment/admin/requests/12/children/13/offerings",
    );
    expect(init).toMatchObject({
      method: "PUT",
      headers: {
        Authorization: "Bearer access-token",
        "Content-Type": "application/json",
      },
    });
    expect(JSON.parse(init?.body as string)).toEqual({
      offerings: [],
      reason: "Betreuung endet",
      complete_withdrawal_confirmed: true,
    });
  });
});
