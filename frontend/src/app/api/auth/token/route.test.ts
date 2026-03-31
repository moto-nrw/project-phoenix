import { describe, it, expect, vi, beforeEach } from "vitest";
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

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url, { method: "POST" });
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/token", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

  it("returns 401 when session has empty token", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "1",
        name: "Test User",
        token: "",
        refreshToken: "refresh-token",
      },
      expires: "2099-01-01",
    });

    const request = createMockRequest("/api/auth/token");
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("No refresh token found");
  });

  it("successfully returns tokens from session", async () => {
    mockAuth.mockResolvedValueOnce({
      user: {
        id: "1",
        token: "current-access-token",
        refreshToken: "current-refresh-token",
        name: "Test User",
      },
      expires: "2099-01-01",
    });

    const request = createMockRequest("/api/auth/token");
    const response = await POST(request);

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{
      access_token: string;
      refresh_token: string;
    }>(response);
    expect(json.access_token).toBe("current-access-token");
    expect(json.refresh_token).toBe("current-refresh-token");
  });

  it("returns 500 on unexpected error", async () => {
    mockAuth.mockRejectedValueOnce(new Error("Unexpected failure"));

    const request = createMockRequest("/api/auth/token");
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });
});
