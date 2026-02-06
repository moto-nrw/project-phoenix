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

import { PUT } from "./route";

describe("PUT /api/operator/suggestions/[id]/status", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("updates suggestion status successfully", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const statusUpdate = { status: "approved" };
    const updatedSuggestion = {
      id: 1,
      title: "Test",
      status: "approved",
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: updatedSuggestion }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/status",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(statusUpdate),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual(updatedSuggestion);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/suggestions/1/status",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(statusUpdate),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/status",
      {
        method: "PUT",
        body: JSON.stringify({ status: "approved" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/status",
      {
        method: "PUT",
        body: JSON.stringify({ status: "approved" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await PUT(request, context);

    expect(response.status).toBe(500);
  });

  it("handles invalid status value", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      text: async () => JSON.stringify({ error: "Invalid status" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/status",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ status: "invalid" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(400);
  });
});
