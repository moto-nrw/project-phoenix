import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { GET, PUT, DELETE } from "./route";

// ============================================================================
// Mocks
// ============================================================================

// Mock global fetch since the route handlers use it internally
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Note: auth() is globally mocked in setup.ts
// It checks for better-auth.session_token cookie

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
      method: options.method ?? "GET",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
}

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

// BetterAuth: Expected cookie header (from global setup.ts mock)
const TEST_COOKIE_HEADER = "better-auth.session_token=test-session-token";

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/active/visits/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Global mock provides authenticated session by default
  });

  it("returns 401 when not authenticated", async () => {
    // Mock cookies to return undefined (no session cookie)
    const { cookies } = await import("next/headers");
    vi.mocked(cookies).mockResolvedValueOnce({
      get: vi.fn(() => undefined),
      toString: vi.fn(() => ""),
    } as never);

    const request = createMockRequest("/api/active/visits/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("fetches visit by ID from backend", async () => {
    const mockVisit = {
      id: 123,
      student_id: 1,
      active_group_id: 1,
      start_time: "2024-01-15T09:00:00Z",
    };
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockVisit,
    });

    const request = createMockRequest("/api/active/visits/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    // BetterAuth: Server-side uses Cookie header
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/active/visits/123"),
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Cookie: TEST_COOKIE_HEADER,
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);
  });
});

describe("PUT /api/active/visits/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Global mock provides authenticated session by default
  });

  it("returns 401 when not authenticated", async () => {
    // Mock cookies to return undefined (no session cookie)
    const { cookies } = await import("next/headers");
    vi.mocked(cookies).mockResolvedValueOnce({
      get: vi.fn(() => undefined),
      toString: vi.fn(() => ""),
    } as never);

    const request = createMockRequest("/api/active/visits/123", {
      method: "PUT",
      body: { end_time: "2024-01-15T17:00:00Z" },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("updates visit via backend", async () => {
    const updateBody = { end_time: "2024-01-15T17:00:00Z" };
    const mockUpdatedVisit = {
      id: 123,
      student_id: 1,
      active_group_id: 1,
      start_time: "2024-01-15T09:00:00Z",
      end_time: "2024-01-15T17:00:00Z",
    };
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockUpdatedVisit,
    });

    const request = createMockRequest("/api/active/visits/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    // BetterAuth: Server-side uses Cookie header
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/active/visits/123"),
      expect.objectContaining({
        method: "PUT",
        headers: expect.objectContaining({
          Cookie: TEST_COOKIE_HEADER,
        }) as Record<string, string>,
        body: JSON.stringify(updateBody),
      }),
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedVisit>>(response);
    expect(json.data.end_time).toBe("2024-01-15T17:00:00Z");
  });
});

describe("DELETE /api/active/visits/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Global mock provides authenticated session by default
  });

  it("returns 401 when not authenticated", async () => {
    // Mock cookies to return undefined (no session cookie)
    const { cookies } = await import("next/headers");
    vi.mocked(cookies).mockResolvedValueOnce({
      get: vi.fn(() => undefined),
      toString: vi.fn(() => ""),
    } as never);

    const request = createMockRequest("/api/active/visits/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("deletes visit via backend and returns 204", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 204,
    });

    const request = createMockRequest("/api/active/visits/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    // BetterAuth: Server-side uses Cookie header
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/active/visits/123"),
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({
          Cookie: TEST_COOKIE_HEADER,
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(204);
  });
});
