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

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET } from "./route";

describe("GET /api/operator/provisioning/schools/[id]/settings/values/[key]/reveal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("proxies reveal request to backend", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: { value: "1234" } }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/security.ogs_device_pin/reveal",
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: "10",
        key: "security.ogs_device_pin",
      }),
    };
    const response = await GET(request, context);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10/settings/values/security.ogs_device_pin/reveal",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/foo.bar/reveal",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "10", key: "foo.bar" }),
    };
    const response = await GET(request, context);

    expect(response.status).toBe(401);
  });

  it("rejects invalid key pattern", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/BAD/reveal",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "10", key: "BAD" }),
    };
    const response = await GET(request, context);

    expect(response.status).toBe(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("rejects non-string id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/foo.bar/reveal",
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: 10 as unknown as string,
        key: "foo.bar",
      }),
    };
    const response = await GET(request, context);

    expect(response.status).toBe(500);
  });
});
