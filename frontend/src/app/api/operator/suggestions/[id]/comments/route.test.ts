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

describe("POST /api/operator/suggestions/[id]/comments", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("creates comment successfully", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const commentData = { content: "Great suggestion!" };
    const createdComment = {
      id: 1,
      content: "Great suggestion!",
      created_at: "2024-01-01T00:00:00Z",
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 201,
      json: async () => ({ status: "success", data: createdComment }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(commentData),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual(createdComment);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/suggestions/1/comments",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(commentData),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments",
      {
        method: "POST",
        body: JSON.stringify({ content: "Test" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments",
      {
        method: "POST",
        body: JSON.stringify({ content: "Test" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await POST(request, context);

    expect(response.status).toBe(500);
  });

  it("handles empty comment content", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      text: async () => JSON.stringify({ error: "Content is required" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content: "" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(400);
  });
});
