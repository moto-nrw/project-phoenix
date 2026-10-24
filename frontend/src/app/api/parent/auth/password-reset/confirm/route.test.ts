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

describe("POST /api/parent/auth/password-reset/confirm", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("proxies to the parent confirm endpoint and returns the success body", async () => {
    const requestBody = {
      token: "reset-token-123",
      new_password: "NewPassword123!",
      confirm_password: "NewPassword123!",
    };
    const backendResponse = { message: "Password reset successfully" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/parent/auth/password-reset/confirm",
      requestBody,
    );
    const response = await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "http://server:8080/parent/auth/password-reset/confirm",
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
    expect(json.message).toBe("Password reset successfully");
  });

  it("propagates a 400 for an invalid or expired token", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(
        JSON.stringify({ detail: "invalid or expired reset token" }),
        {
          status: 400,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    const request = createMockRequest(
      "/api/parent/auth/password-reset/confirm",
      { token: "bad", new_password: "NewPassword123!" },
    );
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("invalid or expired reset token");
  });
});
