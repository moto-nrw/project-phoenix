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

describe("GET /api/operator/provisioning/schools", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches schools successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const schools = [
      { id: 1, name: "School A", slug: "school-a", subdomain: "school-a" },
    ];

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: schools }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(schools);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools",
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
  });
});

describe("POST /api/operator/provisioning/schools", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("creates school successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const newSchool = {
      organization_id: 1,
      name: "New School",
      slug: "new-school",
      subdomain: "new-school",
    };
    const createdSchool = { id: 1, ...newSchool, active: true };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 201,
      json: async () => ({ status: "success", data: createdSchool }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(newSchool),
      },
    );
    const response = await POST(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(createdSchool);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools",
      {
        method: "POST",
        body: JSON.stringify({ name: "Test" }),
      },
    );
    const response = await POST(request, mockContext);

    expect(response.status).toBe(401);
  });
});
