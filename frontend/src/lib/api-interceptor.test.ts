/**
 * Tests for the response interceptor's token refresh queue behavior.
 *
 * Strategy: mock axios.create to capture the registered interceptor handlers,
 * then invoke the error handler directly with crafted 401 errors.
 */
import { describe, it, expect, vi, beforeEach, type Mock } from "vitest";
import type {
  AxiosError,
  AxiosRequestConfig,
  AxiosRequestHeaders,
  AxiosResponse,
} from "axios";

// --- Mocks ---

// Capture the response interceptor handlers registered by api.ts
let requestInterceptorFulfilled: (
  config: AxiosRequestConfig,
) => AxiosRequestConfig | Promise<AxiosRequestConfig>;
let requestInterceptorRejected: (error: Error) => Promise<never>;
let responseInterceptorFulfilled: (
  response: AxiosResponse,
) => AxiosResponse | Promise<AxiosResponse>;
let responseInterceptorRejected: (error: AxiosError) => Promise<AxiosResponse>;

// Controllable mock for the axios instance returned by axios.create
const mockAxiosInstance = vi.fn() as Mock & {
  interceptors: {
    request: { use: Mock };
    response: { use: Mock };
  };
  defaults: { headers: { common: Record<string, string> } };
  get: Mock;
  post: Mock;
  put: Mock;
  delete: Mock;
};
mockAxiosInstance.interceptors = {
  request: {
    use: vi.fn(
      (
        fulfilled: typeof requestInterceptorFulfilled,
        rejected: typeof requestInterceptorRejected,
      ) => {
        requestInterceptorFulfilled = fulfilled;
        requestInterceptorRejected = rejected;
      },
    ),
  },
  response: {
    use: vi.fn(
      (
        fulfilled: typeof responseInterceptorFulfilled,
        rejected: typeof responseInterceptorRejected,
      ) => {
        responseInterceptorFulfilled = fulfilled;
        responseInterceptorRejected = rejected;
      },
    ),
  },
};
mockAxiosInstance.defaults = { headers: { common: {} } };
mockAxiosInstance.get = vi.fn();
mockAxiosInstance.post = vi.fn();
mockAxiosInstance.put = vi.fn();
mockAxiosInstance.delete = vi.fn();

vi.mock("axios", () => ({
  create: vi.fn(() => mockAxiosInstance),
  default: {
    create: vi.fn(() => mockAxiosInstance),
  },
}));

const mockGetSession = vi.fn();
vi.mock("next-auth/react", () => ({
  getSession: (...args: unknown[]) => mockGetSession(...args) as unknown,
}));

const mockHandleAuthFailure = vi.fn();
vi.mock("./auth-failure", () => ({
  handleAuthFailure: (...args: unknown[]) =>
    mockHandleAuthFailure(...args) as unknown,
}));

vi.mock("./api-helpers", () => ({
  fetchWithRetry: vi.fn(),
  convertToBackendRoom: vi.fn(<T>(data: T): T => data),
}));

vi.mock("./student-helpers", () => ({
  mapSingleStudentResponse: vi.fn(<T>(data: T): T => data),
  mapStudentsResponse: vi.fn(<T>(data: T): T => data),
  mapStudentDetailResponse: vi.fn(<T>(data: T): T => data),
  prepareStudentForBackend: vi.fn(<T>(data: T): T => data),
}));

vi.mock("./group-helpers", () => ({
  mapSingleGroupResponse: vi.fn(<T>(data: T): T => data),
  mapGroupResponse: vi.fn(<T>(data: T): T => data),
  prepareGroupForBackend: vi.fn(<T>(data: T): T => data),
  mapGroupsResponse: vi.fn(<T>(data: T): T => data),
}));

vi.mock("./room-helpers", () => ({
  mapSingleRoomResponse: vi.fn(<T>(data: T): T => data),
  prepareRoomForBackend: vi.fn(<T>(data: T): T => data),
  mapRoomsResponse: vi.fn(<T>(data: T): T => data),
  mapRoomResponse: vi.fn(<T>(data: T): T => data),
}));

// --- Helpers ---

function make401Error(
  config: AxiosRequestConfig & { _retry?: boolean; _retryCount?: number } = {},
): AxiosError {
  const err = new Error("Request failed with status code 401") as AxiosError;
  err.response = {
    status: 401,
    statusText: "Unauthorized",
    data: {},
    headers: {},
    config: config as AxiosRequestConfig & {
      headers: AxiosRequestHeaders;
    },
  };
  err.config = config as AxiosRequestConfig & {
    headers: AxiosRequestHeaders;
  };
  err.isAxiosError = true;
  err.toJSON = () => ({});
  return err;
}

function make500Error(config: AxiosRequestConfig = {}): AxiosError {
  const err = new Error("Request failed with status code 500") as AxiosError;
  err.response = {
    status: 500,
    statusText: "Internal Server Error",
    data: {},
    headers: {},
    config: config as AxiosRequestConfig & {
      headers: AxiosRequestHeaders;
    },
  };
  err.config = config as AxiosRequestConfig & {
    headers: AxiosRequestHeaders;
  };
  err.isAxiosError = true;
  err.toJSON = () => ({});
  return err;
}

function make429Error(config: AxiosRequestConfig = {}): AxiosError {
  const err = new Error("Request failed with status code 429") as AxiosError;
  err.response = {
    status: 429,
    statusText: "Too Many Requests",
    data: {},
    headers: { "retry-after": "17" },
    config: config as AxiosRequestConfig & {
      headers: AxiosRequestHeaders;
    },
  };
  err.config = config as AxiosRequestConfig & {
    headers: AxiosRequestHeaders;
  };
  err.isAxiosError = true;
  err.toJSON = () => ({});
  return err;
}

function setupBrowserEnv() {
  const original = globalThis.window;
  Object.defineProperty(globalThis, "window", {
    value: { location: { href: "" } },
    writable: true,
    configurable: true,
  });
  return () => {
    Object.defineProperty(globalThis, "window", {
      value: original,
      writable: true,
      configurable: true,
    });
  };
}

// --- Tests ---

describe("response interceptor token refresh queue", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    // Re-import to trigger interceptor registration with fresh mocks
    vi.resetModules();

    // Re-setup the mock since resetModules clears it
    vi.doMock("axios", () => ({
      create: vi.fn(() => mockAxiosInstance),
      default: {
        create: vi.fn(() => mockAxiosInstance),
      },
    }));

    // Clear the interceptor capture spy to ensure fresh registration
    mockAxiosInstance.interceptors.response.use.mockImplementation(
      (
        fulfilled: typeof responseInterceptorFulfilled,
        rejected: typeof responseInterceptorRejected,
      ) => {
        responseInterceptorFulfilled = fulfilled;
        responseInterceptorRejected = rejected;
      },
    );
    mockAxiosInstance.interceptors.request.use.mockImplementation(
      (
        fulfilled: typeof requestInterceptorFulfilled,
        rejected: typeof requestInterceptorRejected,
      ) => {
        requestInterceptorFulfilled = fulfilled;
        requestInterceptorRejected = rejected;
      },
    );

    await import("./api");
  });

  // The browser session carries no token (#5005): BFF routes read the
  // session cookie, so the interceptor neither looks up the session nor sets
  // an Authorization header.
  it("request interceptor adds no Authorization header in browser context", async () => {
    const restore = setupBrowserEnv();
    try {
      mockGetSession.mockResolvedValue({ user: { id: "1" } });
      const headers = {};

      const result = await requestInterceptorFulfilled({
        method: "get",
        url: "/api/students?search=Mustermann",
        headers,
      });

      expect(mockGetSession).not.toHaveBeenCalled();
      expect(result.headers).toBe(headers);
      expect(result.headers).not.toHaveProperty("Authorization");
    } finally {
      restore();
    }
  });

  it("request interceptor skips session lookup outside browser context", async () => {
    const original = globalThis.window;
    Object.defineProperty(globalThis, "window", {
      value: undefined,
      writable: true,
      configurable: true,
    });
    try {
      const config = { method: "get", url: "/api/test", headers: {} };

      const result = await requestInterceptorFulfilled(config);

      expect(result).toBe(config);
      expect(mockGetSession).not.toHaveBeenCalled();
    } finally {
      Object.defineProperty(globalThis, "window", {
        value: original,
        writable: true,
        configurable: true,
      });
    }
  });

  it("request interceptor rejects setup errors", async () => {
    const error = new Error("request setup failed");

    await expect(requestInterceptorRejected(error)).rejects.toBe(error);
  });

  it("server context rejects 401 retries", async () => {
    const original = globalThis.window;
    Object.defineProperty(globalThis, "window", {
      value: undefined,
      writable: true,
      configurable: true,
    });
    try {
      const headers: Record<string, string> = {};
      const error = make401Error({
        url: "/api/server",
        method: "get",
        headers,
      });

      await expect(responseInterceptorRejected(error)).rejects.toBe(error);
      expect(headers.Authorization).toBeUndefined();
    } finally {
      Object.defineProperty(globalThis, "window", {
        value: original,
        writable: true,
        configurable: true,
      });
    }
  });

  it("queued requests resolve when token refresh succeeds", async () => {
    const restore = setupBrowserEnv();
    try {
      // Simulate first 401 triggers refresh
      mockHandleAuthFailure.mockResolvedValue(true);
      mockGetSession.mockResolvedValue({ user: { id: "1" } });
      mockAxiosInstance.mockResolvedValue({
        status: 200,
        data: { ok: true },
      });

      const error = make401Error({
        url: "/api/test",
        method: "get",
        headers: {},
      });

      const result = await responseInterceptorRejected(error);
      expect(result).toEqual({ status: 200, data: { ok: true } });
    } finally {
      restore();
    }
  });

  it("re-reads the session after a refresh even when the session cache was warm (#2123)", async () => {
    // Regression: the session cache serves a lookup for 10s. After a
    // 401→refresh the retry must follow a fresh session lookup — it persists
    // the rotated cookie — instead of the stale cached session. The retry
    // itself carries no token (#5005).
    const restore = setupBrowserEnv();
    try {
      // 1. Warm the session cache.
      mockGetSession.mockResolvedValue({ user: { id: "1" } });
      const { getCachedSession } = await import("./session-cache");
      await getCachedSession();
      expect(mockGetSession).toHaveBeenCalledTimes(1);

      // 2. The refresh rotates the cookie.
      mockHandleAuthFailure.mockResolvedValue(true);

      // 3. Simulate real axios: the retried request re-runs the request
      //    interceptor, then report which Authorization header went out.
      mockAxiosInstance.mockImplementation(
        async (config: AxiosRequestConfig) => {
          const finalConfig = await requestInterceptorFulfilled(config);
          return {
            status: 200,
            data: {
              sentAuth: (finalConfig.headers as Record<string, string>)
                .Authorization,
            },
          };
        },
      );

      const error = make401Error({
        url: "/api/test",
        method: "get",
        headers: {},
      });
      const result = await responseInterceptorRejected(error);

      expect(mockGetSession).toHaveBeenCalledTimes(2);
      expect(result.data).toEqual({ sentAuth: undefined });
    } finally {
      restore();
    }
  });

  it("queued requests reject when client-side refresh fails", async () => {
    const restore = setupBrowserEnv();
    try {
      // handleAuthFailure returns false → refresh failed
      mockHandleAuthFailure.mockResolvedValue(false);

      // First request triggers refresh (enters isRefreshing=true path)
      const error1 = make401Error({
        url: "/api/first",
        method: "get",
        headers: {},
      });

      // This should throw because refresh fails
      await expect(responseInterceptorRejected(error1)).rejects.toThrow();
    } finally {
      restore();
    }
  });

  it("queued requests reject when client session is missing after refresh", async () => {
    const restore = setupBrowserEnv();
    try {
      // handleAuthFailure succeeds but the session ended meanwhile
      mockHandleAuthFailure.mockResolvedValue(true);
      mockGetSession.mockResolvedValue({
        user: { id: "1" },
        error: "RefreshTokenError",
      });

      const error = make401Error({
        url: "/api/test",
        method: "get",
        headers: {},
      });

      await expect(responseInterceptorRejected(error)).rejects.toThrow();
    } finally {
      restore();
    }
  });

  it("isRefreshing resets to false after failure", async () => {
    const restore = setupBrowserEnv();
    try {
      // First request: refresh fails
      mockHandleAuthFailure.mockResolvedValue(false);

      const error1 = make401Error({
        url: "/api/first",
        method: "get",
        headers: {},
      });

      await expect(responseInterceptorRejected(error1)).rejects.toThrow();

      // Second request: should enter the refresh flow (not queue),
      // proving isRefreshing was reset
      mockHandleAuthFailure.mockResolvedValue(true);
      mockGetSession.mockResolvedValue({
        user: { id: "1" },
      });
      mockAxiosInstance.mockResolvedValue({
        status: 200,
        data: { recovered: true },
      });

      const error2 = make401Error({
        url: "/api/second",
        method: "get",
        headers: {},
      });

      const result = await responseInterceptorRejected(error2);
      expect(result).toEqual({ status: 200, data: { recovered: true } });
      // handleAuthFailure called twice proves both entered the refresh flow
      expect(mockHandleAuthFailure).toHaveBeenCalledTimes(2);
    } finally {
      restore();
    }
  });

  it("multiple concurrent 401s: all queued requests reject on failure", async () => {
    const restore = setupBrowserEnv();
    const consoleDebug = vi
      .spyOn(console, "debug")
      .mockImplementation(() => {});
    try {
      // Create a deferred promise so we can control when handleAuthFailure resolves
      let rejectAuthFailure!: () => void;
      const authFailurePromise = new Promise<boolean>((_, reject) => {
        rejectAuthFailure = () => reject(new Error("Auth refresh exploded"));
      });
      mockHandleAuthFailure.mockReturnValue(authFailurePromise);

      // First 401: enters isRefreshing=true, starts refresh
      const error1 = make401Error({
        url: "/api/request-1",
        method: "get",
        headers: {},
      });
      const promise1 = responseInterceptorRejected(error1);

      // Wait a tick so the first request starts the refresh flow
      await new Promise((r) => setTimeout(r, 0));

      // Second and third 401s: should get queued (isRefreshing is true)
      const error2 = make401Error({
        url: "/api/students?first_name=Erika",
        method: "get",
        headers: {},
      });
      const error3 = make401Error({
        url: "/api/request-3",
        method: "get",
        headers: {},
      });
      const promise2 = responseInterceptorRejected(error2);
      const promise3 = responseInterceptorRejected(error3);

      expect(consoleDebug).toHaveBeenCalledWith(
        "token refresh in progress, queueing request",
        expect.objectContaining({ url: "/api/students" }),
      );

      // Now fail the refresh
      rejectAuthFailure();

      // All three should reject
      await expect(promise1).rejects.toThrow();
      await expect(promise2).rejects.toThrow("Token refresh failed");
      await expect(promise3).rejects.toThrow("Token refresh failed");
    } finally {
      consoleDebug.mockRestore();
      restore();
    }
  });

  it("non-401 errors pass through without touching refresh logic", async () => {
    const error = make500Error({ url: "/api/test", method: "get" });

    await expect(responseInterceptorRejected(error)).rejects.toThrow();
    expect(mockHandleAuthFailure).not.toHaveBeenCalled();
  });

  it("already-retried requests (_retry = true) throw immediately", async () => {
    const error = make401Error({
      url: "/api/test",
      method: "get",
      _retry: true,
    });

    await expect(responseInterceptorRejected(error)).rejects.toThrow();
    expect(mockHandleAuthFailure).not.toHaveBeenCalled();
  });

  it("max retry limit (>3) triggers redirectToLogin and throws", async () => {
    const restore = setupBrowserEnv();
    try {
      const error = make401Error({
        url: "/api/test",
        method: "get",
        headers: {},
        _retryCount: 3,
      });

      await expect(responseInterceptorRejected(error)).rejects.toThrow();
      // redirectToLogin sets window.location.href
      expect(
        (globalThis.window as unknown as { location: { href: string } })
          .location.href,
      ).toBe("/");
      expect(mockHandleAuthFailure).not.toHaveBeenCalled();
    } finally {
      restore();
    }
  });

  it("success path resolves queued requests then finally block is a no-op", async () => {
    const restore = setupBrowserEnv();
    try {
      // Create a deferred promise for handleAuthFailure
      let resolveAuth!: (value: boolean) => void;
      const authPromise = new Promise<boolean>((resolve) => {
        resolveAuth = resolve;
      });
      mockHandleAuthFailure.mockReturnValue(authPromise);
      mockGetSession.mockResolvedValue({
        user: { id: "1" },
      });
      mockAxiosInstance.mockResolvedValue({
        status: 200,
        data: { success: true },
      });

      // First 401: enters refresh flow
      const error1 = make401Error({
        url: "/api/first",
        method: "get",
        headers: {},
      });
      const promise1 = responseInterceptorRejected(error1);

      // Wait a tick
      await new Promise((r) => setTimeout(r, 0));

      // Second 401: gets queued
      const error2 = make401Error({
        url: "/api/second",
        method: "get",
        headers: {},
      });
      const promise2 = responseInterceptorRejected(error2);

      // Resolve auth — success path
      resolveAuth(true);

      // Both should resolve successfully
      const result1 = await promise1;
      const result2 = await promise2;
      expect(result1).toEqual({ status: 200, data: { success: true } });
      expect(result2).toEqual({ status: 200, data: { success: true } });
    } finally {
      restore();
    }
  });

  it("success path fulfillment passes response through", () => {
    const mockResponse = {
      status: 200,
      data: { ok: true },
    } as AxiosResponse;
    const result = responseInterceptorFulfilled(mockResponse);
    expect(result).toBe(mockResponse);
  });

  it("records Axios 429 responses and blocks subsequent browser requests", async () => {
    const error = make429Error({ method: "get", url: "/api/students" });

    await expect(responseInterceptorRejected(error)).rejects.toBe(error);
    await expect(
      requestInterceptorFulfilled({
        method: "get",
        url: "/api/students",
        headers: {},
      }),
    ).rejects.toMatchObject({ status: 429, response: { status: 429 } });
    expect(mockGetSession).not.toHaveBeenCalled();
  });
  it("reports a refused tenant session so TenantGuard switches back (#5130)", async () => {
    const { onTenantSessionMismatch } =
      await import("./tenant-session-mismatch");
    const listener = vi.fn();
    const unsubscribe = onTenantSessionMismatch(listener);
    const conflict = (code: string) => {
      const error = make500Error({
        method: "post",
        url: "/api/time-tracking/check-in",
      });
      error.response!.status = 409;
      error.response!.data = { status: 409, code };
      return error;
    };

    const otherConflict = conflict("care.announcement_stale");
    await expect(responseInterceptorRejected(otherConflict)).rejects.toBe(
      otherConflict,
    );
    expect(listener).not.toHaveBeenCalled();

    const refusal = conflict("identity.tenant_session_mismatch");
    await expect(responseInterceptorRejected(refusal)).rejects.toBe(refusal);
    expect(listener).toHaveBeenCalledTimes(1);
    unsubscribe();
  });
});
