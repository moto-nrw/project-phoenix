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

describe("PUT /api/operator/provisioning/organizations/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("updates organization successfully", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const updateData = {
      name: "Updated Org",
      slug: "updated-org",
      active: true,
    };
    const updatedOrg = { id: 5, ...updateData };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: updatedOrg }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/5",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(updateData),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "5" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(updatedOrg);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/organizations/5",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(updateData),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/5",
      {
        method: "PUT",
        body: JSON.stringify({ name: "Test" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "5" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(401);
  });

  it("returns error for invalid id parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/bad",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Test" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await PUT(request, context);

    expect(response.status).toBe(500);
  });

  it("forwards 409 conflict from backend", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 409,
      text: async () => JSON.stringify({ error: "slug already exists" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/5",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Dup", slug: "taken-slug", active: true }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "5" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(409);
  });
});
