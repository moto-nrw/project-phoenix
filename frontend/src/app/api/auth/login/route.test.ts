import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest } from "next/server";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
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

describe("POST /api/auth/login", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("forwards login request to backend and returns JSON response", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };
    const backendResponse = {
      access_token: "jwt-token",
      refresh_token: "refresh-token",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "http://server:8080/auth/login",
    );
    expect(requestInit).toMatchObject({
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "User-Agent": "unknown",
      },
      body: JSON.stringify(loginPayload),
    });

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<typeof backendResponse>(response);
    expect(json.access_token).toBe("jwt-token");
    expect(json.refresh_token).toBe("refresh-token");
  });

  it("returns 401 when credentials are invalid", async () => {
    const loginPayload = { email: "test@example.com", password: "wrong" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "Invalid credentials" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Invalid credentials");
  });

  it("handles non-JSON response from backend", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("Server Error", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ message: string }>(response);
    expect(json.message).toBe("Server Error");
  });

  it("handles JSON parse error from backend", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("invalid json", {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{ message: string }>(response);
    expect(json.message).toBe("invalid json");
  });

  it("returns fallback payload when backend sends an empty JSON body", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(null, {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{ message: string }>(response);
    expect(json.message).toBe("Empty response");
  });

  it("returns fallback message when backend sends an empty non-JSON body", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(null, {
        status: 502,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(502);
    const json = await parseJsonResponse<{ message: string }>(response);
    expect(json.message).toBe("Request failed with no response");
  });

  it("returns 500 when fetch throws an error", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };

    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });

  it("returns 500 when request parsing throws a non-Error value", async () => {
    const request = {
      json: vi.fn().mockRejectedValueOnce("bad payload"),
    } as unknown as NextRequest;

    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });
});
