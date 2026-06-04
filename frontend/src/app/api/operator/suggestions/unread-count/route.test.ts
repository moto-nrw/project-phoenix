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

describe("GET /api/operator/suggestions/unread-count", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches unread comment count successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const countData = { count: 5 };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: countData }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/unread-count",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual(countData);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/suggestions/unread-count",
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/unread-count",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
  });

  it("handles zero unread comments", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const countData = { count: 0 };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: countData }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/unread-count",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: { count?: number };
      error?: string;
    };
    expect(json.data?.count).toBe(0);
  });
});
