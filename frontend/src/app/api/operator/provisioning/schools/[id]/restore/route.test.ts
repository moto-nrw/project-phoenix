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

describe("POST /api/operator/provisioning/schools/[id]/restore", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("restores a soft-deleted school successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: null }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/restore",
      { method: "POST" },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10/restore",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/restore",
      { method: "POST" },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/restore",
      { method: "POST" },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: ["10", "20"] as unknown as string }),
    };
    const response = await POST(request, context);

    expect(response.status).toBe(500);
  });

  it("returns 409 when school is not deleted", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 409,
      text: async () => JSON.stringify({ error: "School is not deleted" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/restore",
      { method: "POST" },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(409);
  });

  it("returns 404 when school not found", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => JSON.stringify({ error: "School not found" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/999/restore",
      { method: "POST" },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "999" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(404);
  });
});
