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

describe("POST /api/parent/auth/password-reset", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("proxies to the parent backend endpoint and returns the neutral success body", async () => {
    const requestBody = { email: "parent@example.com" };
    const backendResponse = {
      message: "If the email exists, a password reset link has been sent",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/parent/auth/password-reset",
      requestBody,
    );
    const response = await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(global.fetch).toHaveBeenCalledTimes(1);
    // The route MUST target the parent surface, never the staff one — the
    // backend scopes this to guardian accounts. A copy-paste regression to
    // "/auth/password-reset" would silently send staff reset links from the
    // parents portal.
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "http://server:8080/parent/auth/password-reset",
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
    expect(json.message).toBe(backendResponse.message);
  });

  it("forwards the Retry-After header on a 429 so the modal countdown works", async () => {
    // Cross-layer contract: the backend's 429 + Retry-After drives the
    // localStorage-persisted countdown in the password-reset modal. If this
    // header is dropped here, the rate-limit UX silently breaks.
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

    const request = createMockRequest("/api/parent/auth/password-reset", {
      email: "parent@example.com",
    });
    const response = await POST(request);

    expect(response.status).toBe(429);
    expect(response.headers.get("Retry-After")).toBe("120");

    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Too many requests");
  });
});
