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

import { DELETE } from "./route";

describe("DELETE /api/operator/suggestions/[id]/comments/[commentId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("deletes comment successfully", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments/42",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "1", commentId: "42" }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(204);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/suggestions/1/comments/42",
      expect.objectContaining({
        method: "DELETE",
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments/42",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "1", commentId: "42" }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments/42",
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: 123 as unknown as string,
        commentId: "42",
      }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(500);
  });

  it("handles invalid commentId parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments/42",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "1", commentId: 42 as unknown as string }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(500);
  });

  it("returns 404 for non-existent comment", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => JSON.stringify({ error: "Not found" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/comments/999",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "1", commentId: "999" }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(404);
  });
});
