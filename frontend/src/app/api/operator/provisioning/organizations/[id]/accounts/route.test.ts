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

describe("GET /api/operator/provisioning/organizations/[id]/accounts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches organization accounts successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const accounts = [
      {
        account_id: 1,
        email: "admin@org.de",
        role_name: "admin",
        school_id: 10,
        school_name: "GGS Europa",
      },
    ];

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: accounts }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/5/accounts",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "5" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(accounts);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/organizations/5/accounts",
      expect.any(Object),
    );
  });

  it("passes the correct organization id to the backend endpoint", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: [] }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/99/accounts",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "99" }) };
    await GET(request, context);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/organizations/99/accounts",
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/5/accounts",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "5" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(401);
  });

  it("returns error for invalid id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/bad/accounts",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 999 as unknown as string }),
    };
    const response = await GET(request, context);

    expect(response.status).toBe(500);
  });
});
