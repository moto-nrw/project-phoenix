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

describe("POST /api/auth/password-reset/confirm", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("successfully confirms password reset", async () => {
    const requestBody = {
      token: "reset-token-123",
      password: "NewPassword123!",
    };

    const backendResponse = {
      message: "Password reset successful",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/password-reset/confirm",
      requestBody,
    );
    const response = await POST(request);

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/password-reset/confirm",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(requestBody),
      },
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<typeof backendResponse>(response);
    expect(json.message).toBe("Password reset successful");
  });

  it("returns 400 when token is invalid", async () => {
    const requestBody = {
      token: "invalid-token",
      password: "NewPassword123!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "Invalid or expired token" }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/password-reset/confirm",
      requestBody,
    );
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Invalid or expired token");
  });

  it("returns 410 when token has expired", async () => {
    const requestBody = {
      token: "expired-token",
      password: "NewPassword123!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "Token expired" }), {
        status: 410,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/password-reset/confirm",
      requestBody,
    );
    const response = await POST(request);

    expect(response.status).toBe(410);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Token expired");
  });

  it("handles backend error with message field", async () => {
    const requestBody = {
      token: "valid-token",
      password: "weak",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ message: "Password too weak" }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/password-reset/confirm",
      requestBody,
    );
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Password too weak");
  });

  it("handles non-JSON error response", async () => {
    const requestBody = {
      token: "valid-token",
      password: "NewPassword123!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("Server Error", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/password-reset/confirm",
      requestBody,
    );
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Server Error");
  });

  it("handles empty error response body", async () => {
    const requestBody = {
      token: "valid-token",
      password: "NewPassword123!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/password-reset/confirm",
      requestBody,
    );
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Fehler beim Zurücksetzen des Passworts");
  });

  it("handles JSON parse error in error response", async () => {
    const requestBody = {
      token: "valid-token",
      password: "NewPassword123!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("invalid json", {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/password-reset/confirm",
      requestBody,
    );
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Fehler beim Zurücksetzen des Passworts");
  });

  it("returns 500 on fetch failure", async () => {
    const requestBody = {
      token: "valid-token",
      password: "NewPassword123!",
    };

    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest(
      "/api/auth/password-reset/confirm",
      requestBody,
    );
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });
});
