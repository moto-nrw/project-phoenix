import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils";

const { mockAuth, mockFetch, mockGetServerApiUrl } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET, PUT } from "./route";

function createMockRequest(method: string, body?: unknown): NextRequest {
  const init: { method: string; body?: string; headers?: HeadersInit } = {
    method,
  };
  if (body) {
    init.body = JSON.stringify(body);
    init.headers = { "Content-Type": "application/json" };
  }
  return new NextRequest("http://localhost:3000/api/operator/profile", init);
}

const mockContext: RouteContext = { params: Promise.resolve({}) };

describe("GET /api/operator/profile", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns profile data when authenticated", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const profileData = {
      id: 1,
      email: "admin@test.com",
      display_name: "Admin User",
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: profileData }),
    });

    const request = createMockRequest("GET");
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: unknown; error?: string };
    expect(json.data).toEqual(profileData);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/profile",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer valid-token",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = createMockRequest("GET");
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string; data?: unknown };
    expect(json.error).toBe("Unauthorized");
  });

  it("handles API error", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => "Server error",
    });

    const request = createMockRequest("GET");
    const response = await GET(request, mockContext);

    expect(response.status).toBe(500);
  });
});

describe("PUT /api/operator/profile", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("updates profile successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const updateBody = { display_name: "Updated Name" };
    const updatedProfile = {
      id: 1,
      email: "admin@test.com",
      display_name: "Updated Name",
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: updatedProfile }),
    });

    const request = createMockRequest("PUT", updateBody);
    const response = await PUT(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: unknown; error?: string };
    expect(json.data).toEqual(updatedProfile);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/profile",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(updateBody),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = createMockRequest("PUT", { display_name: "Test" });
    const response = await PUT(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string; data?: unknown };
    expect(json.error).toBe("Unauthorized");
  });

  it("handles validation error from backend", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      text: async () => JSON.stringify({ error: "Display name too short" }),
    });

    const request = createMockRequest("PUT", { display_name: "" });
    const response = await PUT(request, mockContext);

    expect(response.status).toBe(400);
  });
});
