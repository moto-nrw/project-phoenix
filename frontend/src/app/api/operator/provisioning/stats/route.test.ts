import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

const { mockAuth, mockFetch, mockGetServerApiUrl } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET } from "./route";

const mockContext: RouteContext = { params: Promise.resolve({}) };

describe("GET /api/operator/provisioning/stats", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns operator stats for an authenticated session", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const stats = {
      organization_count: 3,
      school_count: 12,
      account_count: 250,
      device_count: 64,
    };
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: stats }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/stats",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: typeof stats };
    expect(json.data).toEqual(stats);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/stats",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer valid-token",
        }),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/stats",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("propagates a backend 500 as an error response", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => "internal error",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/stats",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBeGreaterThanOrEqual(400);
  });

  it("returns 401 with TOKEN_EXPIRED when the backend returns 401 and refresh fails", async () => {
    mockAuth.mockResolvedValue({ user: { token: "stale-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      text: async () => "expired",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/stats",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as {
      code?: string;
      error?: string;
    };
    expect(json.code).toBe("TOKEN_EXPIRED");
  });
});
