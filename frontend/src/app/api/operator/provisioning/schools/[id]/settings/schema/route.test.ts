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

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET } from "./route";

describe("GET /api/operator/provisioning/schools/[id]/settings/schema", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("proxies request to the school-scoped backend endpoint", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const schema = { tabs: [] };
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: schema }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/schema",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10/settings/schema",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/schema",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(401);
  });

  it("rejects non-string id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/schema",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 10 as unknown as string }),
    };
    const response = await GET(request, context);

    expect(response.status).toBe(500);
  });
});
