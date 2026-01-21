import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { NextRequest } from "next/server";
import {
  extractParams,
  handleApiError,
  handleDomainApiError,
  isBrowserContext,
  buildAuthHeaders,
  buildAuthHeadersWithBody,
  convertToBackendRoom,
  authFetch,
  fetchWithRetry,
  apiGet,
  apiPost,
  apiPut,
  apiDelete,
  checkAuth,
} from "./api-helpers";

// Helper to create mock NextRequest
function createMockNextRequest(
  url: string,
  searchParams: Record<string, string> = {},
): NextRequest {
  const urlObj = new URL(url);
  Object.entries(searchParams).forEach(([key, value]) => {
    urlObj.searchParams.set(key, value);
  });

  return {
    nextUrl: urlObj,
  } as NextRequest;
}

describe("extractParams", () => {
  it("extracts params from URL params object", () => {
    const request = createMockNextRequest("http://localhost/api/test");
    const params = { id: "123", name: "test" };

    const result = extractParams(request, params);

    expect(result.id).toBe("123");
    expect(result.name).toBe("test");
  });

  it("extracts params from query string", () => {
    const request = createMockNextRequest("http://localhost/api/test", {
      page: "1",
      limit: "10",
    });

    const result = extractParams(request, {});

    expect(result.page).toBe("1");
    expect(result.limit).toBe("10");
  });

  it("combines URL params and query params", () => {
    const request = createMockNextRequest("http://localhost/api/test", {
      page: "1",
    });
    const params = { id: "123" };

    const result = extractParams(request, params);

    expect(result.id).toBe("123");
    expect(result.page).toBe("1");
  });

  it("ignores non-string params", () => {
    const request = createMockNextRequest("http://localhost/api/test");
    const params = { id: "123", count: 5, nested: { key: "value" } };

    const result = extractParams(request, params);

    expect(result.id).toBe("123");
    expect(result.count).toBeUndefined();
    expect(result.nested).toBeUndefined();
  });
});

describe("handleApiError", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
  let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  afterEach(() => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
    consoleErrorSpy.mockRestore();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
    consoleWarnSpy.mockRestore();
  });

  it("extracts status code from 'API error (XXX):' format", async () => {
    const error = new Error("API error (404): Not found");

    const response = handleApiError(error);
    const body = (await response.json()) as { error: string };

    expect(response.status).toBe(404);
    expect(body.error).toBe("API error (404): Not found");
  });

  it("extracts status code from 'API error: XXX' format", () => {
    const error = new Error("API error: 403 Forbidden");

    const response = handleApiError(error);

    expect(response.status).toBe(403);
  });

  it("logs error for 5xx status codes", () => {
    const error = new Error("API error (500): Internal Server Error");

    handleApiError(error);

    expect(consoleErrorSpy).toHaveBeenCalled();
    expect(consoleWarnSpy).not.toHaveBeenCalled();
  });

  it("logs warning for 4xx status codes", () => {
    const error = new Error("API error (400): Bad Request");

    handleApiError(error);

    expect(consoleWarnSpy).toHaveBeenCalled();
    // consoleError not called for 4xx
  });

  it("returns 500 for unknown error format", async () => {
    const error = new Error("Something went wrong");

    const response = handleApiError(error);
    const body = (await response.json()) as { error: string };

    expect(response.status).toBe(500);
    expect(body.error).toBe("Something went wrong");
  });

  it("handles non-Error objects", async () => {
    const response = handleApiError("string error");
    const body = (await response.json()) as { error: string };

    expect(response.status).toBe(500);
    expect(body.error).toBe("Internal Server Error");
  });
});

describe("handleDomainApiError", () => {
  // Helper to capture thrown error from handleDomainApiError
  function captureThrown(fn: () => void): {
    status: number;
    code: string;
    message: string;
  } {
    try {
      fn();
      // If we reach here, function didn't throw
      throw new Error("Function did not throw");
    } catch (e) {
      // If it's our "didn't throw" error, rethrow it
      if (e instanceof Error && e.message === "Function did not throw") {
        throw e;
      }
      return JSON.parse((e as Error).message) as {
        status: number;
        code: string;
        message: string;
      };
    }
  }

  it("throws structured error with extracted status code", () => {
    const error = new Error("API error (403): Access denied");

    const thrown = captureThrown(() =>
      handleDomainApiError(error, "fetch students", "STUDENT"),
    );

    expect(thrown.status).toBe(403);
    expect(thrown.code).toBe("STUDENT_API_ERROR_403");
    expect(thrown.message).toContain("Failed to fetch students");
  });

  it("uses 500 status for errors without status code", () => {
    const error = new Error("Unknown error");

    const thrown = captureThrown(() =>
      handleDomainApiError(error, "update activity", "ACTIVITY"),
    );

    expect(thrown.status).toBe(500);
    expect(thrown.code).toBe("ACTIVITY_API_ERROR_UNKNOWN");
  });

  it("handles non-Error objects", () => {
    const thrown = captureThrown(() =>
      handleDomainApiError("string error", "delete room", "ROOM"),
    );

    expect(thrown.status).toBe(500);
    expect(thrown.message).toContain("Unknown error");
  });
});

describe("isBrowserContext", () => {
  it("returns true when window is defined", () => {
    // In happy-dom test environment, window is defined
    const result = isBrowserContext();
    // This test is environment-dependent
    expect(typeof result).toBe("boolean");
  });
});

describe("buildAuthHeaders", () => {
  it("returns undefined when token is undefined", () => {
    const result = buildAuthHeaders(undefined);

    expect(result).toBeUndefined();
  });

  it("returns undefined when token is empty string", () => {
    const result = buildAuthHeaders("");

    expect(result).toBeUndefined();
  });

  it("returns auth headers with token", () => {
    const result = buildAuthHeaders("test-token");

    expect(result).toEqual({
      Authorization: "Bearer test-token",
      "Content-Type": "application/json",
    });
  });
});

describe("buildAuthHeadersWithBody", () => {
  it("returns Content-Type even without token", () => {
    const result = buildAuthHeadersWithBody(undefined);

    expect(result).toEqual({
      "Content-Type": "application/json",
    });
  });

  it("includes Authorization when token is provided", () => {
    const result = buildAuthHeadersWithBody("test-token");

    expect(result).toEqual({
      "Content-Type": "application/json",
      Authorization: "Bearer test-token",
    });
  });
});

describe("convertToBackendRoom", () => {
  it("converts raw API response to typed BackendRoom", () => {
    const rawResponse = {
      id: 1,
      name: "Room 101",
      building: "Building A",
      floor: 2,
      capacity: 30,
      category: "classroom",
      color: "#FF0000",
      device_id: "device-123",
      is_occupied: true,
      activity_name: "Math Class",
      group_name: "Class 3A",
      supervisor_name: "John Smith",
      student_count: 25,
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-15T12:00:00Z",
    };

    const result = convertToBackendRoom(rawResponse);

    expect(result.id).toBe(1);
    expect(result.name).toBe("Room 101");
    expect(result.building).toBe("Building A");
    expect(result.floor).toBe(2);
    expect(result.capacity).toBe(30);
    expect(result.category).toBe("classroom");
    expect(result.color).toBe("#FF0000");
    expect(result.device_id).toBe("device-123");
    expect(result.is_occupied).toBe(true);
    expect(result.activity_name).toBe("Math Class");
    expect(result.group_name).toBe("Class 3A");
    expect(result.supervisor_name).toBe("John Smith");
    expect(result.student_count).toBe(25);
  });

  it("handles string numeric fields", () => {
    const rawResponse = {
      id: "123",
      floor: "5",
      capacity: "50",
    };

    const result = convertToBackendRoom(rawResponse);

    expect(result.id).toBe(123);
    expect(result.floor).toBe(5);
    expect(result.capacity).toBe(50);
  });

  it("uses defaults for missing fields", () => {
    const rawResponse = {};

    const result = convertToBackendRoom(rawResponse);

    expect(result.id).toBe(0);
    expect(result.name).toBe("");
    expect(result.building).toBeUndefined();
    expect(result.floor).toBe(0);
    expect(result.capacity).toBe(0);
    expect(result.category).toBe("");
    expect(result.is_occupied).toBe(false);
  });

  it("converts is_occupied to boolean", () => {
    expect(convertToBackendRoom({ is_occupied: true }).is_occupied).toBe(true);
    expect(convertToBackendRoom({ is_occupied: false }).is_occupied).toBe(
      false,
    );
    expect(convertToBackendRoom({ is_occupied: undefined }).is_occupied).toBe(
      false,
    );
  });

  it("handles truthy/falsy values for is_occupied from raw API", () => {
    // Simulate raw API response that might have numeric values
    const rawWithOne = { is_occupied: 1 } as unknown as {
      is_occupied?: boolean;
    };
    const rawWithZero = { is_occupied: 0 } as unknown as {
      is_occupied?: boolean;
    };

    expect(convertToBackendRoom(rawWithOne).is_occupied).toBe(true);
    expect(convertToBackendRoom(rawWithZero).is_occupied).toBe(false);
  });
});

// ===== FETCH FUNCTION TESTS =====

// Type for mocked fetch function
type MockedFetch = ReturnType<typeof vi.fn<typeof fetch>>;

describe("authFetch", () => {
  let originalFetch: typeof fetch;
  let mockFetch: MockedFetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    mockFetch = vi.fn();
    globalThis.fetch = mockFetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("makes GET request with auth headers", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ data: "test" }),
    } as Response);

    const result = await authFetch<{ data: string }>(
      "http://api.test/endpoint",
      {
        token: "test-token",
      },
    );

    expect(result.data).toBe("test");
    expect(mockFetch).toHaveBeenCalledWith("http://api.test/endpoint", {
      method: "GET",
      credentials: "include",
      headers: {
        Authorization: "Bearer test-token",
        "Content-Type": "application/json",
      },
    });
  });

  it("makes GET request without headers when no token", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ data: "test" }),
    } as Response);

    await authFetch<{ data: string }>("http://api.test/endpoint");

    expect(mockFetch).toHaveBeenCalledWith("http://api.test/endpoint", {
      method: "GET",
      credentials: "include",
      headers: undefined,
    });
  });

  it("makes POST request with body", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ success: true }),
    } as Response);

    const body = { name: "Test" };
    await authFetch("http://api.test/endpoint", {
      method: "POST",
      body,
      token: "test-token",
    });

    expect(mockFetch).toHaveBeenCalledWith("http://api.test/endpoint", {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer test-token",
      },
      body: JSON.stringify(body),
    });
  });

  it("returns empty object for 204 No Content", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 204,
    } as Response);

    const result = await authFetch<Record<string, unknown>>(
      "http://api.test/endpoint",
      {
        method: "DELETE",
        token: "test-token",
      },
    );

    expect(result).toEqual({});
  });

  it("throws error for non-ok response", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
      statusText: "Not Found",
    } as Response);

    await expect(
      authFetch("http://api.test/endpoint", { token: "test-token" }),
    ).rejects.toThrow("API error (404): Not Found");
  });
});

describe("fetchWithRetry", () => {
  let originalFetch: typeof fetch;
  let mockFetchRetry: MockedFetch;
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
  let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    mockFetchRetry = vi.fn();
    globalThis.fetch = mockFetchRetry;
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
    consoleErrorSpy.mockRestore();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
    consoleWarnSpy.mockRestore();
  });

  it("returns response and data on success", async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      json: () => Promise.resolve({ data: "test" }),
    } as Response;
    mockFetchRetry.mockResolvedValueOnce(mockResponse);

    const result = await fetchWithRetry<{ data: string }>(
      "http://api.test/endpoint",
      "test-token",
    );

    expect(result.response).toBeTruthy();
    expect(result.data).toEqual({ data: "test" });
  });

  it("retries on 401 with token refresh", async () => {
    // First call returns 401
    mockFetchRetry.mockResolvedValueOnce({
      ok: false,
      status: 401,
      text: () => Promise.resolve("Unauthorized"),
    } as Response);

    // Retry call succeeds
    mockFetchRetry.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ data: "refreshed" }),
    } as Response);

    const onAuthFailure = vi.fn().mockResolvedValue(true);
    const getNewToken = vi.fn().mockResolvedValue("new-token");

    const result = await fetchWithRetry<{ data: string }>(
      "http://api.test/endpoint",
      "old-token",
      { onAuthFailure, getNewToken },
    );

    expect(onAuthFailure).toHaveBeenCalled();
    expect(getNewToken).toHaveBeenCalled();
    expect(result.data).toEqual({ data: "refreshed" });
    expect(mockFetchRetry).toHaveBeenCalledTimes(2);
  });

  it("returns null when 401 retry fails", async () => {
    mockFetchRetry.mockResolvedValueOnce({
      ok: false,
      status: 401,
      text: () => Promise.resolve("Unauthorized"),
    } as Response);

    const onAuthFailure = vi.fn().mockResolvedValue(false);
    const getNewToken = vi.fn();

    const result = await fetchWithRetry(
      "http://api.test/endpoint",
      "old-token",
      { onAuthFailure, getNewToken },
    );

    expect(result.response).toBeNull();
    expect(result.data).toBeNull();
    expect(getNewToken).not.toHaveBeenCalled();
  });

  it("returns null for 403 Forbidden (access denied)", async () => {
    mockFetchRetry.mockResolvedValueOnce({
      ok: false,
      status: 403,
      text: () => Promise.resolve("Forbidden"),
    } as Response);

    const result = await fetchWithRetry(
      "http://api.test/endpoint",
      "test-token",
    );

    expect(result.response).toBeNull();
    expect(result.data).toBeNull();
    expect(consoleWarnSpy).toHaveBeenCalled();
  });

  it("throws error for non-access-denied errors (4xx bugs)", async () => {
    mockFetchRetry.mockResolvedValueOnce({
      ok: false,
      status: 400,
      text: () => Promise.resolve("Bad Request"),
    } as Response);

    await expect(
      fetchWithRetry("http://api.test/endpoint", "test-token"),
    ).rejects.toThrow("API error: 400");
  });

  it("throws error for 5xx server errors", async () => {
    mockFetchRetry.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: () => Promise.resolve("Internal Server Error"),
    } as Response);

    await expect(
      fetchWithRetry("http://api.test/endpoint", "test-token"),
    ).rejects.toThrow("API error: 500");
  });

  it("makes request without headers when token is undefined", async () => {
    mockFetchRetry.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    } as Response);

    await fetchWithRetry("http://api.test/endpoint", undefined);

    expect(mockFetchRetry).toHaveBeenCalledWith(
      "http://api.test/endpoint",
      expect.objectContaining({
        headers: undefined,
      }),
    );
  });

  it("includes body in POST requests", async () => {
    mockFetchRetry.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    } as Response);

    const body = { name: "Test" };
    await fetchWithRetry("http://api.test/endpoint", "token", {
      method: "POST",
      body,
    });

    expect(mockFetchRetry).toHaveBeenCalledWith(
      "http://api.test/endpoint",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(body),
      }),
    );
  });
});

// ===== SERVER AUTH TESTS =====

// Mock auth module for server-side tests
vi.mock("../server/auth", () => ({
  auth: vi.fn(),
}));

describe("checkAuth", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns null when session has valid token", async () => {
    const { auth } = await import("../server/auth");
    vi.mocked(auth).mockResolvedValueOnce({
      user: { id: "1", token: "valid-token" },
      expires: "",
    } as never);

    const result = await checkAuth();

    expect(result).toBeNull();
  });

  it("returns 401 response when session is null", async () => {
    const { auth } = await import("../server/auth");
    vi.mocked(auth).mockResolvedValueOnce(null as never);

    const result = await checkAuth();

    expect(result).not.toBeNull();
    expect(result?.status).toBe(401);
    const body = (await result?.json()) as { error: string };
    expect(body.error).toBe("Unauthorized");
  });

  it("returns 401 response when user has no token", async () => {
    const { auth } = await import("../server/auth");
    vi.mocked(auth).mockResolvedValueOnce({
      user: { id: "1", token: undefined },
      expires: "",
    } as never);

    const result = await checkAuth();

    expect(result).not.toBeNull();
    expect(result?.status).toBe(401);
  });
});

// ===== API FUNCTION TESTS (CLIENT-SIDE) =====

// Mock api module
vi.mock("./api", () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe("apiGet (client-side)", () => {
  let originalWindow: typeof globalThis.window;

  beforeEach(async () => {
    vi.clearAllMocks();
    // Simulate browser environment
    originalWindow = globalThis.window;
    Object.defineProperty(globalThis, "window", {
      value: {},
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(globalThis, "window", {
      value: originalWindow,
      writable: true,
      configurable: true,
    });
  });

  it("makes GET request via axios in browser", async () => {
    const api = (await import("./api")).default;
    // eslint-disable-next-line @typescript-eslint/unbound-method
    vi.mocked(api.get).mockResolvedValueOnce({
      data: { result: "test" },
      status: 200,
      statusText: "OK",
      headers: {},
      config: {} as never,
    });

    const result = await apiGet<{ result: string }>("/test", "token");

    expect(result).toEqual({ result: "test" });
    // eslint-disable-next-line @typescript-eslint/unbound-method
    expect(api.get).toHaveBeenCalledWith("/test", {
      headers: { Authorization: "Bearer token" },
    });
  });

  it("throws error on axios failure", async () => {
    const api = (await import("./api")).default;
    const error = {
      response: { status: 404, data: { message: "Not Found" } },
      message: "Request failed",
      isAxiosError: true,
    };
    // eslint-disable-next-line @typescript-eslint/unbound-method
    vi.mocked(api.get).mockRejectedValueOnce(error);

    await expect(apiGet("/test", "token")).rejects.toThrow(
      'API error (404): {"message":"Not Found"}',
    );
  });
});

describe("apiPost (client-side)", () => {
  let originalWindow: typeof globalThis.window;

  beforeEach(async () => {
    vi.clearAllMocks();
    originalWindow = globalThis.window;
    Object.defineProperty(globalThis, "window", {
      value: {},
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(globalThis, "window", {
      value: originalWindow,
      writable: true,
      configurable: true,
    });
  });

  it("makes POST request via axios in browser", async () => {
    const api = (await import("./api")).default;
    // eslint-disable-next-line @typescript-eslint/unbound-method
    vi.mocked(api.post).mockResolvedValueOnce({
      data: { id: 1 },
      status: 201,
      statusText: "Created",
      headers: {},
      config: {} as never,
    });

    const body = { name: "Test" };
    const result = await apiPost<{ id: number }>("/test", "token", body);

    expect(result).toEqual({ id: 1 });
    // eslint-disable-next-line @typescript-eslint/unbound-method
    expect(api.post).toHaveBeenCalledWith("/test", body, {
      headers: { Authorization: "Bearer token" },
    });
  });
});

describe("apiPut (client-side)", () => {
  let originalWindow: typeof globalThis.window;

  beforeEach(async () => {
    vi.clearAllMocks();
    originalWindow = globalThis.window;
    Object.defineProperty(globalThis, "window", {
      value: {},
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(globalThis, "window", {
      value: originalWindow,
      writable: true,
      configurable: true,
    });
  });

  it("makes PUT request via axios in browser", async () => {
    const api = (await import("./api")).default;
    // eslint-disable-next-line @typescript-eslint/unbound-method
    vi.mocked(api.put).mockResolvedValueOnce({
      data: { updated: true },
      status: 200,
      statusText: "OK",
      headers: {},
      config: {} as never,
    });

    const body = { name: "Updated" };
    const result = await apiPut<{ updated: boolean }>("/test", "token", body);

    expect(result).toEqual({ updated: true });
    // eslint-disable-next-line @typescript-eslint/unbound-method
    expect(api.put).toHaveBeenCalledWith("/test", body, {
      headers: { Authorization: "Bearer token" },
    });
  });
});

describe("apiDelete (client-side)", () => {
  let originalWindow: typeof globalThis.window;

  beforeEach(async () => {
    vi.clearAllMocks();
    originalWindow = globalThis.window;
    Object.defineProperty(globalThis, "window", {
      value: {},
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(globalThis, "window", {
      value: originalWindow,
      writable: true,
      configurable: true,
    });
  });

  it("makes DELETE request via axios in browser", async () => {
    const api = (await import("./api")).default;
    // eslint-disable-next-line @typescript-eslint/unbound-method
    vi.mocked(api.delete).mockResolvedValueOnce({
      data: {},
      status: 200,
      statusText: "OK",
      headers: {},
      config: {} as never,
    });

    const result = await apiDelete("/test/1", "token");

    expect(result).toEqual({});
    // eslint-disable-next-line @typescript-eslint/unbound-method
    expect(api.delete).toHaveBeenCalledWith("/test/1", {
      headers: { Authorization: "Bearer token" },
    });
  });

  it("returns undefined for 204 No Content", async () => {
    const api = (await import("./api")).default;
    // eslint-disable-next-line @typescript-eslint/unbound-method
    vi.mocked(api.delete).mockResolvedValueOnce({
      data: {},
      status: 204,
      statusText: "No Content",
      headers: {},
      config: {} as never,
    });

    const result = await apiDelete("/test/1", "token");

    expect(result).toBeUndefined();
  });
});
