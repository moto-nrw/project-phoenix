import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { Session } from "next-auth";
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
// Types
// ============================================================================

/**
 * Extended session type with token property used in this project
 */
interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

// Create hoisted mocks that will be available when vi.mock runs
const { mockAuth, mockApiGet, mockApiPost, mockApiPut, mockApiDelete } =
  vi.hoisted(() => ({
    mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
    mockApiGet: vi.fn(),
    mockApiPost: vi.fn(),
    mockApiPut: vi.fn(),
    mockApiDelete: vi.fn(),
  }));

// Mock auth module
vi.mock("../server/auth", () => ({
  auth: mockAuth,
}));

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

/**
 * Default authenticated session mock
 */
const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

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
    mockAuth.mockResolvedValue(defaultSession);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const handler = createGetHandler(async () => ({ data: "test" }));
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<unknown>(response);
    expect(json).toEqual({ error: "Unauthorized" });
  });

  it("returns 401 when session has no token", async () => {
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", name: "Test" },
      expires: "2099-01-01",
    } as never);

    const handler = createGetHandler(async () => ({ data: "test" }));
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("calls handler with request, token, and params", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ id: 1, name: "Test" });

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test/123");
    await handler(request, createMockContext({ id: "123" }));

    expect(mockHandler).toHaveBeenCalledWith(
      request,
      "test-token",
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
      "test-token",
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
      "test-token",
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
    mockAuth.mockResolvedValue(defaultSession);
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
      "test-token",
      expect.any(Object),
    );
    expect(response.status).toBe(200);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

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
      "test-token",
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
      "test-token",
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
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

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
      "test-token",
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
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

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
// ============================================================================

describe("createProxyGetHandler", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("proxies GET request to backend endpoint", async () => {
    const mockData = [{ id: 1, name: "Item 1" }];
    mockApiGet.mockResolvedValueOnce(mockData);

    const handler = createProxyGetHandler("/api/items");
    const request = createMockRequest("/api/items");
    const response = await handler(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/items", "test-token");
    expect(response.status).toBe(200);
  });

  it("passes query string to backend", async () => {
    mockApiGet.mockResolvedValueOnce([]);

    const handler = createProxyGetHandler("/api/items");
    const request = createMockRequest("/api/items", {
      searchParams: { page: "2", limit: "50" },
    });
    await handler(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/items?page=2&limit=50",
      "test-token",
    );
  });
});

describe("createProxyGetByIdHandler", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("proxies GET request with ID to backend", async () => {
    const mockData = { id: 123, name: "Item 123" };
    mockApiGet.mockResolvedValueOnce(mockData);

    const handler = createProxyGetByIdHandler("/api/items");
    const request = createMockRequest("/api/items/123");
    const response = await handler(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/items/123", "test-token");
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
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("proxies PUT request with ID and body to backend", async () => {
    const mockData = { id: 123, name: "Updated" };
    const body = { name: "Updated" };
    mockApiPut.mockResolvedValueOnce(mockData);

    const handler = createProxyPutHandler("/api/items");
    const request = createMockRequest("/api/items/123", {
      method: "PUT",
      body,
    });
    const response = await handler(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/items/123",
      "test-token",
      body,
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
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("proxies DELETE request with ID to backend", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const handler = createProxyDeleteHandler("/api/items");
    const request = createMockRequest("/api/items/123", { method: "DELETE" });
    const response = await handler(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith("/api/items/123", "test-token");
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
// Tests: Token Refresh / Retry Logic
// ============================================================================

describe("Token Refresh and Retry Logic", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Suppress expected console.log for retry messages
    vi.spyOn(console, "log").mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("retries request with refreshed token on 401 error", async () => {
    // First call returns valid session
    mockAuth.mockResolvedValueOnce(defaultSession);

    // Handler fails first time with 401
    let callCount = 0;
    const mockHandler = vi.fn().mockImplementation(() => {
      callCount++;
      if (callCount === 1) {
        throw new Error("API error (401): Unauthorized");
      }
      return { id: 1, name: "Success after retry" };
    });

    // Second auth call returns refreshed token
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", token: "refreshed-token", name: "Test User" },
      expires: "2099-01-01",
    });

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(mockHandler).toHaveBeenCalledTimes(2);
    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiSuccessResponse<{ name: string }>>(response);
    expect(json.data.name).toBe("Success after retry");
  });

  it("returns TOKEN_EXPIRED when token was not actually refreshed", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const mockHandler = vi
      .fn()
      .mockRejectedValue(new Error("API error (401): Unauthorized"));

    // Second auth call returns same token (not refreshed)
    mockAuth.mockResolvedValueOnce(defaultSession);

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.code).toBe("TOKEN_EXPIRED");
  });

  it("returns TOKEN_EXPIRED when retry with refreshed token also fails", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const mockHandler = vi
      .fn()
      .mockRejectedValue(new Error("API error (401): Unauthorized"));

    // Second auth call returns refreshed token
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", token: "refreshed-token", name: "Test User" },
      expires: "2099-01-01",
    });

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.code).toBe("TOKEN_EXPIRED");
  });

  it("does not retry for non-401 errors", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const mockHandler = vi
      .fn()
      .mockRejectedValue(new Error("API error (500): Internal Server Error"));

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(mockHandler).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(500);
  });
});
