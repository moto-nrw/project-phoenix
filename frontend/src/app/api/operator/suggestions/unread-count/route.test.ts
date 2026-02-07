import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils";

const { mockGetOperatorToken, mockFetch, mockGetServerApiUrl } = vi.hoisted(
  () => ({
    mockGetOperatorToken: vi.fn<() => Promise<string | undefined>>(),
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  }),
);

vi.mock("~/lib/operator/cookies", () => ({
  getOperatorToken: mockGetOperatorToken,
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
    mockGetOperatorToken.mockResolvedValue("valid-token");
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
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/unread-count",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
  });

  it("handles zero unread comments", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
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
