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

import { POST } from "./route";

describe("POST /api/operator/provisioning/devices/[id]/set-api-key", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sets device API key successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: { api_key: "new-key" } }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/devices/123/set-api-key",
      {
        method: "POST",
        body: JSON.stringify({ api_key: "new-key" }),
      },
    );
    const mockContext: RouteContext = {
      params: Promise.resolve({ id: "123" }),
    };
    const response = await POST(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual({ api_key: "new-key" });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/devices/123/set-api-key",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/devices/123/set-api-key",
      {
        method: "POST",
        body: JSON.stringify({ api_key: "new-key" }),
      },
    );
    const mockContext: RouteContext = {
      params: Promise.resolve({ id: "123" }),
    };
    const response = await POST(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.error).toBe("Unauthorized");
  });
});
