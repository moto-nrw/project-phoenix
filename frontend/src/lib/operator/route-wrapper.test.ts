import { describe, it, expect, vi, beforeEach } from "vitest";

const {
  mockGetOperatorToken,
  mockGetOperatorRefreshToken,
  mockSetOperatorTokens,
  mockFetch,
  mockGetServerApiUrl,
} = vi.hoisted(() => ({
  mockGetOperatorToken: vi.fn(),
  mockGetOperatorRefreshToken: vi.fn(),
  mockSetOperatorTokens: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/lib/operator/cookies", () => ({
  getOperatorToken: mockGetOperatorToken,
  getOperatorRefreshToken: mockGetOperatorRefreshToken,
  setOperatorTokens: mockSetOperatorTokens,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import {
  operatorApiGet,
  operatorApiPost,
  _resetRefreshState,
} from "./route-wrapper";

describe("operatorServerFetch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    _resetRefreshState();
  });

  it("returns unwrapped data from envelope response", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { id: 1, name: "Test" },
        message: "OK",
      }),
    });

    const result = await operatorApiGet("/api/test", "my-token");

    expect(result).toEqual({ id: 1, name: "Test" });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/test",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer my-token",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("returns undefined for 204 response", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const result = await operatorApiGet("/api/test", "my-token");
    expect(result).toBeUndefined();
  });

  it("returns raw JSON for non-envelope response", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ custom: "response" }),
    });

    const result = await operatorApiGet("/api/test", "my-token");
    expect(result).toEqual({ custom: "response" });
  });

  it("retries with refreshed token on 401", async () => {
    // First call returns 401
    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Unauthorized",
      })
      // Refresh call succeeds
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: {
            access_token: "new-access",
            refresh_token: "new-refresh",
          },
        }),
      })
      // Retry call succeeds
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          status: "success",
          data: { retried: true },
        }),
      });

    mockGetOperatorRefreshToken.mockResolvedValue("old-refresh-token");

    const result = await operatorApiGet("/api/test", "old-token");

    expect(result).toEqual({ retried: true });
    expect(mockSetOperatorTokens).toHaveBeenCalledWith(
      "new-access",
      "new-refresh",
    );
  });

  it("throws original error when 401 refresh fails", async () => {
    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Unauthorized",
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
      });

    mockGetOperatorRefreshToken.mockResolvedValue("old-refresh-token");

    await expect(operatorApiGet("/api/test", "old-token")).rejects.toThrow(
      "API error (401)",
    );
  });

  it("throws original error when no refresh token available", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      text: async () => "Unauthorized",
    });

    mockGetOperatorRefreshToken.mockResolvedValue(undefined);

    await expect(operatorApiGet("/api/test", "old-token")).rejects.toThrow(
      "API error (401)",
    );
  });

  it("throws retry error when retry also fails", async () => {
    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Unauthorized",
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: {
            access_token: "new-access",
            refresh_token: "new-refresh",
          },
        }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 403,
        text: async () => "Forbidden",
      });

    mockGetOperatorRefreshToken.mockResolvedValue("old-refresh-token");

    await expect(operatorApiGet("/api/test", "old-token")).rejects.toThrow(
      "API error (403)",
    );
  });

  it("throws without refresh attempt on non-401 error", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => "Internal Server Error",
    });

    await expect(operatorApiGet("/api/test", "my-token")).rejects.toThrow(
      "API error (500)",
    );

    // Only one fetch call (no refresh attempt)
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("logs warning and returns null when refresh throws exception", async () => {
    const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {
      // noop
    });

    // First call returns 401
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      text: async () => "Unauthorized",
    });

    // Refresh token exists but fetch throws
    mockGetOperatorRefreshToken.mockResolvedValue("some-refresh-token");

    // The refresh fetch (second call) throws
    mockFetch.mockRejectedValueOnce(new Error("Network down"));

    await expect(operatorApiGet("/api/test", "old-token")).rejects.toThrow(
      "API error (401)",
    );

    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "operator_transparent_refresh_failed",
      expect.objectContaining({ error: "Network down" }),
    );

    consoleWarnSpy.mockRestore();
  });

  it("sends POST body correctly", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { created: true },
      }),
    });

    const result = await operatorApiPost("/api/test", "my-token", {
      name: "test",
    });

    expect(result).toEqual({ created: true });
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/test",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ name: "test" }),
      }),
    );
  });

  it("deduplicates concurrent refresh calls (singleflight)", async () => {
    const callsByUrl = new Map<string, number>();

    mockFetch.mockImplementation(async (url: string) => {
      const count = (callsByUrl.get(url) ?? 0) + 1;
      callsByUrl.set(url, count);

      // Refresh endpoint — only one call expected
      if (url.includes("/operator/auth/refresh")) {
        return {
          ok: true,
          json: async () => ({
            data: {
              access_token: "new-access",
              refresh_token: "new-refresh",
            },
          }),
        };
      }

      // API endpoints: first call returns 401, retry succeeds
      if (count === 1) {
        return {
          ok: false,
          status: 401,
          text: async () => "Unauthorized",
        };
      }
      return {
        ok: true,
        status: 200,
        json: async () => ({
          status: "success",
          data: { url, retried: true },
        }),
      };
    });

    mockGetOperatorRefreshToken.mockResolvedValue("old-refresh-token");

    const [result1, result2] = await Promise.all([
      operatorApiGet<{ url: string; retried: boolean }>(
        "/api/test1",
        "old-token",
      ),
      operatorApiGet<{ url: string; retried: boolean }>(
        "/api/test2",
        "old-token",
      ),
    ]);

    expect(result1.retried).toBe(true);
    expect(result2.retried).toBe(true);

    // Refresh endpoint should have been called only once
    expect(callsByUrl.get("http://localhost:8080/operator/auth/refresh")).toBe(
      1,
    );
  });

  it("queued requests are rejected when refresh fails", async () => {
    const callsByUrl = new Map<string, number>();

    mockFetch.mockImplementation(async (url: string) => {
      const count = (callsByUrl.get(url) ?? 0) + 1;
      callsByUrl.set(url, count);

      // Refresh endpoint fails
      if (url.includes("/operator/auth/refresh")) {
        return { ok: false, status: 401 };
      }

      // API endpoints: always return 401
      return {
        ok: false,
        status: 401,
        text: async () => "Unauthorized",
      };
    });

    mockGetOperatorRefreshToken.mockResolvedValue("old-refresh-token");

    const results = await Promise.allSettled([
      operatorApiGet("/api/test1", "old-token"),
      operatorApiGet("/api/test2", "old-token"),
    ]);

    expect(results[0]?.status).toBe("rejected");
    expect(results[1]?.status).toBe("rejected");
  });
});
