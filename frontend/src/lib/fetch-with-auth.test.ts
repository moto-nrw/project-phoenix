import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { fetchWithAuth } from "./fetch-with-auth";

// Mock auth-api
vi.mock("./auth-api", () => ({
  handleAuthFailure: vi.fn(),
}));

// Mock console.error to suppress error logs in tests
const originalConsoleError = console.error;

describe("fetchWithAuth", () => {
  let mockFetch: ReturnType<typeof vi.fn>;
  let mockWindow: typeof globalThis.window | undefined;
  let mockHandleAuthFailure: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    // Import after mocks are set up
    const authApiModule = await import("./auth-api");
    mockHandleAuthFailure =
      authApiModule.handleAuthFailure as typeof mockHandleAuthFailure;

    vi.clearAllMocks();
    mockFetch = vi.fn();
    globalThis.fetch = mockFetch as unknown as typeof globalThis.fetch;

    // Store original window state
    mockWindow = globalThis.window;

    // Mock console.error to use () => undefined
    console.error = () => undefined;
  });

  afterEach(() => {
    // Restore console.error
    console.error = originalConsoleError;

    // Restore window state
    if (mockWindow === undefined) {
      // @ts-expect-error - restoring original state
      delete globalThis.window;
    } else {
      globalThis.window = mockWindow;
    }
  });

  it("returns response directly for successful request", async () => {
    const mockResponse = new Response("success", { status: 200 });
    mockFetch.mockResolvedValue(mockResponse);

    const result = await fetchWithAuth("/api/test");

    expect(mockFetch).toHaveBeenCalledWith("/api/test", {});
    expect(result).toBe(mockResponse);
    expect(mockHandleAuthFailure).not.toHaveBeenCalled();
  });

  it("returns non-401 error response without retry", async () => {
    const mockResponse = new Response("error", { status: 500 });
    mockFetch.mockResolvedValue(mockResponse);

    const result = await fetchWithAuth("/api/test");

    expect(result.status).toBe(500);
    expect(mockHandleAuthFailure).not.toHaveBeenCalled();
  });

  it("retries request after successful token refresh (client-side)", async () => {
    const mock401Response = new Response("Unauthorized", { status: 401 });
    const mockSuccessResponse = new Response("success", { status: 200 });

    mockFetch
      .mockResolvedValueOnce(mock401Response)
      .mockResolvedValueOnce(mockSuccessResponse);

    mockHandleAuthFailure.mockResolvedValue(true);

    // Ensure we're on client side
    globalThis.window = {} as Window & typeof globalThis;

    const result = await fetchWithAuth("/api/test");

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch).toHaveBeenNthCalledWith(1, "/api/test", {});
    // Second call also passes empty object because retry is extracted before calling fetch
    expect(mockFetch).toHaveBeenNthCalledWith(2, "/api/test", {});
    expect(mockHandleAuthFailure).toHaveBeenCalledTimes(1);
    expect(result).toBe(mockSuccessResponse);
  });

  it("returns 401 response when token refresh fails (client-side)", async () => {
    const mock401Response = new Response("Unauthorized", { status: 401 });

    mockFetch.mockResolvedValue(mock401Response);
    mockHandleAuthFailure.mockResolvedValue(false);

    // Ensure we're on client side
    globalThis.window = {} as Window & typeof globalThis;

    const result = await fetchWithAuth("/api/test");

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockHandleAuthFailure).toHaveBeenCalledTimes(1);
    expect(result.status).toBe(401);
  });

  it("returns 401 response when token refresh throws error (client-side)", async () => {
    const mock401Response = new Response("Unauthorized", { status: 401 });

    mockFetch.mockResolvedValue(mock401Response);
    mockHandleAuthFailure.mockRejectedValue(new Error("Refresh failed"));

    // Ensure we're on client side
    globalThis.window = {} as Window & typeof globalThis;

    const result = await fetchWithAuth("/api/test");

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockHandleAuthFailure).toHaveBeenCalledTimes(1);
    expect(result.status).toBe(401);
  });

  it("does not retry on 401 when retry=false", async () => {
    const mock401Response = new Response("Unauthorized", { status: 401 });
    mockFetch.mockResolvedValue(mock401Response);

    // Ensure we're on client side
    globalThis.window = {} as Window & typeof globalThis;

    const result = await fetchWithAuth("/api/test", { retry: false });

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockHandleAuthFailure).not.toHaveBeenCalled();
    expect(result.status).toBe(401);
  });

  it("does not retry on 401 when on server-side (no window)", async () => {
    const mock401Response = new Response("Unauthorized", { status: 401 });
    mockFetch.mockResolvedValue(mock401Response);

    // Ensure we're on server side
    // @ts-expect-error - intentionally setting to undefined
    globalThis.window = undefined;

    const result = await fetchWithAuth("/api/test");

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockHandleAuthFailure).not.toHaveBeenCalled();
    expect(result.status).toBe(401);
  });

  it("passes through fetch options", async () => {
    const mockResponse = new Response("success", { status: 200 });
    mockFetch.mockResolvedValue(mockResponse);

    const options = {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ test: "data" }),
    };

    await fetchWithAuth("/api/test", options);

    expect(mockFetch).toHaveBeenCalledWith("/api/test", options);
  });
});
