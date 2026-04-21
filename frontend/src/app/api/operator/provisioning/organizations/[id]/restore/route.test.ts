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

import { POST } from "./route";

describe("POST /api/operator/provisioning/organizations/[id]/restore", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("restores organization successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: null,
        message: "Organization restored successfully",
      }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/77/restore",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "77" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/organizations/77/restore",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/77/restore",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "77" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(401);
  });

  it("returns error for invalid id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/123/restore",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await POST(request, context);

    expect(response.status).toBe(500);
  });

  it("forwards 409 not deleted from backend", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 409,
      text: async () =>
        JSON.stringify({ error: "Organization is not deleted" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/77/restore",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "77" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(409);
  });
});
