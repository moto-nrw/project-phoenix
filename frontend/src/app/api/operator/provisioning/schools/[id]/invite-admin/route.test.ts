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

import { POST } from "./route";

describe("POST /api/operator/provisioning/schools/[id]/invite-admin", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("invites admin for school successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const inviteData = { email: "admin@school.de" };
    const invitation = {
      id: 1,
      email: "admin@school.de",
      delivery_status: "sent",
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 201,
      json: async () => ({ status: "success", data: invitation }),
    });

    const context: RouteContext = {
      params: Promise.resolve({ id: "42" }),
    };
    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/42/invite-admin",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(inviteData),
      },
    );
    const response = await POST(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(invitation);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/42/invite-admin",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(inviteData),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const context: RouteContext = {
      params: Promise.resolve({ id: "42" }),
    };
    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/42/invite-admin",
      {
        method: "POST",
        body: JSON.stringify({ email: "test@test.de" }),
      },
    );
    const response = await POST(request, context);

    expect(response.status).toBe(401);
  });
});
