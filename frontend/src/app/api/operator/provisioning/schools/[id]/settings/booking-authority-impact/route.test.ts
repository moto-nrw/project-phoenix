import { beforeEach, describe, expect, it, vi } from "vitest";
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

describe("GET booking-authority-impact", () => {
  beforeEach(() => vi.clearAllMocks());

  it("proxies the school-scoped preview", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: {
          reference_date: "2026-08-25",
          blocking_children: [],
          planned_completions: [],
        },
      }),
    });
    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/booking-authority-impact",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };

    const response = await GET(request, context);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10/settings/booking-authority-impact",
      expect.objectContaining({ method: "GET" }),
    );
  });
});
