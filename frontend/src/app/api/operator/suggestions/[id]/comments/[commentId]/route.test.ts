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

import { DELETE } from "./route";

describe("DELETE /api/operator/suggestions/[id]/comments/[commentId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("deletes comment successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
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
    mockAuth.mockResolvedValue(null);

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
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

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
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

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
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
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
