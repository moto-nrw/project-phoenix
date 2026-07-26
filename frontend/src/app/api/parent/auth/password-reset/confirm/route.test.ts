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
        JSON.stringify({ error: "invalid or expired reset token" }),
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
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("invalid or expired reset token");
  });

  it("falls back to the message field when error is absent", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ message: "Password too weak" }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/parent/auth/password-reset/confirm",
      { token: "valid", new_password: "weak" },
    );
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Password too weak");
  });

  it("returns non-JSON error bodies verbatim", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("Server Error", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest(
      "/api/parent/auth/password-reset/confirm",
      { token: "valid", new_password: "NewPassword123!" },
    );
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Server Error");
  });

  it("uses the German fallback message for an empty error body", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest(
      "/api/parent/auth/password-reset/confirm",
      { token: "valid", new_password: "NewPassword123!" },
    );
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Fehler beim Zurücksetzen des Passworts");
  });

  it("uses the German fallback when the JSON error body is unparseable", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => undefined);

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("invalid json", {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/parent/auth/password-reset/confirm",
      { token: "valid", new_password: "NewPassword123!" },
    );
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Fehler beim Zurücksetzen des Passworts");
  });

  it("returns 500 when the upstream fetch throws", async () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);

    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest(
      "/api/parent/auth/password-reset/confirm",
      { token: "valid", new_password: "NewPassword123!" },
    );
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });
});
