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

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "admin-token", name: "Admin User" },
  expires: "2099-01-01",
};

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/register", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    mockAuth.mockResolvedValue(null);
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("forwards unauthenticated request to backend", async () => {
    const registrationPayload = {
      email: "newuser@example.com",
      password: "Test1234!",
      first_name: "New",
      last_name: "User",
    };

    const backendResponse = {
      status: "success",
      message: "User registered",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "http://server:8080/auth/register",
    );
    expect(requestInit).toMatchObject({
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "User-Agent": "unknown",
      },
      body: JSON.stringify(registrationPayload),
    });

    expect(response.status).toBe(201);
    const json = await parseJsonResponse<typeof backendResponse>(response);
    expect(json.status).toBe("success");
  });

  it("registers user with admin authentication", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const registrationPayload = {
      email: "newuser@example.com",
      password: "Test1234!",
    };

    const backendResponse = {
      status: "success",
      message: "User registered by admin",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "http://server:8080/auth/register",
    );
    expect(requestInit).toMatchObject({
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer admin-token",
        "User-Agent": "unknown",
      },
      body: JSON.stringify(registrationPayload),
    });

    expect(response.status).toBe(201);
  });

  it("does not forward Authorization when session exists without a token", async () => {
    mockAuth.mockResolvedValueOnce({
      ...defaultSession,
      user: { ...defaultSession.user, token: undefined },
    });

    const registrationPayload = {
      email: "newuser@example.com",
      password: "Test1234!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ status: "success" }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(requestInit.headers).not.toHaveProperty("Authorization");
  });

  it("handles backend validation error", async () => {
    const registrationPayload = {
      email: "invalid-email",
      password: "weak",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          status: "error",
          error: "Invalid email format",
        }),
        {
          status: 400,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ status: string; error: string }>(
      response,
    );
    expect(json.error).toBe("Invalid email format");
  });

  it("handles conflict error (user already exists)", async () => {
    const registrationPayload = {
      email: "existing@example.com",
      password: "Test1234!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          status: "error",
          error: "User already exists",
        }),
        {
          status: 409,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);

    expect(response.status).toBe(409);
    const json = await parseJsonResponse<{ status: string; error: string }>(
      response,
    );
    expect(json.error).toBe("User already exists");
  });

  it("handles non-JSON response from backend", async () => {
    const registrationPayload = {
      email: "test@example.com",
      password: "Test1234!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("Server Error", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ status: string; error: string }>(
      response,
    );
    expect(json.error).toBe("Server Error");
  });

  it("handles JSON parse error from backend", async () => {
    const registrationPayload = {
      email: "test@example.com",
      password: "Test1234!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("invalid json", {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{ status: string; error: string }>(
      response,
    );
    expect(json.status).toBe("error");
    expect(json.error).toBe("invalid json");
  });

  it("returns fallback payload when backend sends an empty JSON body", async () => {
    const registrationPayload = {
      email: "test@example.com",
      password: "Test1234!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(null, {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);

    expect(response.status).toBe(201);
    const json = await parseJsonResponse<{ status: string; error: string }>(
      response,
    );
    expect(json.status).toBe("error");
    expect(json.error).toBe("Empty response");
  });

  it("returns fallback message when backend sends an empty non-JSON body", async () => {
    const registrationPayload = {
      email: "test@example.com",
      password: "Test1234!",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(null, {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ status: string; error: string }>(
      response,
    );
    expect(json.status).toBe("error");
    expect(json.error).toBe("Request failed with no response");
  });

  it("returns 500 on fetch failure", async () => {
    const registrationPayload = {
      email: "test@example.com",
      password: "Test1234!",
    };

    vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest(
      "/api/auth/register",
      registrationPayload,
    );
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ message: string; error: string }>(
      response,
    );
    expect(json.message).toBe("An error occurred during registration");
  });

  it("returns 500 when request parsing throws a non-Error value", async () => {
    const request = {
      json: vi.fn().mockRejectedValueOnce("bad payload"),
    } as unknown as NextRequest;

    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ message: string; error: string }>(
      response,
    );
    expect(json.message).toBe("An error occurred during registration");
    expect(json.error).toBe("bad payload");
  });
});
