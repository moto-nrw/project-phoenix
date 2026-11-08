import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest } from "next/server";
import type { ExtendedSession } from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
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

function createMockRequest(path: string, body?: unknown): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: {
    method: string;
    body?: string;
    headers: Record<string, string>;
  } = {
    method: "POST",
    headers: { "User-Agent": "test-browser" },
  };

  if (body) {
    requestInit.body = JSON.stringify(body);
    requestInit.headers["Content-Type"] = "application/json";
  }

  return new NextRequest(url, requestInit);
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "admin-token", name: "Admin User" },
  expires: "2099-01-01",
};

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/register", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    mockAuth.mockResolvedValue(null);
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("forwards unauthenticated request to backend", async () => {
    const registrationPayload = {
      email: "newuser@example.com",
      password: "Test1234!",
      first_name: "New",
      last_name: "User",
    };

    const backendResponse = {
      status: "success",
      message: "User registered",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "http://server:8080/auth/register",
    );
    expect(requestInit).toMatchObject({
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        // Contract change: the browser's User-Agent travels, no "unknown" default.
        "User-Agent": "test-browser",
      },
      body: JSON.stringify(registrationPayload),
    });

    expect(response.status).toBe(201);
    const json = await parseJsonResponse<typeof backendResponse>(response);
    expect(json.status).toBe("success");
  });

  it("registers user with admin authentication", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const registrationPayload = {
      email: "newuser@example.com",
      password: "Test1234!",
    };

    const backendResponse = {
      status: "success",
      message: "User registered by admin",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "http://server:8080/auth/register",
    );
    expect(requestInit).toMatchObject({
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer admin-token",
        // Contract change: the browser's User-Agent travels, no "unknown" default.
        "User-Agent": "test-browser",
      },
      body: JSON.stringify(registrationPayload),
    });

    expect(response.status).toBe(201);
  });

  it("does not forward Authorization when session exists without a token", async () => {
    mockAuth.mockResolvedValueOnce({
      ...defaultSession,
      user: { ...defaultSession.user, token: undefined },
    });

    const registrationPayload = {
      email: "newuser@example.com",
      password: "Test1234!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ status: "success" }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(requestInit.headers).toMatchObject({
      "Content-Type": "application/json",
    });
    expect(requestInit.headers).not.toHaveProperty("Authorization");
  });

  it("returns 503 on fetch failure", async () => {
    const registrationPayload = {
      email: "test@example.com",
      password: "Test1234!",
    };

    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);

    // Contract change: an unreachable backend answers 503 general.unavailable.
    expect(response.status).toBe(503);
    const json = await parseJsonResponse<{ detail: string; code: string }>(
      response,
    );
    expect(json.code).toBe("general.unavailable");
    expect(JSON.stringify(json)).not.toContain("Network error");
  });

  it("handles backend validation error", async () => {
    const registrationPayload = {
      email: "invalid-email",
      password: "weak",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ detail: "Invalid email format" }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid email format");
  });
});
