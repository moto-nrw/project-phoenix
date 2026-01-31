import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  refreshToken,
  handleAuthFailure,
  requestPasswordReset,
  confirmPasswordReset,
  type ApiError,
} from "./auth-api";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  signOut: vi.fn(),
  signIn: vi.fn(),
}));

// Mock auth-service
vi.mock("./auth-service", () => ({
  authService: {
    resetPassword: vi.fn(),
  },
}));

// Mock server-side token refresh module
vi.mock("~/server/auth/token-refresh", () => ({
  refreshSessionTokensOnServer: vi.fn(),
}));

// Helper to setup browser environment
function setupBrowserEnv() {
  const originalWindow = globalThis.window;
  Object.defineProperty(globalThis, "window", {
    value: { location: { href: "" } },
    writable: true,
    configurable: true,
  });
  return () => {
    Object.defineProperty(globalThis, "window", {
      value: originalWindow,
      writable: true,
      configurable: true,
    });
  };
}

// Helper to setup server environment
function setupServerEnv() {
  const originalWindow = globalThis.window;
  Object.defineProperty(globalThis, "window", {
    value: undefined,
    writable: true,
    configurable: true,
  });
  return () => {
    Object.defineProperty(globalThis, "window", {
      value: originalWindow,
      writable: true,
      configurable: true,
    });
  };
}

describe("auth-api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Clear sessionStorage
    if (typeof sessionStorage !== "undefined") {
      sessionStorage.clear();
    }
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("refreshToken", () => {
    it("returns tokens on successful refresh in browser", async () => {
      const restore = setupBrowserEnv();
      try {
        const mockTokens = {
          access_token: "new-access-token",
          refresh_token: "new-refresh-token",
        };

        global.fetch = vi.fn().mockResolvedValue({
          ok: true,
          json: () => Promise.resolve(mockTokens),
        });

        const result = await refreshToken();

        expect(result).toEqual(mockTokens);
        expect(global.fetch).toHaveBeenCalledWith("/api/auth/token", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include",
        });
      } finally {
        restore();
      }
    });

    it("returns null when refresh fails with non-ok response", async () => {
      const restore = setupBrowserEnv();
      try {
        global.fetch = vi.fn().mockResolvedValue({
          ok: false,
          status: 401,
        });

        const consoleSpy = vi
          .spyOn(console, "error")
          .mockImplementation(/* noop */ () => undefined);
        const result = await refreshToken();

        expect(result).toBeNull();
        expect(consoleSpy).toHaveBeenCalledWith("Token refresh failed:", 401);
      } finally {
        restore();
      }
    });

    it("returns null when fetch throws error", async () => {
      const restore = setupBrowserEnv();
      try {
        const networkError = new Error("Network error");
        global.fetch = vi.fn().mockRejectedValue(networkError);

        const consoleSpy = vi
          .spyOn(console, "error")
          .mockImplementation(/* noop */ () => undefined);
        const result = await refreshToken();

        expect(result).toBeNull();
        expect(consoleSpy).toHaveBeenCalledWith(
          "Error refreshing token:",
          networkError,
        );
      } finally {
        restore();
      }
    });

    it("returns null when called from server context", async () => {
      const restore = setupServerEnv();
      try {
        const consoleSpy = vi
          .spyOn(console, "error")
          .mockImplementation(/* noop */ () => undefined);
        const result = await refreshToken();

        expect(result).toBeNull();
        expect(consoleSpy).toHaveBeenCalledWith(
          "Token refresh attempted from server context",
        );
      } finally {
        restore();
      }
    });

    it("deduplicates concurrent refresh requests", async () => {
      const restore = setupBrowserEnv();
      try {
        const mockTokens = {
          access_token: "new-access-token",
          refresh_token: "new-refresh-token",
        };

        // Use a delayed response to ensure both calls happen while first is pending
        let resolvePromise: (value: unknown) => void;
        const fetchPromise = new Promise((resolve) => {
          resolvePromise = resolve;
        });

        global.fetch = vi.fn().mockReturnValue(fetchPromise);

        // Start two refresh requests concurrently
        const promise1 = refreshToken();
        const promise2 = refreshToken();

        // Now resolve the fetch
        resolvePromise!({
          ok: true,
          json: () => Promise.resolve(mockTokens),
        });

        const [result1, result2] = await Promise.all([promise1, promise2]);

        // Both should get the same result
        expect(result1).toEqual(mockTokens);
        expect(result2).toEqual(mockTokens);

        // But fetch should only be called once
        expect(global.fetch).toHaveBeenCalledTimes(1);
      } finally {
        restore();
      }
    });
  });

  describe("handleAuthFailure", () => {
    it("handles server context by calling server-side refresh", async () => {
      const restore = setupServerEnv();
      try {
        const { refreshSessionTokensOnServer } =
          await import("~/server/auth/token-refresh");
        vi.mocked(refreshSessionTokensOnServer).mockResolvedValue({
          accessToken: "new-token",
          refreshToken: "new-refresh",
        });

        const result = await handleAuthFailure();

        expect(result).toBe(true);
        expect(refreshSessionTokensOnServer).toHaveBeenCalled();
      } finally {
        restore();
      }
    });

    it("returns false when server-side refresh fails", async () => {
      const restore = setupServerEnv();
      try {
        const { refreshSessionTokensOnServer } =
          await import("~/server/auth/token-refresh");
        vi.mocked(refreshSessionTokensOnServer).mockResolvedValue(null);

        const result = await handleAuthFailure();

        expect(result).toBe(false);
      } finally {
        restore();
      }
    });

    it("returns false when server-side refresh throws", async () => {
      const restore = setupServerEnv();
      try {
        const { refreshSessionTokensOnServer } =
          await import("~/server/auth/token-refresh");
        vi.mocked(refreshSessionTokensOnServer).mockRejectedValue(
          new Error("Server error"),
        );

        const consoleSpy = vi
          .spyOn(console, "error")
          .mockImplementation(/* noop */ () => undefined);
        const result = await handleAuthFailure();

        expect(result).toBe(false);
        expect(consoleSpy).toHaveBeenCalled();
      } finally {
        restore();
      }
    });

    it("returns true immediately if recently refreshed (within 5 seconds)", async () => {
      const restore = setupBrowserEnv();
      try {
        // Mock sessionStorage using Object.defineProperty for reliable mocking
        const lastRefreshTime = (Date.now() - 2000).toString(); // 2 seconds ago
        const getItemMock = vi.fn().mockImplementation((key: string) => {
          if (key === "lastSuccessfulRefresh") {
            return lastRefreshTime;
          }
          return null;
        });
        Object.defineProperty(globalThis, "sessionStorage", {
          value: { getItem: getItemMock, setItem: vi.fn(), clear: vi.fn() },
          writable: true,
          configurable: true,
        });

        const consoleSpy = vi
          .spyOn(console, "log")
          .mockImplementation(/* noop */ () => undefined);
        const result = await handleAuthFailure();

        expect(result).toBe(true);
        expect(consoleSpy).toHaveBeenCalledWith(
          "Recently refreshed tokens, retrying request...",
        );
      } finally {
        restore();
      }
    });

    it("attempts token refresh when not recently refreshed", async () => {
      const restore = setupBrowserEnv();
      try {
        const mockTokens = {
          access_token: "new-access-token",
          refresh_token: "new-refresh-token",
        };

        // No recent refresh - use a function mock that checks for the specific key
        const getItemMock = vi.fn().mockImplementation((key: string) => {
          if (key === "lastSuccessfulRefresh") {
            return null;
          }
          return null;
        });
        const setItemMock = vi.fn();
        Object.defineProperty(globalThis, "sessionStorage", {
          value: { getItem: getItemMock, setItem: setItemMock, clear: vi.fn() },
          writable: true,
        });

        global.fetch = vi.fn().mockResolvedValue({
          ok: true,
          json: () => Promise.resolve(mockTokens),
        });

        const { signIn } = await import("next-auth/react");
        vi.mocked(signIn).mockResolvedValue({
          ok: true,
          error: undefined,
          status: 200,
          url: "",
          code: undefined,
        });

        const result = await handleAuthFailure();

        expect(result).toBe(true);
        expect(signIn).toHaveBeenCalledWith("credentials", {
          internalRefresh: "true",
          token: "new-access-token",
          refreshToken: "new-refresh-token",
          redirect: false,
        });
      } finally {
        restore();
      }
    });

    it("signs out and redirects when token refresh fails", async () => {
      const restore = setupBrowserEnv();
      try {
        // No recent refresh
        const getItemMock = vi.fn().mockReturnValue(null);
        Object.defineProperty(globalThis, "sessionStorage", {
          value: { getItem: getItemMock, setItem: vi.fn(), clear: vi.fn() },
          writable: true,
        });

        global.fetch = vi.fn().mockResolvedValue({
          ok: false,
          status: 401,
        });

        const { signOut } = await import("next-auth/react");
        vi.mocked(signOut).mockResolvedValue({ url: "/" });

        const consoleSpy = vi
          .spyOn(console, "log")
          .mockImplementation(/* noop */ () => undefined);
        vi.spyOn(console, "error").mockImplementation(
          /* noop */ () => undefined,
        );

        const result = await handleAuthFailure();

        expect(result).toBe(false);
        expect(signOut).toHaveBeenCalledWith({ redirect: false });
        expect(consoleSpy).toHaveBeenCalledWith(
          "Token refresh failed, signing out",
        );
        expect(globalThis.window.location.href).toBe("/");
      } finally {
        restore();
      }
    });

    it("handles errors during auth failure handling", async () => {
      const restore = setupBrowserEnv();
      try {
        // Mock sessionStorage to throw error
        Object.defineProperty(globalThis, "sessionStorage", {
          value: {
            getItem: vi.fn().mockImplementation(() => {
              throw new Error("Storage error");
            }),
            setItem: vi.fn(),
            clear: vi.fn(),
          },
          writable: true,
        });

        const { signOut } = await import("next-auth/react");
        vi.mocked(signOut).mockResolvedValue({ url: "/" });

        const consoleSpy = vi
          .spyOn(console, "error")
          .mockImplementation(/* noop */ () => undefined);
        const result = await handleAuthFailure();

        expect(result).toBe(false);
        expect(consoleSpy).toHaveBeenCalled();
        expect(signOut).toHaveBeenCalled();
      } finally {
        restore();
      }
    });

    it("handles session update failure gracefully", async () => {
      const restore = setupBrowserEnv();
      try {
        const mockTokens = {
          access_token: "new-access-token",
          refresh_token: "new-refresh-token",
        };

        // No recent refresh
        Object.defineProperty(globalThis, "sessionStorage", {
          value: {
            getItem: vi.fn().mockReturnValue(null),
            setItem: vi.fn(),
            clear: vi.fn(),
          },
          writable: true,
        });

        global.fetch = vi.fn().mockResolvedValue({
          ok: true,
          json: () => Promise.resolve(mockTokens),
        });

        const { signIn } = await import("next-auth/react");
        vi.mocked(signIn).mockResolvedValue({
          ok: false,
          error: "Session update failed",
          status: 500,
          url: "",
          code: undefined,
        });

        const consoleSpy = vi
          .spyOn(console, "error")
          .mockImplementation(/* noop */ () => undefined);
        const result = await handleAuthFailure();

        // Should still return true to retry, even if session update failed
        expect(result).toBe(true);
        expect(consoleSpy).toHaveBeenCalledWith(
          "Failed to update session with new tokens:",
          "Session update failed",
        );
      } finally {
        restore();
      }
    });
  });

  describe("requestPasswordReset", () => {
    it("returns success message on successful request", async () => {
      const mockResponse = { message: "Password reset email sent" };

      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await requestPasswordReset("test@example.com");

      expect(result).toEqual(mockResponse);
      expect(global.fetch).toHaveBeenCalledWith("/api/auth/password-reset", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ email: "test@example.com" }),
      });
    });

    it("throws ApiError with JSON error message", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 429,
        headers: new Headers({
          "Content-Type": "application/json",
          "Retry-After": "60",
        }),
        json: () => Promise.resolve({ error: "Rate limit exceeded" }),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      await expect(requestPasswordReset("test@example.com")).rejects.toThrow(
        "Rate limit exceeded",
      );
    });

    it("throws ApiError with text error message", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        headers: new Headers({
          "Content-Type": "text/plain",
        }),
        text: () => Promise.resolve("Internal server error"),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      await expect(requestPasswordReset("test@example.com")).rejects.toThrow(
        "Internal server error",
      );
    });

    it("throws ApiError with retryAfterSeconds from Retry-After header (numeric)", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 429,
        headers: new Headers({
          "Content-Type": "application/json",
          "Retry-After": "120",
        }),
        json: () => Promise.resolve({ error: "Too many requests" }),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      try {
        await requestPasswordReset("test@example.com");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as ApiError;
        expect(apiError.status).toBe(429);
        expect(apiError.retryAfterSeconds).toBe(120);
      }
    });

    it("throws ApiError with retryAfterSeconds from Retry-After header (date)", async () => {
      const futureDate = new Date(Date.now() + 30000); // 30 seconds from now

      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 429,
        headers: new Headers({
          "Content-Type": "application/json",
          "Retry-After": futureDate.toUTCString(),
        }),
        json: () => Promise.resolve({ error: "Too many requests" }),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      try {
        await requestPasswordReset("test@example.com");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as ApiError;
        expect(apiError.status).toBe(429);
        expect(apiError.retryAfterSeconds).toBeGreaterThan(0);
        expect(apiError.retryAfterSeconds).toBeLessThanOrEqual(31);
      }
    });

    it("handles response parse failure gracefully", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        headers: new Headers({}),
        json: () => Promise.reject(new Error("JSON parse error")),
        text: () => Promise.reject(new Error("Text parse error")),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "warn").mockImplementation(() => {});

      await expect(requestPasswordReset("test@example.com")).rejects.toThrow(
        "Fehler beim Senden der Passwort-Zurücksetzen-E-Mail",
      );
    });
  });

  describe("confirmPasswordReset", () => {
    it("calls authService.resetPassword with correct params", async () => {
      const { authService } = await import("./auth-service");
      vi.mocked(authService.resetPassword).mockResolvedValue({
        message: "Password reset successful",
      });

      const result = await confirmPasswordReset(
        "reset-token",
        "newPassword123",
        "newPassword123",
      );

      expect(result).toEqual({ message: "Password reset successful" });
      expect(authService.resetPassword).toHaveBeenCalledWith({
        token: "reset-token",
        newPassword: "newPassword123",
        confirmPassword: "newPassword123",
      });
    });

    it("throws error when authService.resetPassword fails", async () => {
      const { authService } = await import("./auth-service");
      vi.mocked(authService.resetPassword).mockRejectedValue(
        new Error("Invalid token"),
      );

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      await expect(
        confirmPasswordReset(
          "invalid-token",
          "newPassword123",
          "newPassword123",
        ),
      ).rejects.toThrow("Invalid token");
    });
  });

  describe("parseRetryAfter (via buildApiError)", () => {
    it("handles null Retry-After header", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 429,
        headers: new Headers({
          "Content-Type": "application/json",
        }),
        json: () => Promise.resolve({ error: "Too many requests" }),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      try {
        await requestPasswordReset("test@example.com");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as ApiError;
        expect(apiError.retryAfterSeconds).toBeUndefined();
      }
    });

    it("handles negative numeric Retry-After (clamps to 0)", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 429,
        headers: new Headers({
          "Content-Type": "application/json",
          "Retry-After": "-10",
        }),
        json: () => Promise.resolve({ error: "Too many requests" }),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      try {
        await requestPasswordReset("test@example.com");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as ApiError;
        expect(apiError.retryAfterSeconds).toBe(0);
      }
    });

    it("handles past date Retry-After (returns 0)", async () => {
      const pastDate = new Date(Date.now() - 30000); // 30 seconds in past

      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 429,
        headers: new Headers({
          "Content-Type": "application/json",
          "Retry-After": pastDate.toUTCString(),
        }),
        json: () => Promise.resolve({ error: "Too many requests" }),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      try {
        await requestPasswordReset("test@example.com");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as ApiError;
        expect(apiError.retryAfterSeconds).toBe(0);
      }
    });

    it("handles invalid Retry-After value (not number or date)", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 429,
        headers: new Headers({
          "Content-Type": "application/json",
          "Retry-After": "invalid-value",
        }),
        json: () => Promise.resolve({ error: "Too many requests" }),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      try {
        await requestPasswordReset("test@example.com");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as ApiError;
        expect(apiError.retryAfterSeconds).toBeUndefined();
      }
    });
  });

  describe("buildApiError (via requestPasswordReset)", () => {
    it("extracts error from JSON response with error field", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
        headers: new Headers({
          "Content-Type": "application/json",
        }),
        json: () => Promise.resolve({ error: "Validation failed" }),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      await expect(requestPasswordReset("test@example.com")).rejects.toThrow(
        "Validation failed",
      );
    });

    it("extracts message from JSON response with message field", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 400,
        headers: new Headers({
          "Content-Type": "application/json",
        }),
        json: () => Promise.resolve({ message: "Email not found" }),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      await expect(requestPasswordReset("test@example.com")).rejects.toThrow(
        "Email not found",
      );
    });

    it("uses fallback message for empty JSON response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        headers: new Headers({
          "Content-Type": "application/json",
        }),
        json: () => Promise.resolve({}),
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      await expect(requestPasswordReset("test@example.com")).rejects.toThrow(
        "Fehler beim Senden der Passwort-Zurücksetzen-E-Mail",
      );
    });

    it("uses fallback for empty text response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        headers: new Headers({
          "Content-Type": "text/plain",
        }),
        text: () => Promise.resolve("   "), // whitespace only
      });

      // eslint-disable-next-line @typescript-eslint/no-empty-function
      vi.spyOn(console, "error").mockImplementation(() => {});

      await expect(requestPasswordReset("test@example.com")).rejects.toThrow(
        "Fehler beim Senden der Passwort-Zurücksetzen-E-Mail",
      );
    });
  });
});
