import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest, NextResponse } from "next/server";
import {
  createGetHandler,
  createPostHandler,
  createPutHandler,
  createDeleteHandler,
} from "./route-wrapper.server";
import { mockServerSessionData } from "~/test/mocks/next-auth";
import { ApiResponseError } from "./api-response-error.server";
import {
  type ExtendedSession,
  createMockContext,
} from "~/test/route-test-helpers";

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
vi.mock("../server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockAuth,
    withTenantAuth: (handler: unknown) => handler,
  }),
);

// Mock api-helpers module
vi.mock("./api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: mockApiGet,
    apiPost: mockApiPost,
    apiPut: mockApiPut,
    apiDelete: mockApiDelete,
    handleApiError: vi.fn(mockHandleApiError),
  };
});

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
  status: number;
  detail: string;
  code?: string;
}

/**
 * Default authenticated session mock
 */
const defaultSession = mockServerSessionData() as ExtendedSession;

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
    expect(json).toMatchObject({
      status: 401,
      detail: "Unauthorized",
      code: "general.permission",
    });
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

  it("answers a Response the handler returns as it is", async () => {
    const roomsData = [
      { id: 1, name: "Room 1" },
      { id: 2, name: "Room 2" },
    ];
    const handler = createGetHandler(async () => NextResponse.json(roomsData));
    const request = createMockRequest("/api/rooms");
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    expect(await parseJsonResponse<unknown>(response)).toEqual(roomsData);
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
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Database connection failed");
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

  it("returns error response for invalid JSON body", async () => {
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

    // Malformed JSON is the client's mistake: 400, and the handler never runs.
    expect(mockHandler).not.toHaveBeenCalled();
    expect(response.status).toBe(400);
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
        throw new ApiResponseError(401, "Unauthorized");
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

  it("answers 204 when a DELETE succeeds on its retry", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", token: "refreshed-token", name: "Test User" },
      expires: "2099-01-01",
    });
    const mockHandler = vi
      .fn()
      .mockRejectedValueOnce(new ApiResponseError(401, "Unauthorized"))
      .mockResolvedValueOnce(null);

    const handler = createDeleteHandler(mockHandler);
    const response = await handler(
      createMockRequest("/api/test/1", { method: "DELETE" }),
      createMockContext({ id: "1" }),
    );

    expect(mockHandler).toHaveBeenCalledTimes(2);
    expect(response.status).toBe(204);
  });

  // A refresh that could not be decided leaves the old token: the session
  // stays, the request answers 503 and the next refresh decides.
  it("answers 503 general.unavailable when the renewal kept the old token", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);
    const mockHandler = vi
      .fn()
      .mockRejectedValue(new ApiResponseError(401, "Unauthorized"));
    mockAuth.mockResolvedValueOnce(defaultSession);

    const handler = createGetHandler(mockHandler);
    const response = await handler(
      createMockRequest("/api/test"),
      createMockContext(),
    );

    expect(mockHandler).toHaveBeenCalledOnce();
    expect(response.status).toBe(503);
    expect(await parseJsonResponse<ApiErrorResponse>(response)).toMatchObject({
      status: 503,
      code: "general.unavailable",
    });
  });

  // #5194/#5399: a refused refresh ended the session; the browser signs out.
  it("returns the 401 envelope when the renewal ended the session", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);
    const mockHandler = vi
      .fn()
      .mockRejectedValue(new ApiResponseError(401, "Unauthorized"));
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", token: "", name: "Test User" },
      error: "RefreshTokenError",
      expires: "2099-01-01",
    });

    const handler = createGetHandler(mockHandler);
    const response = await handler(
      createMockRequest("/api/test"),
      createMockContext(),
    );

    expect(mockHandler).toHaveBeenCalledOnce();
    expect(response.status).toBe(401);
    expect(await parseJsonResponse<ApiErrorResponse>(response)).toMatchObject({
      status: 401,
      detail: "Token expired",
      code: "general.permission",
    });
  });

  // The renewed token refused again is the backend's decision, not an ended
  // session: its own answer reaches the browser.
  it("answers the backend's 401 when the retry with the renewed token fails", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);
    const mockHandler = vi
      .fn()
      .mockRejectedValueOnce(new ApiResponseError(401, "Unauthorized"))
      .mockRejectedValueOnce(
        new ApiResponseError(
          401,
          JSON.stringify({
            type: "about:blank",
            title: "Unauthorized",
            status: 401,
            detail: "second factor needed",
            code: "identity.mfa_session_required",
          }),
          { contentType: "application/problem+json" },
        ),
      );
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", token: "refreshed-token", name: "Test User" },
      expires: "2099-01-01",
    });

    const handler = createGetHandler(mockHandler);
    const response = await handler(
      createMockRequest("/api/test"),
      createMockContext(),
    );

    expect(mockHandler).toHaveBeenLastCalledWith(
      expect.anything(),
      "refreshed-token",
      expect.anything(),
    );
    expect(response.status).toBe(401);
    expect(await parseJsonResponse<ApiErrorResponse>(response)).toMatchObject({
      code: "identity.mfa_session_required",
    });
  });

  it("forwards a non-401 backend error from the retry instead of the 401 envelope", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const mockHandler = vi
      .fn()
      .mockRejectedValueOnce(new ApiResponseError(401, "Unauthorized"))
      .mockRejectedValueOnce(
        new ApiResponseError(
          403,
          JSON.stringify({
            type: "about:blank",
            title: "Forbidden",
            status: 403,
            detail: "day log disabled",
            code: "students.day_log_disabled",
          }),
          { contentType: "application/problem+json" },
        ),
      );

    mockAuth.mockResolvedValueOnce({
      user: { id: "1", token: "refreshed-token", name: "Test User" },
      expires: "2099-01-01",
    });

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(mockHandler).toHaveBeenCalledTimes(2);
    expect(response.status).toBe(403);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json).toMatchObject({
      status: 403,
      code: "students.day_log_disabled",
    });
  });

  it("retries a body handler (PUT) with the refreshed token and the same body", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", token: "refreshed-token", name: "Test User" },
      expires: "2099-01-01",
    });
    const mockHandler = vi
      .fn()
      .mockRejectedValueOnce(new ApiResponseError(401, "Unauthorized"))
      .mockResolvedValueOnce({ id: 7, active: true });

    const handler = createPutHandler(mockHandler);
    const request = createMockRequest("/api/test/7", {
      method: "PUT",
      body: { active: true },
    });
    const response = await handler(request, createMockContext({ id: "7" }));

    expect(mockHandler).toHaveBeenCalledTimes(2);
    expect(mockHandler.mock.calls[0]?.slice(1, 3)).toEqual([
      { active: true },
      "test-token",
    ]);
    expect(mockHandler.mock.calls[1]?.slice(1, 3)).toEqual([
      { active: true },
      "refreshed-token",
    ]);
    expect(response.status).toBe(200);
    const json =
      await parseJsonResponse<ApiSuccessResponse<{ id: number }>>(response);
    expect(json.data).toEqual({ id: 7, active: true });
  });

  it("answers 503 from a body handler (PUT) when the renewal kept the old token", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);
    mockAuth.mockResolvedValueOnce(defaultSession);
    const mockHandler = vi
      .fn()
      .mockRejectedValue(new ApiResponseError(401, "Unauthorized"));

    const handler = createPutHandler(mockHandler);
    const request = createMockRequest("/api/test/7", {
      method: "PUT",
      body: { active: true },
    });
    const response = await handler(request, createMockContext({ id: "7" }));

    expect(mockHandler).toHaveBeenCalledOnce();
    expect(response.status).toBe(503);
    expect(await parseJsonResponse<ApiErrorResponse>(response)).toMatchObject({
      code: "general.unavailable",
    });
  });

  it("does not retry for non-401 errors", async () => {
    mockAuth.mockResolvedValueOnce(defaultSession);

    const mockHandler = vi
      .fn()
      .mockRejectedValue(new ApiResponseError(500, "Internal Server Error"));

    const handler = createGetHandler(mockHandler);
    const request = createMockRequest("/api/test");
    const response = await handler(request, createMockContext());

    expect(mockHandler).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(500);
  });
});
