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

import { DELETE } from "./route";

describe("DELETE /api/operator/provisioning/persons/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("re-encodes decoded person ids before forwarding", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/persons/10%2Fevil%3Frole%3Dadmin",
      {
        method: "DELETE",
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "10/evil?role=admin" }),
    };

    const response = await DELETE(request, context);

    expect(response.status).toBe(204);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/persons/10%2Fevil%3Frole%3Dadmin",
      expect.objectContaining({ method: "DELETE" }),
    );
  });
});
