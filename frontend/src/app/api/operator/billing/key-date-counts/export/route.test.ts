import { describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockUncachedAuth,
  withOperatorAuth: (handler: unknown) => handler,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://server:8080",
}));

vi.mock("~/lib/sentry-bff.server", () => ({
  captureBffException: vi.fn(),
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET } from "./route";

const request = new NextRequest(
  "http://localhost:3000/api/operator/billing/key-date-counts/export",
);

describe("GET /api/operator/billing/key-date-counts/export", () => {
  it("returns unavailable when the backend cannot be reached", async () => {
    mockAuth.mockResolvedValue({ user: { token: "operator-token" } });
    mockFetch.mockRejectedValue(new TypeError("Failed to fetch"));

    const response = await GET(request);

    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toEqual({
      status: "error",
      error: "Export proxy failed",
      code: "general.unavailable",
    });
  });

  it("returns unavailable when refreshing the expired session fails", async () => {
    mockAuth.mockResolvedValue({ user: { token: "operator-token" } });
    mockFetch.mockResolvedValue(new Response(null, { status: 401 }));
    mockUncachedAuth.mockRejectedValue(
      new Error("session service unavailable"),
    );

    const response = await GET(request);

    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toEqual({
      status: "error",
      error: "Export proxy failed",
      code: "general.unavailable",
    });
  });
});
