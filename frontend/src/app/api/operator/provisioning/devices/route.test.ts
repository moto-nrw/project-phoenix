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

import { GET, POST } from "./route";

const mockContext: RouteContext = { params: Promise.resolve({}) };

describe("GET /api/operator/provisioning/devices", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches all devices successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const devices = [
      { id: 1, device_id: "dev-001", status: "active" },
      { id: 2, device_id: "dev-002", status: "inactive" },
    ];

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: devices }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/devices",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(devices);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/devices",
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/devices",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.error).toBe("Unauthorized");
  });
});

describe("POST /api/operator/provisioning/devices", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("creates a device successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const device = { id: 1, device_id: "dev-003", status: "active" };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: device }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/devices",
      {
        method: "POST",
        body: JSON.stringify({
          device_id: "dev-003",
          device_type: "rfid_reader",
          school_id: 10,
        }),
      },
    );
    const response = await POST(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(device);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/devices",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/devices",
      {
        method: "POST",
        body: JSON.stringify({ device_id: "dev-003" }),
      },
    );
    const response = await POST(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.error).toBe("Unauthorized");
  });
});
