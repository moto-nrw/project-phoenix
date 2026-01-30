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

  it("registers user without authentication (public registration)", async () => {
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

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/register",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(registrationPayload),
      },
    );

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

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/register",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer admin-token",
        },
        body: JSON.stringify(registrationPayload),
      },
    );

    expect(response.status).toBe(201);
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

    // When response.json() fails, the body is consumed, so response.text() also fails
    // This triggers the outer catch block, returning 500
    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ message: string; error: string }>(
      response,
    );
    expect(json.message).toBe("An error occurred during registration");
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
});
