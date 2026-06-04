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

function contextWith(id: string | string[] | undefined): RouteContext {
  return { params: Promise.resolve({ id }) };
}

describe("GET /api/operator/provisioning/organizations/[id]/schools", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards the org id to the backend and returns the schools list", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const schools = [
      { id: 1, name: "School A", organization_id: 42 },
      { id: 2, name: "School B", organization_id: 42 },
    ];
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: schools }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/42/schools",
    );
    const response = await GET(request, contextWith("42"));

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: typeof schools };
    expect(json.data).toEqual(schools);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/organizations/42/schools",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/42/schools",
    );
    const response = await GET(request, contextWith("42"));

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("returns an error response when the id param is missing", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations//schools",
    );
    const response = await GET(request, contextWith(undefined));

    expect(response.status).toBeGreaterThanOrEqual(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("returns an error response when the id param is an array", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/x/schools",
    );
    const response = await GET(request, contextWith(["1", "2"]));

    expect(response.status).toBeGreaterThanOrEqual(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  // Negative paths — these used to be uncovered. The drill-in dashboard
  // depends on 404 propagating verbatim so an org that was deleted in
  // another tab surfaces a "not found" toast rather than a generic 500.
  it("forwards a backend 404 with the same status code", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => JSON.stringify({ error: "organization not found" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/999/schools",
    );
    const response = await GET(request, contextWith("999"));

    expect(response.status).toBe(404);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("organization not found");
  });

  it("forwards a backend 500 with the same status code", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => "internal error",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/42/schools",
    );
    const response = await GET(request, contextWith("42"));

    expect(response.status).toBe(500);
  });

  it("returns 500 when fetch throws a network error", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockRejectedValue(new TypeError("fetch failed"));

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/organizations/42/schools",
    );
    const response = await GET(request, contextWith("42"));

    expect(response.status).toBe(500);
  });
});
