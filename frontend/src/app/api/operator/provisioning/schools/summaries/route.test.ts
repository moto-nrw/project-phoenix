import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils";

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

describe("GET /api/operator/provisioning/schools/summaries", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns school summaries for an authenticated session", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const summaries = [
      { school_id: 10, account_count: 12, device_count: 4 },
      { school_id: 11, account_count: 6, device_count: 1 },
    ];
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: summaries }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/summaries",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: typeof summaries };
    expect(json.data).toEqual(summaries);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/summaries",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/summaries",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("returns 401 with TOKEN_EXPIRED when the backend returns 401 and refresh fails", async () => {
    mockAuth.mockResolvedValue({ user: { token: "stale-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      text: async () => "expired",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/summaries",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as { code?: string };
    expect(json.code).toBe("TOKEN_EXPIRED");
  });
});
