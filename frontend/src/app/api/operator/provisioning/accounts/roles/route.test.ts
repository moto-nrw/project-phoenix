import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

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

describe("GET /api/operator/provisioning/accounts/roles", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches assignable roles successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const roles = [
      { id: 1, name: "admin", description: "Admin", is_system: true },
      { id: 2, name: "user", description: "User", is_system: true },
    ];

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: roles }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/accounts/roles",
    );
    const response = await GET(request, { params: Promise.resolve({}) });

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: unknown };
    expect(json.data).toEqual(roles);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/accounts/roles",
      expect.any(Object),
    );
  });
});
