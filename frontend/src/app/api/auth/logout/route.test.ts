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
  user: { id: "1", token: "test-token", name: "Test User" },
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
    expect(JSON.parse(text)).toEqual({ error: "No active session" });
  });

  it("successfully logs out and returns 204", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(null, { status: 204 }),
    );

    const request = createMockRequest("/api/auth/logout");
    const response = await POST(request);

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/logout",
      {
        method: "POST",
        headers: {
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        },
      },
    );

    expect(response.status).toBe(204);
    const text = await response.text();
    expect(text).toBe("");
  });

  it("returns 204 even when backend returns error", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("Internal Error", { status: 500 }),
    );

    const request = createMockRequest("/api/auth/logout");
    const response = await POST(request);

    expect(response.status).toBe(204);
  });

  it("returns 204 when backend logout succeeds with 200", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ success: true }), { status: 200 }),
    );

    const request = createMockRequest("/api/auth/logout");
    const response = await POST(request);

    expect(response.status).toBe(204);
  });

  it("returns 204 even when fetch throws error", async () => {
    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/auth/logout");
    const response = await POST(request);

    expect(response.status).toBe(204);
  });
});
