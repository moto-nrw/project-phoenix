import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/server/auth/tenant-route", () => ({
  withTenantAuth: <T>(handler: T) => handler,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn() }),
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

    await PUT(request);

    const [, init] = mockFetch.mock.calls[0] ?? [];
    expect(JSON.parse(init?.body as string)).toEqual({
      offerings: [],
      reason: "Betreuung endet",
      complete_withdrawal_confirmed: true,
    });
  });
});
