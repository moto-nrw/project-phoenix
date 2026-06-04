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

describe("GET /api/operator/provisioning/schools/[id]/persons", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("re-encodes decoded school ids before forwarding", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const persons = [{ id: "person-1", first_name: "Ada" }];

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: persons }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10%2Fevil/persons",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "10/evil" }),
    };

    const response = await GET(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: unknown };
    expect(json.data).toEqual(persons);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10%2Fevil/persons",
      expect.objectContaining({ method: "GET" }),
    );
  });
});
