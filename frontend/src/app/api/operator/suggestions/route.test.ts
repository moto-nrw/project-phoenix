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

describe("GET /api/operator/suggestions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches suggestions without query parameters", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const suggestions = [
      { id: 1, title: "Suggestion 1", status: "pending" },
      { id: 2, title: "Suggestion 2", status: "approved" },
    ];

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: suggestions }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual(suggestions);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/suggestions",
      expect.any(Object),
    );
  });

  it("forwards status query parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: [] }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions?status=pending",
    );
    await GET(request, mockContext);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/suggestions?status=pending",
      expect.any(Object),
    );
  });

  it("forwards search query parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: [] }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions?search=test",
    );
    await GET(request, mockContext);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/suggestions?search=test",
      expect.any(Object),
    );
  });

  it("forwards sort query parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: [] }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions?sort=created_at",
    );
    await GET(request, mockContext);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/suggestions?sort=created_at",
      expect.any(Object),
    );
  });

  it("forwards multiple query parameters", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: [] }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions?status=pending&search=bug&sort=priority",
    );
    await GET(request, mockContext);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("status=pending"),
      expect.any(Object),
    );
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("search=bug"),
      expect.any(Object),
    );
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("sort=priority"),
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
  });
});
