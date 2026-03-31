import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

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
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: "POST",
    };

  if (body) {
    requestInit.body = JSON.stringify(body);
    requestInit.headers = { "Content-Type": "application/json" };
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

describe("POST /api/auth/link-to-tenant", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    mockAuth.mockResolvedValue(null);
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/link-to-tenant", {
      email: "test@example.com",
      role_id: 1,
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it("returns 401 when session has no token", async () => {
    mockAuth.mockResolvedValueOnce({
      ...defaultSession,
      user: { ...defaultSession.user, token: undefined },
    });

    const request = createMockRequest("/api/auth/link-to-tenant", {
      email: "test@example.com",
      role_id: 1,
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Nicht authentifiziert");
  });

  it("forwards request with auth token to backend", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const payload = { email: "existing@example.com", role_id: 42 };
    const backendResponse = {
      status: "success",
      data: { id: 100, email: "existing@example.com" },
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/link-to-tenant", payload);
    const response = await POST(request);

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/auth/link-to-tenant",
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<typeof backendResponse>(response);
    expect(json.data.email).toBe("existing@example.com");
  });

  it("forwards 404 from backend for non-existent email", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(
        JSON.stringify({ status: "error", error: "account not found" }),
        {
          status: 404,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    const request = createMockRequest("/api/auth/link-to-tenant", {
      email: "nonexistent@example.com",
      role_id: 1,
    });
    const response = await POST(request);

    expect(response.status).toBe(404);
  });

  it("handles non-JSON response from backend", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("Internal Server Error", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/auth/link-to-tenant", {
      email: "test@example.com",
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });

  it("returns 500 on fetch failure", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/auth/link-to-tenant", {
      email: "test@example.com",
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });
});
