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

describe("GET /api/operator/announcements/[id]/views", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches announcement views successfully", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const views = [
      { account_id: 1, viewed_at: "2024-01-01T00:00:00Z", dismissed_at: null },
      {
        account_id: 2,
        viewed_at: "2024-01-02T00:00:00Z",
        dismissed_at: "2024-01-03T00:00:00Z",
      },
    ];

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: views }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1/views",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual(views);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/announcements/1/views",
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1/views",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1/views",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await GET(request, context);

    expect(response.status).toBe(500);
  });

  it("returns empty array for announcement with no views", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: [] }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1/views",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual([]);
  });
});
