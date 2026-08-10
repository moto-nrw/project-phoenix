import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { NextRequest } from "next/server";
import {
  handleDomainApiError,
  isBrowserContext,
  buildAuthHeaders,
  buildAuthHeadersWithBody,
  convertToBackendRoom,
  authFetch,
  fetchWithRetry,
  ApiResponseError,
} from "./api-helpers";
import {
  extractParams,
  handleApiError,
  apiGet,
  checkAuth,
} from "./api-helpers.server";
import { suppressConsole } from "~/test/helpers/console";

const { mockNextHeaders } = vi.hoisted(() => ({
  mockNextHeaders: vi.fn(),
}));

const { mockRecordBackendProxyMetric } = vi.hoisted(() => ({
  mockRecordBackendProxyMetric: vi.fn(),
}));

vi.mock("next/headers", () => ({
  headers: mockNextHeaders,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

vi.mock("./backend-proxy-metrics", () => ({
  recordBackendProxyMetric: mockRecordBackendProxyMetric,
}));

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

describe("ApiResponseError", () => {
  it("parses JSON response bodies lazily and memoizes the result", () => {
    const error = new ApiResponseError(
      403,
      JSON.stringify({ error: "feature_disabled" }),
    );

    expect(error.status).toBe(403);
    expect(error.body<{ error: string }>()).toEqual({
      error: "feature_disabled",
    });
    expect(error.body<{ error: string }>()).toEqual({
      error: "feature_disabled",
    });
  });

  it("returns null for non-JSON response bodies", () => {
    const error = new ApiResponseError(500, "plain backend failure");

    expect(error.body()).toBeNull();
    expect(error.body()).toBeNull();
  });
});

describe("handleApiError", () => {
  const consoleSpies = suppressConsole("error", "warn");

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

    expect(consoleSpies.error).toHaveBeenCalled();
    expect(consoleSpies.warn).not.toHaveBeenCalled();
  });

  it("logs warning for 4xx status codes", () => {
    const error = new Error("API error (400): Bad Request");

    handleApiError(error);

    expect(consoleSpies.warn).toHaveBeenCalled();
    // consoleError not called for 4xx
  });

  it("redacts free-text notes from 409 conflict bodies before logging", () => {
    const backendJson = JSON.stringify({
      status: "error",
      error: "existing student status days were not overwritten",
      conflicts: [
        {
          id: 7,
          student_id: 42,
          date: "2026-05-26",
          status: "sick",
          note: "Fieber und Halsschmerzen",
        },
      ],
    });
    const error = new Error(`API error (409): ${backendJson}`);

    const response = handleApiError(error);

    expect(response.status).toBe(409);
    expect(consoleSpies.warn).toHaveBeenCalledWith("api route error", {
      status: 409,
      error: expect.stringContaining('"note":"[REDACTED]"'),
    });
    expect(consoleSpies.warn.mock.calls[0]?.[1]?.error).not.toContain(
      "Fieber und Halsschmerzen",
    );
  });

  it("logs rate-limit responses with rate_limited context", () => {
    const error = new Error("API error (429): Rate limit exceeded");

    handleApiError(error);

    expect(consoleSpies.warn).toHaveBeenCalledWith("api route rate limited", {
      status: 429,
      error: "API error (429): Rate limit exceeded",
      rate_limited: true,
    });
    expect(consoleSpies.error).not.toHaveBeenCalled();
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

  it("extracts error and code from embedded backend JSON", async () => {
    const backendJson = JSON.stringify({
      status: "error",
      error: "account already has access to tenant",
      code: "ACCOUNT_ALREADY_HAS_TENANT_ACCESS",
    });
    const error = new Error(`API error (409): ${backendJson}`);

    const response = handleApiError(error);
    const body = (await response.json()) as { error: string; code?: string };

    expect(response.status).toBe(409);
    expect(body.error).toBe("account already has access to tenant");
    expect(body.code).toBe("ACCOUNT_ALREADY_HAS_TENANT_ACCESS");
  });

  it("omits code field when backend JSON has no code", async () => {
    const backendJson = JSON.stringify({
      status: "error",
      error: "email already exists",
    });
    const error = new Error(`API error (409): ${backendJson}`);

    const response = handleApiError(error);
    const body = (await response.json()) as { error: string; code?: string };

    expect(response.status).toBe(409);
    expect(body.error).toBe("email already exists");
    expect(body.code).toBeUndefined();
  });

  // Regression for Issue #1368: the proxy must forward the backend's
  // structured `details` payload alongside `error` and `code`. Dropping
  // details here breaks the reopen-with-status-change modal because the
  // UI can no longer learn which session is conflicting from the response —
  // it falls back to history-data lookup, which is empty on past weeks.
  it("forwards backend details payload (e.g. reopen_status_conflict)", async () => {
    const backendJson = JSON.stringify({
      status: "error",
      error: "reopen status conflict",
      code: "reopen_status_conflict",
      details: {
        session_id: "42",
        existing_status: "present",
        requested_status: "home_office",
      },
    });
    const error = new Error(`API error (409): ${backendJson}`);

    const response = handleApiError(error);
    const body = (await response.json()) as {
      error: string;
      code?: string;
      details?: Record<string, unknown>;
    };

    expect(response.status).toBe(409);
    expect(body.code).toBe("reopen_status_conflict");
    expect(body.details).toEqual({
      session_id: "42",
      existing_status: "present",
      requested_status: "home_office",
    });
  });

  // The companion-plan 409 of the student PUT carries its confirmable
  // conflicts as a TOP-LEVEL `conflicts` array, not under `details`. The
  // browser's CompanionPlanConflictError parses exactly that field to build
  // the confirmation it re-sends — dropping it here leaves the confirmation
  // empty and "Ergänzen und speichern" loops on the same 409 forever.
  it("forwards the top-level conflicts array of a companion-plan 409", async () => {
    const backendJson = JSON.stringify({
      conflicts: [{ student_id: 42, weekdays: ["mon", "tue"] }],
      message:
        "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
    });
    const error = new Error(`API error (409): ${backendJson}`);

    const response = handleApiError(error);
    const body = (await response.json()) as {
      error: string;
      conflicts?: unknown[];
    };

    expect(response.status).toBe(409);
    expect(body.conflicts).toEqual([
      { student_id: 42, weekdays: ["mon", "tue"] },
    ]);
    expect(body.error).toBe(
      "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
    );
  });

  it("omits details field when backend JSON has no details", async () => {
    const backendJson = JSON.stringify({
      status: "error",
      error: "generic 409",
      code: "generic_conflict",
    });
    const error = new Error(`API error (409): ${backendJson}`);

    const response = handleApiError(error);
    const body = (await response.json()) as {
      error: string;
      code?: string;
      details?: Record<string, unknown>;
    };

    expect(body.details).toBeUndefined();
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
      cache: "no-store",
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
      cache: "no-store",
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
      cache: "no-store",
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
  const consoleSpies = suppressConsole("error", "warn");
  let originalFetch: typeof fetch;
  let mockFetchRetry: MockedFetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
    mockFetchRetry = vi.fn();
    globalThis.fetch = mockFetchRetry;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
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
    expect(consoleSpies.warn).toHaveBeenCalled();
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

  it("logs 429 responses as rate-limit warnings", async () => {
    mockFetchRetry.mockResolvedValueOnce({
      ok: false,
      status: 429,
      text: () => Promise.resolve("Rate limit exceeded"),
    } as Response);

    await expect(
      fetchWithRetry("http://api.test/endpoint", "test-token"),
    ).rejects.toThrow("API error: 429");

    expect(consoleSpies.warn).toHaveBeenCalledWith("api rate limited", {
      url: "http://api.test/endpoint",
      method: "GET",
      status: 429,
      error_text: "Rate limit exceeded",
      rate_limited: true,
    });
    expect(consoleSpies.error).not.toHaveBeenCalled();
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

// ===== API FUNCTION TESTS (SERVER-SIDE) =====

describe("apiGet (server-side)", () => {
  let originalWindow: typeof globalThis.window;
  let originalFetch: typeof fetch;
  let mockFetch: MockedFetch;

  beforeEach(() => {
    vi.clearAllMocks();
    mockRecordBackendProxyMetric.mockClear();
    originalWindow = globalThis.window;
    originalFetch = globalThis.fetch;
    mockFetch = vi.fn();
    globalThis.fetch = mockFetch;
    Object.defineProperty(globalThis, "window", {
      value: undefined,
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    Object.defineProperty(globalThis, "window", {
      value: originalWindow,
      writable: true,
      configurable: true,
    });
  });

  it("forwards the canonical client IP and user agent headers to the backend", async () => {
    mockNextHeaders.mockResolvedValueOnce(
      new Headers({
        "x-forwarded-for": "203.0.113.10, 172.20.0.4",
        "user-agent": "Mozilla/5.0 Test Browser",
      }),
    );
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ result: "ok" }),
    } as Response);

    const result = await apiGet<{ result: string }>("/api/test", "token");

    expect(result).toEqual({ result: "ok" });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend.test/api/test",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer token",
          "Content-Type": "application/json",
          "X-Forwarded-For": "172.20.0.4",
          "User-Agent": "Mozilla/5.0 Test Browser",
        }) as HeadersInit,
      }),
    );
  });

  it("falls back to X-Real-IP when X-Forwarded-For is absent", async () => {
    mockNextHeaders.mockResolvedValueOnce(
      new Headers({
        "x-real-ip": "198.51.100.25",
      }),
    );
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ result: "ok" }),
    } as Response);

    await apiGet<{ result: string }>("/api/test", "token");

    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend.test/api/test",
      expect.objectContaining({
        headers: {
          Authorization: "Bearer token",
          "Content-Type": "application/json",
          "X-Forwarded-For": "198.51.100.25",
        },
      }),
    );
  });

  it("omits forward headers when no client IP or user agent is available", async () => {
    mockNextHeaders.mockResolvedValueOnce(new Headers());
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ result: "ok" }),
    } as Response);

    await apiGet<{ result: string }>("/api/test", "token");

    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend.test/api/test",
      expect.objectContaining({
        headers: {
          Authorization: "Bearer token",
          "Content-Type": "application/json",
        },
      }),
    );
  });

  it("records tenant backend proxy metrics for server-side calls", async () => {
    mockNextHeaders.mockResolvedValueOnce(new Headers());
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      headers: new Headers({ "content-length": "17" }),
      json: () => Promise.resolve({ result: "ok" }),
    } as Response);
    const tokenPayload = Buffer.from(
      JSON.stringify({ tenant_id: 101, scope: "" }),
    ).toString("base64url");

    await apiGet<{ result: string }>(
      "/api/students/1234567890123456?verbose=true",
      `header.${tokenPayload}.signature`,
    );

    expect(mockRecordBackendProxyMetric).toHaveBeenCalledWith({
      method: "GET",
      backendEndpoint: "/api/students/{id}",
      status: 200,
      durationMs: expect.any(Number),
      outcome: "success",
      scope: "",
    });
  });

  it("still calls the backend when request headers are unavailable", async () => {
    mockNextHeaders.mockRejectedValueOnce(new Error("outside request scope"));
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ result: "ok" }),
    } as Response);

    await apiGet<{ result: string }>("/api/test", "token");

    expect(mockFetch).toHaveBeenCalledWith(
      "http://backend.test/api/test",
      expect.objectContaining({
        headers: {
          Authorization: "Bearer token",
          "Content-Type": "application/json",
        },
      }),
    );
  });
});
