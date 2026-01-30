import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest } from "next/server";

// ============================================================================
// Mocks
// ============================================================================

vi.mock("~/env", () => ({
  env: {
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

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/password-reset",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(requestBody),
      },
    );

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
      new Response(JSON.stringify({ error: "Too many requests" }), {
        status: 429,
        headers: backendHeaders,
      }),
    );

    const request = createMockRequest("/api/auth/password-reset", requestBody);
    const response = await POST(request);

    expect(response.status).toBe(429);
    expect(response.headers.get("Retry-After")).toBe("120");

    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Too many requests");
  });

  it("handles backend error with JSON response", async () => {
    const requestBody = { email: "notfound@example.com" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "User not found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/password-reset", requestBody);
    const response = await POST(request);

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("User not found");
  });

  it("handles backend error with message field", async () => {
    const requestBody = { email: "invalid@example.com" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ message: "Invalid email format" }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/password-reset", requestBody);
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Invalid email format");
  });

  it("handles non-JSON error response", async () => {
    const requestBody = { email: "user@example.com" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("Server Error", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/auth/password-reset", requestBody);
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Server Error");
  });

  it("handles empty error response body", async () => {
    const requestBody = { email: "user@example.com" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/auth/password-reset", requestBody);
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe(
      "Fehler beim Senden der Passwort-Zurücksetzen-E-Mail",
    );
  });

  it("handles JSON parse error in error response", async () => {
    const requestBody = { email: "user@example.com" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("invalid json", {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/password-reset", requestBody);
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe(
      "Fehler beim Senden der Passwort-Zurücksetzen-E-Mail",
    );
  });

  it("returns 500 on fetch failure", async () => {
    const requestBody = { email: "user@example.com" };

    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/auth/password-reset", requestBody);
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });
});
