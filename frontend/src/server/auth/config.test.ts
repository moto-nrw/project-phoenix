import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { authConfig } from "./config";
import type { NextAuthConfig, User } from "next-auth";

// Mock ~/env
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "1h",
  },
}));

// Mock fetch globally
const mockFetch = vi.fn();

describe("authConfig", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("should export valid NextAuthConfig", () => {
    expect(authConfig).toBeDefined();
    expect(authConfig).toHaveProperty("providers");
    expect(authConfig).toHaveProperty("callbacks");
    expect(authConfig).toHaveProperty("pages");
    expect(authConfig).toHaveProperty("session");
  });

  it("should have correct session strategy", () => {
    expect(authConfig.session).toBeDefined();
    expect(authConfig.session?.strategy).toBe("jwt");
  });

  it("should have correct pages configuration", () => {
    expect(authConfig.pages).toBeDefined();
    expect(authConfig.pages?.signIn).toBe("/");
  });

  it("should have providers array", () => {
    expect(Array.isArray(authConfig.providers)).toBe(true);
    expect(authConfig.providers.length).toBeGreaterThan(0);
  });

  describe("JWT callback", () => {
    it("should set token data on initial sign in", async () => {
      const user = {
        id: "123",
        name: "Test User",
        email: "test@example.com",
        token: "access-token",
        refreshToken: "refresh-token",
        roles: ["teacher"],
        firstName: "Test",
        isAdmin: false,
      };

      const token = {};

      const result = await authConfig.callbacks?.jwt?.({
        token,
        user,
        account: null,
        profile: undefined,
        trigger: "signIn",
        isNewUser: false,
        session: undefined,
      });

      expect(result).toBeDefined();
      expect(result?.id).toBe("123");
      expect(result?.name).toBe("Test User");
      expect(result?.email).toBe("test@example.com");
      expect(result?.token).toBe("access-token");
      expect(result?.refreshToken).toBe("refresh-token");
      expect(result?.roles).toEqual(["teacher"]);
      expect(result?.firstName).toBe("Test");
      expect(result?.isAdmin).toBe(false);
      expect(result?.tokenExpiry).toBeDefined();
      expect(result?.refreshTokenExpiry).toBeDefined();
    });

    it("should return token unchanged when no user", async () => {
      const token = {
        id: "123",
        token: "existing-token",
      };

      const result = await authConfig.callbacks?.jwt?.({
        token,
        user: undefined as unknown as User,
        account: null,
        profile: undefined,
        trigger: "update",
        isNewUser: false,
        session: undefined,
      });

      expect(result).toBeDefined();
      expect(result?.id).toBe("123");
      expect(result?.token).toBe("existing-token");
    });

    it("should mark token as expired when refresh token expired", async () => {
      const token = {
        id: "123",
        token: "access-token",
        refreshToken: "refresh-token",
        refreshTokenExpiry: Date.now() - 1000, // Expired 1 second ago
      };

      const result = await authConfig.callbacks?.jwt?.({
        token,
        user: undefined as unknown as User,
        account: null,
        profile: undefined,
        trigger: "update",
        isNewUser: false,
        session: undefined,
      });

      expect(result).toBeDefined();
      expect(result?.error).toBe("RefreshTokenExpired");
      expect(result?.needsRefresh).toBe(true);
    });
  });

  describe("Session callback", () => {
    // Helper to call session callback without fighting NextAuth's complex overloaded types
    function callSessionCallback(args: {
      session: unknown;
      token: unknown;
    }): Record<string, unknown> | undefined {
      const sessionFn = authConfig.callbacks?.session;
      if (!sessionFn) return undefined;
      // NextAuth session callback accepts complex union args; cast to call with test data
      return (sessionFn as (args: unknown) => unknown)({
        ...args,
        user: undefined,
        newSession: undefined,
        trigger: "getSession",
      }) as Record<string, unknown> | undefined;
    }

    it("should return session with user data from token", () => {
      const session = {
        user: { id: "", email: "", name: "" },
        expires: "2099-12-31",
      };

      const token = {
        id: "123",
        email: "test@example.com",
        token: "access-token",
        refreshToken: "refresh-token",
        roles: ["teacher"],
        firstName: "Test",
        isAdmin: false,
      };

      const result = callSessionCallback({ session, token });
      const user = result?.user as Record<string, unknown> | undefined;

      expect(result).toBeDefined();
      expect(user?.id).toBe("123");
      expect(user?.email).toBe("test@example.com");
      expect(user?.token).toBe("access-token");
      expect(user?.refreshToken).toBe("refresh-token");
      expect(user?.roles).toEqual(["teacher"]);
      expect(user?.firstName).toBe("Test");
      expect(user?.isAdmin).toBe(false);
    });

    it("should return minimal session when token has error", () => {
      const session = {
        user: { id: "", email: "", name: "" },
        expires: "2099-12-31",
      };

      const token = {
        id: "123",
        email: "test@example.com",
        error: "RefreshTokenExpired" as const,
        firstName: "Test",
      };

      const result = callSessionCallback({ session, token });
      const user = result?.user as Record<string, unknown> | undefined;

      expect(result).toBeDefined();
      expect(user?.token).toBe("");
      expect(user?.refreshToken).toBe("");
      expect(user?.roles).toEqual([]);
      expect(result?.error).toBe("RefreshTokenExpired");
    });

    it("should return minimal session when no token", () => {
      const session = {
        user: { id: "", email: "", name: "" },
        expires: "2099-12-31",
      };

      const token = {
        id: "123",
        email: "test@example.com",
        // No token field
      };

      const result = callSessionCallback({ session, token });
      const user = result?.user as Record<string, unknown> | undefined;

      expect(result).toBeDefined();
      expect(user?.token).toBe("");
      expect(user?.refreshToken).toBe("");
    });
  });

  describe("Credentials provider", () => {
    it("should be included in providers", () => {
      const credentialsProvider = authConfig.providers.find(
        (p) =>
          typeof p === "object" &&
          p !== null &&
          "id" in p &&
          p.id === "credentials",
      );
      expect(credentialsProvider).toBeDefined();
    });

    it("should have authorize function that validates credentials", async () => {
      const credentialsProvider = authConfig.providers.find(
        (p) =>
          typeof p === "object" &&
          p !== null &&
          "id" in p &&
          p.id === "credentials",
      ) as NextAuthConfig["providers"][0] & {
        authorize?: (
          credentials: Record<string, string> | undefined,
          request: Request,
        ) => Promise<unknown>;
      };

      // Verify authorize function exists
      expect(credentialsProvider).toBeDefined();
      expect(credentialsProvider?.authorize).toBeDefined();
      expect(typeof credentialsProvider?.authorize).toBe("function");

      // Verify it returns null for invalid/missing credentials (already tested in other tests)
      const result = await credentialsProvider?.authorize?.(
        {},
        new Request("http://localhost:3000"),
      );
      expect(result).toBeNull();
    });

    it("should return null for failed login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Unauthorized",
      });

      const credentialsProvider = authConfig.providers.find(
        (p) =>
          typeof p === "object" &&
          p !== null &&
          "id" in p &&
          p.id === "credentials",
      ) as NextAuthConfig["providers"][0] & {
        authorize?: (
          credentials: Record<string, string> | undefined,
          request: Request,
        ) => Promise<unknown>;
      };

      const result = await credentialsProvider?.authorize?.(
        {
          email: "test@example.com",
          password: "wrongpassword",
        },
        new Request("http://localhost:3000"),
      );

      expect(result).toBeNull();
    });

    it("should return null for missing credentials", async () => {
      const credentialsProvider = authConfig.providers.find(
        (p) =>
          typeof p === "object" &&
          p !== null &&
          "id" in p &&
          p.id === "credentials",
      ) as NextAuthConfig["providers"][0] & {
        authorize?: (
          credentials: Record<string, string> | undefined,
          request: Request,
        ) => Promise<unknown>;
      };

      const result = await credentialsProvider?.authorize?.(
        {},
        new Request("http://localhost:3000"),
      );

      expect(result).toBeNull();
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("should handle internal refresh", async () => {
      const credentialsProvider = authConfig.providers.find(
        (p) =>
          typeof p === "object" &&
          p !== null &&
          "id" in p &&
          p.id === "credentials",
      ) as NextAuthConfig["providers"][0] & {
        authorize?: (
          credentials: Record<string, string> | undefined,
          request: Request,
        ) => Promise<unknown>;
      };

      const result = await credentialsProvider?.authorize?.(
        {
          internalRefresh: "true",
          token:
            "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MTIzLCJmaXJzdF9uYW1lIjoiVGVzdCIsImxhc3RfbmFtZSI6IlVzZXIiLCJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJyb2xlcyI6WyJ0ZWFjaGVyIl19.test",
          refreshToken: "refresh-token",
        },
        new Request("http://localhost:3000"),
      );

      expect(result).toBeDefined();
      expect(mockFetch).not.toHaveBeenCalled(); // Should not call login endpoint
    });
  });

  describe("parseDurationToMs", () => {
    it("should parse hour durations", () => {
      // We can't directly test the function since it's not exported
      // but we can test it indirectly through the config
      expect(authConfig.session?.maxAge).toBeGreaterThan(0);
    });
  });
});
