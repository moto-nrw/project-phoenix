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

import { PUT } from "./route";

describe("PUT /api/operator/suggestions/[id]/hidden", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("updates hidden state successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const payload = { hidden: true };
    const updatedSuggestion = { id: 1, is_hidden: true };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: updatedSuggestion }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/hidden",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: unknown };
    expect(json.data).toEqual(updatedSuggestion);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/suggestions/1/hidden",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/hidden",
      {
        method: "PUT",
        body: JSON.stringify({ hidden: true }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/hidden",
      {
        method: "PUT",
        body: JSON.stringify({ hidden: true }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await PUT(request, context);

    expect(response.status).toBe(500);
  });

  it("returns validation error from upstream API", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      text: async () => JSON.stringify({ error: "hidden is required" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/suggestions/1/hidden",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(400);
  });
});
