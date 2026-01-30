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

const { mockAuth, mockSignIn } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockSignIn: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  signIn: mockSignIn,
}));

vi.mock("~/env", () => ({
  env: {
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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

const defaultSession: ExtendedSession = {
  user: {
    id: "1",
    token: "access-token",
    refreshToken: "refresh-token",
    name: "Test User",
  },
  expires: "2099-01-01",
};

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/token", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    mockAuth.mockResolvedValue(defaultSession);
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("returns 401 when no session exists", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/token");
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("No refresh token found");
  });

  it("returns 401 when no refresh token in session", async () => {
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", name: "Test User" },
      expires: "2099-01-01",
    });

    const request = createMockRequest("/api/auth/token");
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("No refresh token found");
  });

  it("successfully refreshes tokens", async () => {
    const newTokens = {
      access_token: "new-access-token",
      refresh_token: "new-refresh-token",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(newTokens), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    mockSignIn.mockResolvedValueOnce({});

    const request = createMockRequest("/api/auth/token");
    const response = await POST(request);

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/refresh",
      {
        method: "POST",
        headers: {
          Authorization: "Bearer refresh-token",
          "Content-Type": "application/json",
        },
      },
    );

    expect(mockSignIn).toHaveBeenCalledWith("credentials", {
      redirect: false,
      internalRefresh: "true",
      token: "new-access-token",
      refreshToken: "new-refresh-token",
    });

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<typeof newTokens>(response);
    expect(json.access_token).toBe("new-access-token");
    expect(json.refresh_token).toBe("new-refresh-token");
  });

  it("returns error when backend refresh fails", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "Invalid refresh token" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/token");
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Failed to refresh token");
  });

  it("returns 500 when signIn fails after token refresh", async () => {
    const newTokens = {
      access_token: "new-access-token",
      refresh_token: "new-refresh-token",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(newTokens), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    mockSignIn.mockRejectedValueOnce(new Error("SignIn failed"));

    const request = createMockRequest("/api/auth/token");
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Failed to refresh token");
  });

  it("returns 500 on unexpected error", async () => {
    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/auth/token");
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });
});
