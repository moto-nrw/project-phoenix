import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest } from "next/server";

// ============================================================================
// Mocks
// ============================================================================

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

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/password-reset", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("successfully requests password reset", async () => {
    const requestBody = { email: "user@example.com" };
    const backendResponse = {
      message: "Password reset email sent",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/password-reset", requestBody);
    const response = await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "http://server:8080/auth/password-reset",
    );
    expect(requestInit).toMatchObject({
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "User-Agent": "unknown",
      },
      body: JSON.stringify(requestBody),
    });

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<typeof backendResponse>(response);
    expect(json.message).toBe("Password reset email sent");
  });

  it("returns 429 with Retry-After header when rate limited", async () => {
    const requestBody = { email: "user@example.com" };

    const backendHeaders = new Headers({
      "Content-Type": "application/json",
      "Retry-After": "120",
    });

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ detail: "Too many requests" }), {
        status: 429,
        headers: backendHeaders,
      }),
    );

    const request = createMockRequest("/api/auth/password-reset", requestBody);
    const response = await POST(request);

    expect(response.status).toBe(429);
    expect(response.headers.get("Retry-After")).toBe("120");

    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Too many requests");
  });
});
