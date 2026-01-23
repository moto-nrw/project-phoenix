import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest } from "next/server";
import {
  isStringParam,
  createGetHandler,
  createPostHandler,
  createPutHandler,
  createDeleteHandler,
  createProxyGetHandler,
  createProxyGetByIdHandler,
  createProxyPutHandler,
  createProxyDeleteHandler,
} from "./route-wrapper";

// ============================================================================
// Constants
// ============================================================================

// BetterAuth: The "token" parameter is now the cookie header string
const TEST_COOKIE_HEADER = "better-auth.session_token=test-session-token";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

// Create hoisted mocks that will be available when vi.mock runs
const { mockApiGet, mockApiPost, mockApiPut, mockApiDelete } = vi.hoisted(
  () => ({
    mockApiGet: vi.fn(),
    mockApiPost: vi.fn(),
    mockApiPut: vi.fn(),
    mockApiDelete: vi.fn(),
  }),
);

// Note: auth() is globally mocked in setup.ts
// It checks for better-auth.session_token cookie

// Mock api-helpers module
vi.mock("./api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

// ============================================================================
// Test Helpers
// ============================================================================

/**
 * Creates a mock NextRequest for testing route handlers
 */
function createMockRequest(
  path: string,
  options: {
    method?: string;
    body?: unknown;
    searchParams?: Record<string, string>;
  } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  if (options.searchParams) {
    Object.entries(options.searchParams).forEach(([key, value]) => {
      url.searchParams.set(key, value);
    });
  }

  // Use NextRequest's expected init format
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

/**
 * Creates a mock route context with params (Next.js 15+ pattern)
 */
function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

/**
 * Helper to parse JSON response with proper typing
 */
async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

/**
 * Types for common API responses
 */
interface ApiSuccessResponse<T = unknown> {
  success: boolean;
  message: string;
  data: T;
}

interface ApiErrorResponse {
  error: string;
  code?: string;
}

// Note: BetterAuth doesn't use session tokens in the session object
// The cookie header is used for authentication instead

// ============================================================================
// Tests: isStringParam (pure function)
// ============================================================================

describe("isStringParam", () => {
  it("returns true for string values", () => {
    expect(isStringParam("hello")).toBe(true);
    expect(isStringParam("")).toBe(true);
    expect(isStringParam("123")).toBe(true);
  });

  it("returns false for numbers", () => {
    expect(isStringParam(123)).toBe(false);
    expect(isStringParam(0)).toBe(false);
    expect(isStringParam(-1)).toBe(false);
  });

  it("returns false for null and undefined", () => {
    expect(isStringParam(null)).toBe(false);
    expect(isStringParam(undefined)).toBe(false);
  });

  it("returns false for objects and arrays", () => {
    expect(isStringParam({})).toBe(false);
    expect(isStringParam([])).toBe(false);
    expect(isStringParam(["a", "b"])).toBe(false);
  });

  it("returns false for booleans", () => {
    expect(isStringParam(true)).toBe(false);
    expect(isStringParam(false)).toBe(false);
  });
});

// ============================================================================
// Tests: createGetHandler
// ============================================================================

describe("createGetHandler", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Global mock provides authenticated session by default
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns 401 when not authenticated", async () => {
    // Mock cookies to return undefined (no session cookie)
    const { cookies } = await import("next/headers");
    vi.mocked(cookies).mockResolvedValueOnce({
      get: vi.fn(() => undefined),
      toString: vi.fn(() => ""),
    } as never);

    const handler = createGetHandler(async () => ({ data: "test" }));
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<unknown>(response);
    expect(json).toEqual({ error: "Unauthorized" });
  });

  it("returns 401 when session has no token", async () => {
    // Mock cookies to return undefined (no valid session token)
    const { cookies } = await import("next/headers");
    vi.mocked(cookies).mockResolvedValueOnce({
      get: vi.fn(() => undefined),
      toString: vi.fn(() => ""),
    } as never);

    const handler = createGetHandler(async () => ({ data: "test" }));
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("calls handler with request, cookie header, and params", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ id: 1, name: "Test" });

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test/123");
    await handler(request, createMockContext({ id: "123" }));

    // BetterAuth: second param is now cookie header string (not JWT token)
    expect(mockHandler).toHaveBeenCalledWith(
      request,
      TEST_COOKIE_HEADER,
      expect.objectContaining({ id: "123" }),
    );
  });

  it("wraps response in ApiResponse format", async () => {
    const handler = createGetHandler(async () => ({ id: 1, name: "Test" }));
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<unknown>(response);
    expect(json).toEqual({
      success: true,
      message: "Success",
      data: { id: 1, name: "Test" },
    });
  });

  it("returns raw data for /api/rooms endpoint (special case)", async () => {
    const roomsData = [
      { id: 1, name: "Room 1" },
      { id: 2, name: "Room 2" },
    ];
    const handler = createGetHandler(async () => roomsData);
    const request = createMockRequest("/api/rooms");
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<unknown>(response);
    // Rooms endpoint returns raw data, not wrapped
    expect(json).toEqual(roomsData);
  });

  it("passes through response if already wrapped with success field", async () => {
    const alreadyWrapped = { success: true, data: { id: 1 } };
    const handler = createGetHandler(async () => alreadyWrapped);
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    const json = await parseJsonResponse<unknown>(response);
    expect(json).toEqual(alreadyWrapped);
  });

  it("extracts ID from URL path when not in context params", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ id: 1 });

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/students/456");
    await handler(request, createMockContext({}));

    expect(mockHandler).toHaveBeenCalledWith(
      request,
      TEST_COOKIE_HEADER,
      expect.objectContaining({ id: "456" }),
    );
  });

  it("includes search params in extracted params", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ data: [] });

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/students", {
      searchParams: { search: "Max", page: "2" },
    });
    await handler(request, createMockContext());

    expect(mockHandler).toHaveBeenCalledWith(
      request,
      TEST_COOKIE_HEADER,
      expect.objectContaining({ search: "Max", page: "2" }),
    );
  });

  it("handles errors by calling handleApiError", async () => {
    const handler = createGetHandler(async () => {
      throw new Error("Database connection failed");
    });
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Database connection failed");
  });
});

// ============================================================================
// Tests: createPostHandler
// ============================================================================

describe("createPostHandler", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Global mock provides authenticated session by default
  });

  it("handles invalid JSON body gracefully by passing empty object", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ id: 1, created: true });

    const handler = createPostHandler(mockHandler);

    // Create a request with invalid JSON body
    const url = new URL("/api/test", "http://localhost:3000");
    const request = new NextRequest(url, {
      method: "POST",
      body: "this is not valid JSON {{{",
      headers: { "Content-Type": "application/json" },
    });

    const response = await handler(request, createMockContext());

    // Handler should receive empty object when JSON parsing fails
    expect(mockHandler).toHaveBeenCalledWith(
      request,
      {}, // Empty object due to parse error
      TEST_COOKIE_HEADER,
      expect.any(Object),
    );
    expect(response.status).toBe(200);
  });

  it("returns 401 when not authenticated", async () => {
    // Mock cookies to return undefined (no session cookie)
    const { cookies } = await import("next/headers");
    vi.mocked(cookies).mockResolvedValueOnce({
      get: vi.fn(() => undefined),
      toString: vi.fn(() => ""),
    } as never);

    const handler = createPostHandler(async () => ({ id: 1 }));
    const request = createMockRequest("/api/test", {
      method: "POST",
      body: { name: "Test" },
    });
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("parses request body and passes to handler", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ id: 1, name: "Created" });
    const body = { name: "New Item", description: "Test" };

    const handler = createPostHandler(mockHandler);
    const request = createMockRequest("/api/test", { method: "POST", body });
    await handler(request, createMockContext());

    expect(mockHandler).toHaveBeenCalledWith(
      request,
      body,
      TEST_COOKIE_HEADER,
      expect.any(Object),
    );
  });

  it("handles empty body gracefully", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ id: 1 });

    const handler = createPostHandler(mockHandler);
    const request = createMockRequest("/api/test", { method: "POST" });
    await handler(request, createMockContext());

    expect(mockHandler).toHaveBeenCalledWith(
      request,
      {}, // Empty object for missing body
      TEST_COOKIE_HEADER,
      expect.any(Object),
    );
  });

  it("wraps response in ApiResponse format", async () => {
    const handler = createPostHandler(async () => ({ id: 1, name: "Created" }));
    const request = createMockRequest("/api/test", {
      method: "POST",
      body: { name: "Test" },
    });
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<unknown>(response);
    expect(json).toEqual({
      success: true,
      message: "Success",
      data: { id: 1, name: "Created" },
    });
  });
});

// ============================================================================
// Tests: createPutHandler
// ============================================================================

describe("createPutHandler", () => {
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

    const handler = createPutHandler(async () => ({ id: 1 }));
    const request = createMockRequest("/api/test/1", {
      method: "PUT",
      body: { name: "Updated" },
    });
    const response = await handler(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("parses request body and passes to handler with params", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ id: 1, name: "Updated" });
    const body = { name: "Updated Name" };

    const handler = createPutHandler(mockHandler);
    const request = createMockRequest("/api/test/123", { method: "PUT", body });
    await handler(request, createMockContext({ id: "123" }));

    expect(mockHandler).toHaveBeenCalledWith(
      request,
      body,
      TEST_COOKIE_HEADER,
      expect.objectContaining({ id: "123" }),
    );
  });

  it("wraps response in ApiResponse format", async () => {
    const handler = createPutHandler(async () => ({ id: 1, name: "Updated" }));
    const request = createMockRequest("/api/test/1", {
      method: "PUT",
      body: { name: "Updated" },
    });
    const response = await handler(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<unknown>(response);
    expect(json).toEqual({
      success: true,
      message: "Success",
      data: { id: 1, name: "Updated" },
    });
  });
});

// ============================================================================
// Tests: createDeleteHandler
// ============================================================================

describe("createDeleteHandler", () => {
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

    const handler = createDeleteHandler(async () => null);
    const request = createMockRequest("/api/test/1", { method: "DELETE" });
    const response = await handler(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("returns 204 No Content for successful deletion with null response", async () => {
    const handler = createDeleteHandler(async () => null);
    const request = createMockRequest("/api/test/1", { method: "DELETE" });
    const response = await handler(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(204);
  });

  it("returns 204 No Content for successful deletion with undefined response", async () => {
    const handler = createDeleteHandler(async () => undefined);
    const request = createMockRequest("/api/test/1", { method: "DELETE" });
    const response = await handler(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(204);
  });

  it("returns wrapped response if handler returns data", async () => {
    const handler = createDeleteHandler(async () => ({
      message: "Deleted successfully",
    }));
    const request = createMockRequest("/api/test/1", { method: "DELETE" });
    const response = await handler(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<unknown>(response);
    expect(json).toEqual({
      success: true,
      message: "Success",
      data: { message: "Deleted successfully" },
    });
  });
});

// ============================================================================
// Tests: Proxy Handlers
// Note: These handlers use apiGetWithCookies/apiPutWithCookies/apiDeleteWithCookies
// which call fetch directly, so we mock fetch instead of api-helpers
// ============================================================================

// Helper to create a mock fetch response
function createMockFetchResponse(
  data: unknown,
  status = 200,
): Promise<Response> {
  return Promise.resolve(
    new Response(JSON.stringify(data), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

describe("createProxyGetHandler", () => {
  const mockFetch = vi.fn();
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    globalThis.fetch = mockFetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("proxies GET request to backend endpoint", async () => {
    const mockData = [{ id: 1, name: "Item 1" }];
    mockFetch.mockReturnValueOnce(createMockFetchResponse(mockData));

    const handler = createProxyGetHandler("/api/items");
    const request = createMockRequest("/api/items");
    const response = await handler(request, createMockContext());

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/items"),
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Cookie: TEST_COOKIE_HEADER,
        }),
      }),
    );
    expect(response.status).toBe(200);
  });

  it("passes query string to backend", async () => {
    mockFetch.mockReturnValueOnce(createMockFetchResponse([]));

    const handler = createProxyGetHandler("/api/items");
    const request = createMockRequest("/api/items", {
      searchParams: { page: "2", limit: "50" },
    });
    await handler(request, createMockContext());

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/items?page=2&limit=50"),
      expect.anything(),
    );
  });
});

describe("createProxyGetByIdHandler", () => {
  const mockFetch = vi.fn();
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    globalThis.fetch = mockFetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("proxies GET request with ID to backend", async () => {
    const mockData = { id: 123, name: "Item 123" };
    mockFetch.mockReturnValueOnce(createMockFetchResponse(mockData));

    const handler = createProxyGetByIdHandler("/api/items");
    const request = createMockRequest("/api/items/123");
    const response = await handler(request, createMockContext({ id: "123" }));

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/items/123"),
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Cookie: TEST_COOKIE_HEADER,
        }),
      }),
    );
    expect(response.status).toBe(200);
  });

  it("throws error for invalid id parameter", async () => {
    const handler = createProxyGetByIdHandler("/api/items");
    const request = createMockRequest("/api/items");
    const response = await handler(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid id parameter");
  });
});

describe("createProxyPutHandler", () => {
  const mockFetch = vi.fn();
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    globalThis.fetch = mockFetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("proxies PUT request with ID and body to backend", async () => {
    const mockData = { id: 123, name: "Updated" };
    const body = { name: "Updated" };
    mockFetch.mockReturnValueOnce(createMockFetchResponse(mockData));

    const handler = createProxyPutHandler("/api/items");
    const request = createMockRequest("/api/items/123", {
      method: "PUT",
      body,
    });
    const response = await handler(request, createMockContext({ id: "123" }));

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/items/123"),
      expect.objectContaining({
        method: "PUT",
        headers: expect.objectContaining({
          Cookie: TEST_COOKIE_HEADER,
        }),
        body: JSON.stringify(body),
      }),
    );
    expect(response.status).toBe(200);
  });

  it("throws error for invalid id parameter", async () => {
    const handler = createProxyPutHandler("/api/items");
    const request = createMockRequest("/api/items", {
      method: "PUT",
      body: { name: "Test" },
    });
    const response = await handler(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid id parameter");
  });
});

describe("createProxyDeleteHandler", () => {
  const mockFetch = vi.fn();
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    globalThis.fetch = mockFetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("proxies DELETE request with ID to backend", async () => {
    mockFetch.mockReturnValueOnce(
      Promise.resolve(new Response(null, { status: 204 })),
    );

    const handler = createProxyDeleteHandler("/api/items");
    const request = createMockRequest("/api/items/123", { method: "DELETE" });
    const response = await handler(request, createMockContext({ id: "123" }));

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/items/123"),
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({
          Cookie: TEST_COOKIE_HEADER,
        }),
      }),
    );
    expect(response.status).toBe(204);
  });

  it("throws error for invalid id parameter", async () => {
    const handler = createProxyDeleteHandler("/api/items");
    const request = createMockRequest("/api/items", { method: "DELETE" });
    const response = await handler(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid id parameter");
  });
});

// ============================================================================
// Tests: Error Handling
// Note: BetterAuth uses cookie-based sessions without JWT token refresh.
// Sessions are managed server-side; no retry logic needed.
// ============================================================================

describe("Error Handling", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 401 for unauthorized errors from handler", async () => {
    const mockHandler = vi
      .fn()
      .mockRejectedValue(new Error("API error (401): Unauthorized"));

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(mockHandler).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(401);
  });

  it("returns 500 for server errors from handler", async () => {
    const mockHandler = vi
      .fn()
      .mockRejectedValue(new Error("API error (500): Internal Server Error"));

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(mockHandler).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(500);
  });

  it("returns 404 for not found errors from handler", async () => {
    const mockHandler = vi
      .fn()
      .mockRejectedValue(new Error("API error (404): Not Found"));

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(mockHandler).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(404);
  });
});
