/**
 * Tests for admin-auth.ts
 *
 * Verifies admin authentication helpers for SaaS admin dashboard.
 * Tests session verification and admin email checking.
 */
import {
  describe,
  it,
  expect,
  vi,
  beforeEach,
  afterEach,
  type Mock,
} from "vitest";
import type { NextRequest } from "next/server";

// Mock environment variables before importing the module
const originalEnv = { ...process.env };

describe("admin-auth", () => {
  beforeEach(() => {
    vi.resetModules();
    global.fetch = vi.fn() as Mock;
  });

  afterEach(() => {
    vi.restoreAllMocks();
    process.env = { ...originalEnv };
  });

  describe("constants", () => {
    it("exports INTERNAL_API_KEY with default value when env not set", async () => {
      delete process.env.INTERNAL_API_KEY;
      vi.resetModules();
      const { INTERNAL_API_KEY } = await import("./admin-auth");
      expect(INTERNAL_API_KEY).toBe("dev-internal-key");
    });

    it("exports INTERNAL_API_KEY from environment when set", async () => {
      process.env.INTERNAL_API_KEY = "custom-api-key";
      vi.resetModules();
      const { INTERNAL_API_KEY } = await import("./admin-auth");
      expect(INTERNAL_API_KEY).toBe("custom-api-key");
    });

    it("exports SAAS_ADMIN_EMAILS with default value when env not set", async () => {
      delete process.env.SAAS_ADMIN_EMAILS;
      vi.resetModules();
      const { SAAS_ADMIN_EMAILS } = await import("./admin-auth");
      expect(SAAS_ADMIN_EMAILS).toEqual(["admin@example.com"]);
    });

    it("parses SAAS_ADMIN_EMAILS from comma-separated env variable", async () => {
      process.env.SAAS_ADMIN_EMAILS =
        "admin1@test.com, admin2@test.com, admin3@test.com";
      vi.resetModules();
      const { SAAS_ADMIN_EMAILS } = await import("./admin-auth");
      expect(SAAS_ADMIN_EMAILS).toEqual([
        "admin1@test.com",
        "admin2@test.com",
        "admin3@test.com",
      ]);
    });

    it("trims whitespace and lowercases SAAS_ADMIN_EMAILS", async () => {
      process.env.SAAS_ADMIN_EMAILS = "  Admin@Test.COM  ,  USER@Example.ORG  ";
      vi.resetModules();
      const { SAAS_ADMIN_EMAILS } = await import("./admin-auth");
      expect(SAAS_ADMIN_EMAILS).toEqual(["admin@test.com", "user@example.org"]);
    });
  });

  describe("verifyAdminAccess", () => {
    const createMockRequest = (cookies?: string): NextRequest => {
      const headers = new Map<string, string>();
      if (cookies) {
        headers.set("Cookie", cookies);
      }
      return {
        headers: {
          get: (name: string) => headers.get(name) ?? null,
        },
      } as unknown as NextRequest;
    };

    beforeEach(() => {
      // Reset to default admin emails for consistent tests
      process.env.SAAS_ADMIN_EMAILS = "admin@example.com";
      vi.resetModules();
    });

    it("returns AdminSession when user is authorized admin", async () => {
      const mockSession = {
        user: {
          id: "user-123",
          email: "admin@example.com",
          name: "Admin User",
        },
      };

      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSession),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc123");

      const result = await verifyAdminAccess(request);

      expect(result).toEqual({
        email: "admin@example.com",
        userId: "user-123",
        name: "Admin User",
      });
    });

    it("passes cookies to BetterAuth session endpoint", async () => {
      const mockSession = {
        user: {
          id: "user-123",
          email: "admin@example.com",
          name: "Test",
        },
      };

      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSession),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("better_auth.session_token=xyz789");

      await verifyAdminAccess(request);

      expect(global.fetch).toHaveBeenCalledWith(
        "http://localhost:3001/api/auth/get-session",
        {
          method: "GET",
          headers: {
            "Content-Type": "application/json",
            Cookie: "better_auth.session_token=xyz789",
          },
        },
      );
    });

    it("omits Cookie header when no cookies present", async () => {
      const mockSession = {
        user: {
          id: "user-123",
          email: "admin@example.com",
          name: "Test",
        },
      };

      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSession),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest(); // No cookies

      await verifyAdminAccess(request);

      expect(global.fetch).toHaveBeenCalledWith(
        "http://localhost:3001/api/auth/get-session",
        {
          method: "GET",
          headers: {
            "Content-Type": "application/json",
          },
        },
      );
    });

    it("returns null when session response is not ok", async () => {
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 401,
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=expired");

      const result = await verifyAdminAccess(request);

      expect(result).toBeNull();
    });

    it("returns null when session is null", async () => {
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(null),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      const result = await verifyAdminAccess(request);

      expect(result).toBeNull();
    });

    it("returns null when session.user is undefined", async () => {
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({}),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      const result = await verifyAdminAccess(request);

      expect(result).toBeNull();
    });

    it("returns null when session.user.email is missing", async () => {
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            user: { id: "user-123", name: "Test" },
          }),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      const result = await verifyAdminAccess(request);

      expect(result).toBeNull();
    });

    it("returns null when session.user.id is missing", async () => {
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            user: { email: "admin@example.com", name: "Test" },
          }),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      const result = await verifyAdminAccess(request);

      expect(result).toBeNull();
    });

    it("returns null when user email is not in admin list", async () => {
      const mockSession = {
        user: {
          id: "user-456",
          email: "notadmin@example.com",
          name: "Regular User",
        },
      };

      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSession),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      const result = await verifyAdminAccess(request);

      expect(result).toBeNull();
    });

    it("matches admin email case-insensitively", async () => {
      const mockSession = {
        user: {
          id: "user-123",
          email: "ADMIN@EXAMPLE.COM",
          name: "Admin User",
        },
      };

      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSession),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      const result = await verifyAdminAccess(request);

      expect(result).not.toBeNull();
      expect(result?.email).toBe("admin@example.com");
    });

    it("returns null and logs error when fetch throws", async () => {
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});
      const networkError = new Error("Network failure");

      (global.fetch as Mock).mockRejectedValueOnce(networkError);

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      const result = await verifyAdminAccess(request);

      expect(result).toBeNull();
      expect(consoleSpy).toHaveBeenCalledWith(
        "Failed to verify admin access:",
        networkError,
      );

      consoleSpy.mockRestore();
    });

    it("returns name as null when session.user.name is undefined", async () => {
      const mockSession = {
        user: {
          id: "user-123",
          email: "admin@example.com",
        },
      };

      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSession),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      const result = await verifyAdminAccess(request);

      expect(result).toEqual({
        email: "admin@example.com",
        userId: "user-123",
        name: null,
      });
    });

    it("uses custom BETTERAUTH_INTERNAL_URL when set", async () => {
      process.env.BETTERAUTH_INTERNAL_URL = "https://auth.example.com";
      vi.resetModules();

      const mockSession = {
        user: {
          id: "user-123",
          email: "admin@example.com",
          name: "Admin",
        },
      };

      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSession),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      await verifyAdminAccess(request);

      expect(global.fetch).toHaveBeenCalledWith(
        "https://auth.example.com/api/auth/get-session",
        expect.any(Object),
      );
    });

    it("handles multiple admin emails correctly", async () => {
      process.env.SAAS_ADMIN_EMAILS =
        "admin1@test.com,admin2@test.com,admin3@test.com";
      vi.resetModules();

      const mockSession = {
        user: {
          id: "user-789",
          email: "admin2@test.com",
          name: "Second Admin",
        },
      };

      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSession),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      const result = await verifyAdminAccess(request);

      expect(result).toEqual({
        email: "admin2@test.com",
        userId: "user-789",
        name: "Second Admin",
      });
    });

    it("returns null when json() throws", async () => {
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});

      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.reject(new Error("Invalid JSON")),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = createMockRequest("session=abc");

      const result = await verifyAdminAccess(request);

      expect(result).toBeNull();
      expect(consoleSpy).toHaveBeenCalled();

      consoleSpy.mockRestore();
    });
  });

  describe("AdminSession type", () => {
    it("has correct shape for exported type", async () => {
      const mockSession = {
        user: {
          id: "user-123",
          email: "admin@example.com",
          name: "Test Admin",
        },
      };

      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockSession),
      });

      const { verifyAdminAccess } = await import("./admin-auth");
      const request = {
        headers: {
          get: () => "session=abc",
        },
      } as unknown as NextRequest;

      const result = await verifyAdminAccess(request);

      // Type check - these properties must exist
      expect(result).toHaveProperty("email");
      expect(result).toHaveProperty("userId");
      expect(result).toHaveProperty("name");
      expect(typeof result?.email).toBe("string");
      expect(typeof result?.userId).toBe("string");
    });
  });
});
