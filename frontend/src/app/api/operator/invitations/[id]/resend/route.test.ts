import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const {
  mockFetch,
  mockGetServerApiUrl,
  mockGetClientForwardHeaders,
  mockAuth,
  mockUncachedAuth,
} = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  mockGetClientForwardHeaders: vi.fn(() => ({
    "X-Forwarded-For": "127.0.0.1",
    "User-Agent": "test-agent",
  })),
  mockAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockUncachedAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("~/lib/client-headers", () => ({
  getClientForwardHeaders: mockGetClientForwardHeaders,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

const mockSession = { user: { token: "access-token-123" } };

function createMockRequest(): NextRequest {
  return new NextRequest(
    "http://localhost:3000/api/operator/invitations/42/resend",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
    },
  );
}

function createMockContext() {
  return { params: Promise.resolve({ id: "42" }) };
}

describe("POST /api/operator/invitations/[id]/resend", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(mockSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const response = await POST(createMockRequest(), createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns 401 when token is missing", async () => {
    mockAuth.mockResolvedValue({ user: { token: null } });

    const response = await POST(createMockRequest(), createMockContext());

    expect(response.status).toBe(401);
  });

  it("proxies POST to backend resend endpoint", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ message: "Einladung erneut gesendet" }),
    });

    const response = await POST(createMockRequest(), createMockContext());

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/invitations/42/resend",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer access-token-123",
          "Content-Type": "application/json",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("proxies backend error JSON with original status", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ message: "Einladung nicht gefunden" }),
    });

    const response = await POST(createMockRequest(), createMockContext());

    expect(response.status).toBe(404);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Einladung nicht gefunden");
  });

  it("retries with refreshed token on 401", async () => {
    const refreshedSession = { user: { token: "refreshed-token-456" } };

    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        headers: new Headers({ "content-type": "application/json" }),
        json: async () => ({ error: "Unauthorized" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: new Headers({ "content-type": "application/json" }),
        json: async () => ({ message: "OK" }),
      });

    mockUncachedAuth.mockResolvedValue(refreshedSession);

    const response = await POST(createMockRequest(), createMockContext());

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch).toHaveBeenLastCalledWith(
      "http://localhost:8080/operator/invitations/42/resend",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer refreshed-token-456",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("returns 401 when refresh returns same token", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ error: "Unauthorized" }),
    });

    mockUncachedAuth.mockResolvedValue(mockSession);

    const response = await POST(createMockRequest(), createMockContext());

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string; code?: string };
    expect(json.code).toBe("TOKEN_EXPIRED");
  });

  it("handles non-JSON response", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 502,
      headers: new Headers({ "content-type": "text/html" }),
      text: async () => "Bad Gateway",
    });

    const response = await POST(createMockRequest(), createMockContext());

    expect(response.status).toBe(502);
    const json = (await response.json()) as { message?: string };
    expect(json.message).toBe("Bad Gateway");
  });

  it("returns 500 on fetch error", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const response = await POST(createMockRequest(), createMockContext());

    expect(response.status).toBe(500);
  });
});
