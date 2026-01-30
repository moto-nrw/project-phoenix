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

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "POST",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
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

describe("POST /api/invitations/accept", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("accepts invitation successfully", async () => {
    const requestPayload = {
      token: "invitation-token-123",
      firstName: "John",
      lastName: "Doe",
      password: "Test1234!",
      confirmPassword: "Test1234!",
    };

    const backendResponse = {
      status: "success",
      message: "Invitation accepted",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/invitations/accept", {
      body: requestPayload,
    });
    const response = await POST(request);

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/invitations/invitation-token-123/accept",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          first_name: "John",
          last_name: "Doe",
          password: "Test1234!",
          confirm_password: "Test1234!",
        }),
      },
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<typeof backendResponse>(response);
    expect(json.status).toBe("success");
  });

  it("returns 400 when token is missing", async () => {
    const requestPayload = {
      password: "Test1234!",
      confirmPassword: "Test1234!",
    };

    const request = createMockRequest("/api/invitations/accept", {
      body: requestPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Missing invitation token");
  });

  it("handles backend error responses", async () => {
    const requestPayload = {
      token: "expired-token",
      password: "Test1234!",
      confirmPassword: "Test1234!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "Token expired" }), {
        status: 410,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/invitations/accept", {
      body: requestPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(410);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Token expired");
  });

  it("handles non-JSON response from backend", async () => {
    const requestPayload = {
      token: "invitation-token",
      password: "Test1234!",
      confirmPassword: "Test1234!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("Server Error", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/invitations/accept", {
      body: requestPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Server Error");
  });

  it("handles empty response body", async () => {
    const requestPayload = {
      token: "invitation-token",
      password: "Test1234!",
      confirmPassword: "Test1234!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("", {
        status: 204,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/invitations/accept", {
      body: requestPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(204);
    const json = await parseJsonResponse<Record<string, unknown>>(response);
    expect(json).toEqual({});
  });

  it("returns 500 on fetch failure", async () => {
    const requestPayload = {
      token: "invitation-token",
      password: "Test1234!",
      confirmPassword: "Test1234!",
    };

    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/invitations/accept", {
      body: requestPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });
});
