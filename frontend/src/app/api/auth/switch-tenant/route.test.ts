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

function createMockRequest(
  path: string,
  options: { body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const init: { method: string; body?: string; headers?: HeadersInit } = {
    method: "POST",
  };

  if (options.body) {
    init.body = JSON.stringify(options.body);
    init.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, init);
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/switch-tenant", () => {
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

    const request = createMockRequest("/api/auth/switch-tenant", {
      body: { tenant_slug: "school-b" },
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Unauthorized");
  });

  it("proxies switch request and returns token pair", async () => {
    const tokens = {
      access_token: "new-jwt",
      refresh_token: "new-refresh",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(tokens), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/switch-tenant", {
      body: { tenant_slug: "school-b" },
    });
    const response = await POST(request);

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/switch-tenant",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer test-token",
        },
        body: JSON.stringify({ tenant_slug: "school-b" }),
      },
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<typeof tokens>(response);
    expect(json.access_token).toBe("new-jwt");
  });

  it("forwards backend error status", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "Tenant not found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/switch-tenant", {
      body: { tenant_slug: "nonexistent" },
    });
    const response = await POST(request);

    expect(response.status).toBe(404);
  });

  it("returns 500 on fetch failure", async () => {
    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/auth/switch-tenant", {
      body: { tenant_slug: "school-b" },
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });
});
