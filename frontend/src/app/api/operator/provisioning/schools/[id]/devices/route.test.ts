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

import { GET } from "./route";

describe("GET /api/operator/provisioning/schools/[id]/devices", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches school devices successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const devices = [
      { id: 1, device_id: "dev-001", status: "active", school_id: 10 },
    ];

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: devices }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/devices",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(devices);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10/devices",
      expect.any(Object),
    );
  });

  it("passes the correct school id to the backend endpoint", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: [] }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/77/devices",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "77" }) };
    await GET(request, context);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/77/devices",
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/devices",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(401);
  });

  it("returns error for invalid id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/bad/devices",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 999 as unknown as string }),
    };
    const response = await GET(request, context);

    expect(response.status).toBe(500);
  });
});
