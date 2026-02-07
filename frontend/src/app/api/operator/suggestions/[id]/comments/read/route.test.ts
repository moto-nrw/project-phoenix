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

import { POST } from "./route";

describe("POST /api/operator/suggestions/[id]/comments/read", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("marks comments as read successfully", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const response_data = { success: true };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: response_data }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments/read",
      {
        method: "POST",
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success?: boolean;
      error?: string;
    };
    // Check that success is wrapped in ApiResponse
    expect(json.success).toBe(true);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/suggestions/1/comments/read",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({}),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments/read",
      {
        method: "POST",
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments/read",
      {
        method: "POST",
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await POST(request, context);

    expect(response.status).toBe(500);
  });
});
