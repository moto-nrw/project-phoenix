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

import { GET, POST } from "./route";

const mockContext: RouteContext = { params: Promise.resolve({}) };

describe("GET /api/operator/provisioning/organizations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches organizations successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const organizations = [
      { id: 1, name: "Org 1", slug: "org-1" },
      { id: 2, name: "Org 2", slug: "org-2" },
    ];

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: organizations }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(organizations);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/organizations",
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations",
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

describe("POST /api/operator/provisioning/organizations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("creates organization successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const newOrg = { name: "New Org", slug: "new-org" };
    const createdOrg = { id: 1, ...newOrg, active: true };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 201,
      json: async () => ({ status: "success", data: createdOrg }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(newOrg),
      },
    );
    const response = await POST(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(createdOrg);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/organizations",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(newOrg),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations",
      {
        method: "POST",
        body: JSON.stringify({ name: "Test" }),
      },
    );
    const response = await POST(request, mockContext);

    expect(response.status).toBe(401);
  });

  it("handles backend validation errors", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 409,
      text: async () => JSON.stringify({ error: "slug already exists" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Dup", slug: "existing" }),
      },
    );
    const response = await POST(request, mockContext);

    expect(response.status).toBe(409);
  });
});
