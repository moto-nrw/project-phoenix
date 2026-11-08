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
const context = { params: Promise.resolve({}) };

describe("GET /api/operator/billing/key-date-counts/export", () => {
  it("returns unavailable when the backend cannot be reached", async () => {
    mockAuth.mockResolvedValue({ user: { token: "operator-token" } });
    mockFetch.mockRejectedValue(new TypeError("Failed to fetch"));

    const response = await GET(request, context);

    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toEqual({
      type: "about:blank",
      title: "Service Unavailable",
      status: 503,
      detail: "Service Unavailable",
      code: "general.unavailable",
    });
  });

  it("answers 503 when renewing the session could not be decided", async () => {
    mockFetch.mockClear();
    mockAuth.mockResolvedValue({ user: { token: "operator-token" } });
    mockFetch.mockResolvedValue(new Response(null, { status: 401 }));
    mockUncachedAuth.mockRejectedValue(
      new Error("session service unavailable"),
    );

    const response = await GET(request, context);

    // A renewal that throws (session service down) is undecided: 503, and
    // the session stays as it is.
    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toMatchObject({
      code: "general.unavailable",
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});
