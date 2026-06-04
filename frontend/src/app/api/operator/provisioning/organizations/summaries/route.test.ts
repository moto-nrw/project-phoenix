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

describe("GET /api/operator/provisioning/organizations/summaries", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns organization summaries for an authenticated session", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const summaries = [
      {
        organization_id: 1,
        school_count: 3,
        account_count: 24,
        device_count: 7,
      },
      {
        organization_id: 2,
        school_count: 1,
        account_count: 5,
        device_count: 2,
      },
    ];
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: summaries }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/summaries",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: typeof summaries };
    expect(json.data).toEqual(summaries);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/organizations/summaries",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("returns 401 when no session token", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/summaries",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("forwards the Authorization header to the backend", async () => {
    mockAuth.mockResolvedValue({ user: { token: "the-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: [] }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/summaries",
    );
    await GET(request, mockContext);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer the-token",
        }),
      }),
    );
  });

  // Negative paths — these used to be uncovered. Without them a regression
  // in handleApiError (e.g. losing the status-code regex) would silently
  // return 500 for every backend non-2xx, which masks 404/409 etc. on the
  // dashboard.
  it("forwards a backend 500 with the same status code", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => JSON.stringify({ error: "summaries failed" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/summaries",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(500);
    const json = (await response.json()) as { error?: string };
    // handleApiError extracts the inner backend error and surfaces it,
    // dropping the "API error (500): " wrapper from the proxy layer.
    expect(json.error).toBe("summaries failed");
  });

  it("forwards a backend 503 with the same status code", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 503,
      text: async () => "service unavailable",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/summaries",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(503);
  });

  it("returns 500 when fetch throws a network error", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockRejectedValue(new TypeError("fetch failed"));

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/summaries",
    );
    const response = await GET(request, mockContext);

    // No HTTP status pattern in the message → handleApiError falls through
    // to a generic 500 instead of leaking the raw stack.
    expect(response.status).toBe(500);
  });
});
