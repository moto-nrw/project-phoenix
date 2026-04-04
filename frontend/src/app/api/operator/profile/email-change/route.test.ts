import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils";

const {
  mockFetch,
  mockGetServerApiUrl,
  mockAuth,
  mockGetClientForwardHeaders,
} = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  mockAuth: vi.fn(),
  mockGetClientForwardHeaders: vi.fn(() => ({
    "X-Forwarded-For": "localhost",
    "X-Real-IP": "localhost",
    "User-Agent": "test-agent",
  })),
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
}));

vi.mock("~/lib/client-headers", () => ({
  getClientForwardHeaders: mockGetClientForwardHeaders,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

const mockContext: RouteContext = { params: Promise.resolve({}) };

function createMockRequest(body: unknown): NextRequest {
  return new NextRequest(
    "http://localhost:3000/api/operator/profile/email-change",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  );
}

describe("POST /api/operator/profile/email-change", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = createMockRequest({ email: "new@test.com" });
    const response = await POST(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Unauthorized");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("proxies successful response", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ message: "E-Mail-Änderung eingeleitet" }),
    });

    const request = createMockRequest({
      email: "new@test.com",
      password: "secure-password",
    });
    const response = await POST(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("E-Mail-Änderung eingeleitet");
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/profile/email-change",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer valid-token",
          "Content-Type": "application/json",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("returns 500 on fetch error", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockRejectedValue(new Error("Network error"));

    const request = createMockRequest({ email: "new@test.com" });
    const response = await POST(request, mockContext);

    expect(response.status).toBe(500);
  });
});
