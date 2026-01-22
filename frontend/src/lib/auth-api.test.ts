import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  handleAuthFailure,
  requestPasswordReset,
  confirmPasswordReset,
  type ApiError,
} from "./auth-api";

// Mock auth-client (BetterAuth)
vi.mock("./auth-client", () => ({
  authClient: {
    signOut: vi.fn().mockResolvedValue({}),
  },
}));

// Mock auth-service
vi.mock("./auth-service", () => ({
  authService: {
    resetPassword: vi.fn(),
  },
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
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("handleAuthFailure", () => {
    it("returns false in server context and logs error", async () => {
      const restore = setupServerEnv();
      try {
        const consoleSpy = vi
          .spyOn(console, "error")
          .mockImplementation(() => {});
        const result = await handleAuthFailure();

        expect(result).toBe(false);
        expect(consoleSpy).toHaveBeenCalledWith(
          "Auth failure in server context",
        );
      } finally {
        restore();
      }
    });

    it("signs out and redirects in browser context", async () => {
      const restore = setupBrowserEnv();
      try {
        const consoleSpy = vi
          .spyOn(console, "log")
          .mockImplementation(() => {});
        const { authClient } = await import("./auth-client");

        const result = await handleAuthFailure();

        expect(result).toBe(false);
        expect(authClient.signOut).toHaveBeenCalled();
        expect(consoleSpy).toHaveBeenCalledWith("Session expired, signing out");
        expect(globalThis.window.location.href).toBe("/");
      } finally {
        restore();
      }
    });

    it("handles errors during sign out and still redirects", async () => {
      const restore = setupBrowserEnv();
      try {
        const consoleSpy = vi
          .spyOn(console, "error")
          .mockImplementation(() => {});
        const { authClient } = await import("./auth-client");
        vi.mocked(authClient.signOut).mockRejectedValue(
          new Error("Sign out failed"),
        );

        const result = await handleAuthFailure();

        expect(result).toBe(false);
        expect(consoleSpy).toHaveBeenCalled();
        expect(globalThis.window.location.href).toBe("/");
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

      vi.spyOn(console, "error").mockImplementation(() => {});
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

      vi.spyOn(console, "error").mockImplementation(() => {});

      await expect(requestPasswordReset("test@example.com")).rejects.toThrow(
        "Fehler beim Senden der Passwort-Zurücksetzen-E-Mail",
      );
    });
  });
});
