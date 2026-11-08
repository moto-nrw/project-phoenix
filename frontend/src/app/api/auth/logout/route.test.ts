import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string; refreshToken?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockUncachedAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockUncachedAuth,
    withTenantAuth: (handler: unknown) => handler,
  }),
);

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

const { POST } = await import("./route");

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url, { method: "POST" });
}

const defaultSession: ExtendedSession = {
  user: {
    id: "1",
    token: "test-access-token",
    refreshToken: "test-refresh-token",
    name: "Test User",
  },
  expires: "2099-01-01",
};

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/logout", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    mockAuth.mockResolvedValue(defaultSession);
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/logout");
    const response = await POST(request);

    expect(response.status).toBe(401);
    const text = await response.text();
    expect(JSON.parse(text)).toMatchObject({
      status: 401,
      detail: "No active session",
      code: "general.permission",
    });
  });

  it("returns 401 when session has no refresh token", async () => {
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", token: "access-token-only", name: "Test" },
      expires: "2099-01-01",
    });

    const request = createMockRequest("/api/auth/logout");
    const response = await POST(request);

    expect(response.status).toBe(401);
  });

  it("sends refresh token (not access token) to backend", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(null, { status: 204 }),
    );

    const request = new NextRequest("http://localhost:3000/api/auth/logout", {
      method: "POST",
      headers: { "User-Agent": "test-browser" },
    });
    const response = await POST(request);
    const [url, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(url).toBe("http://server:8080/auth/logout");

    // Must send refresh token — backend /auth/logout is guarded by
    // AuthenticateRefreshJWT which rejects access tokens.
    const headers = requestInit.headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer test-refresh-token");

    // Contract change: identity headers only as the request carries them,
    // no Content-Type without a JSON body.
    expect(requestInit).toMatchObject({
      method: "POST",
      cache: "no-store",
      headers: {
        Authorization: "Bearer test-refresh-token",
        "User-Agent": "test-browser",
        "X-Moto-Frontend-Origin": "http://localhost:3000",
      },
    });
    expect(headers).not.toHaveProperty("Content-Type");

    expect(response.status).toBe(204);
    const text = await response.text();
    expect(text).toBe("");
  });

  it("replaces a backend server error with a safe problem envelope", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response('{"code":"logout_failed","details":{"retry":true}}', {
        status: 500,
        headers: { "Content-Type": "application/problem+json" },
      }),
    );

    const request = createMockRequest("/api/auth/logout");
    const response = await POST(request);

    expect(response.status).toBe(500);
    expect(response.headers.get("Content-Type")).toBe(
      "application/problem+json",
    );
    expect(await response.json()).toMatchObject({
      status: 500,
      detail: "Internal Server Error",
      code: "general.server",
    });
  });

  it("forwards a backend 200 response", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ success: true }), { status: 200 }),
    );

    const request = createMockRequest("/api/auth/logout");
    const response = await POST(request);

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ success: true });
  });

  it("returns 503 when the backend cannot be reached", async () => {
    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/auth/logout");
    const response = await POST(request);

    // Contract change: an unreachable backend answers 503 general.unavailable.
    expect(response.status).toBe(503);
    expect(await response.json()).toMatchObject({
      code: "general.unavailable",
    });
  });

  // #5194: a refused refresh token may still belong to a live family that
  // another tab rotated; the browser takes 401 as "nothing left to revoke".
  describe("after the backend refused the refresh token", () => {
    const refused = () =>
      Response.json(
        { status: 401, code: "general.permission" },
        { status: 401 },
      );
    const renewedTo = (refreshToken: string) =>
      mockUncachedAuth.mockResolvedValue({
        ...defaultSession,
        user: { ...defaultSession.user, refreshToken },
      });

    it("revokes with the renewed refresh token", async () => {
      vi.mocked(global.fetch)
        .mockResolvedValueOnce(refused())
        .mockResolvedValueOnce(new Response(null, { status: 204 }));
      renewedTo("test-refresh-renewed");

      const response = await POST(createMockRequest("/api/auth/logout"));

      expect(response.status).toBe(204);
      const [, retry] = vi.mocked(global.fetch).mock.calls[1] as [
        string,
        RequestInit,
      ];
      expect((retry.headers as Record<string, string>).Authorization).toBe(
        "Bearer test-refresh-renewed",
      );
    });

    it("answers 503 while the renewal yields no new token", async () => {
      vi.mocked(global.fetch).mockResolvedValueOnce(refused());
      renewedTo("test-refresh-token");

      const response = await POST(createMockRequest("/api/auth/logout"));

      expect(response.status).toBe(503);
      expect(global.fetch).toHaveBeenCalledTimes(1);
    });

    it("passes the 401 once the renewal ended the session", async () => {
      vi.mocked(global.fetch).mockResolvedValueOnce(refused());
      renewedTo("");

      const response = await POST(createMockRequest("/api/auth/logout"));

      expect(response.status).toBe(401);
    });
  });
});
