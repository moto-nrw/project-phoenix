import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockSetOperatorTokens, mockFetch, mockGetServerApiUrl } = vi.hoisted(
  () => ({
    mockSetOperatorTokens: vi.fn(),
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  }),
);

vi.mock("~/lib/operator/cookies", () => ({
  setOperatorTokens: mockSetOperatorTokens,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

function createMockRequest(body: unknown): NextRequest {
  return new NextRequest("http://localhost:3000/api/operator/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("POST /api/operator/login", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("successfully logs in and sets tokens", async () => {
    const loginBody = { email: "admin@test.com", password: "password123" };
    const backendResponse = {
      status: "success",
      data: {
        access_token: "access-token-123",
        refresh_token: "refresh-token-456",
        operator: {
          id: 1,
          email: "admin@test.com",
          display_name: "Admin User",
        },
      },
      message: "Login successful",
    };

    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => backendResponse,
    });

    const request = createMockRequest(loginBody);
    const response = await POST(request);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success?: boolean;
      operator?: unknown;
      error?: string;
    };
    expect(json).toEqual({
      success: true,
      operator: {
        id: "1",
        email: "admin@test.com",
        displayName: "Admin User",
      },
    });

    expect(mockSetOperatorTokens).toHaveBeenCalledWith(
      "access-token-123",
      "refresh-token-456",
    );
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/auth/login",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(loginBody),
      }),
    );
  });

  it("returns error on invalid credentials", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      text: async () => JSON.stringify({ error: "Invalid credentials" }),
    });

    const request = createMockRequest({
      email: "wrong@test.com",
      password: "wrong",
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = (await response.json()) as {
      error?: string;
      success?: boolean;
    };
    expect(json.error).toBe("Invalid credentials");
  });

  it("uses default error message when backend error is malformed", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      text: async () => "not json",
    });

    const request = createMockRequest({
      email: "test@test.com",
      password: "test",
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = (await response.json()) as {
      error?: string;
      success?: boolean;
    };
    expect(json.error).toBe("Ungültige Anmeldedaten");
  });

  it("handles network errors gracefully", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const request = createMockRequest({
      email: "test@test.com",
      password: "test",
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = (await response.json()) as {
      error?: string;
      success?: boolean;
    };
    expect(json.error).toBe("Anmeldefehler. Bitte versuchen Sie es erneut.");
  });

  it("forwards client IP from x-forwarded-for header", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        status: "success",
        data: {
          access_token: "token",
          refresh_token: "refresh",
          operator: { id: 1, email: "test@test.com", display_name: "Test" },
        },
      }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/login",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "x-forwarded-for": "192.168.1.100, 10.0.0.1",
        },
        body: JSON.stringify({ email: "test@test.com", password: "test" }),
      },
    );

    await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({
          "X-Forwarded-For": "192.168.1.100",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("uses x-real-ip when x-forwarded-for is not present", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        status: "success",
        data: {
          access_token: "token",
          refresh_token: "refresh",
          operator: { id: 1, email: "test@test.com", display_name: "Test" },
        },
      }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/login",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "x-real-ip": "192.168.1.50",
        },
        body: JSON.stringify({ email: "test@test.com", password: "test" }),
      },
    );

    await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({
          "X-Forwarded-For": "192.168.1.50",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("uses 'unknown' IP when no IP headers present", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        status: "success",
        data: {
          access_token: "token",
          refresh_token: "refresh",
          operator: { id: 1, email: "test@test.com", display_name: "Test" },
        },
      }),
    });

    const request = createMockRequest({
      email: "test@test.com",
      password: "test",
    });
    await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({
          "X-Forwarded-For": "unknown",
        }) as Record<string, unknown>,
      }),
    );
  });
});
