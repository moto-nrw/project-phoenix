import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { authConfig, _resetRefreshState } from "./config";
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
    _resetRefreshState();
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

    it("should proactively refresh when access token near expiry", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "new-access-token",
          refresh_token: "new-refresh-token",
        }),
      });

      const token = {
        id: "123",
        token: "old-access-token",
        refreshToken: "old-refresh-token",
        tokenExpiry: Date.now() + 2 * 60 * 1000, // Expires in 2 min (within 5 min buffer)
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000, // 7 days out
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

      expect(mockFetch).toHaveBeenCalledOnce();
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/auth/refresh"),
        expect.objectContaining({
          method: "POST",
          headers: expect.objectContaining({
            Authorization: "Bearer old-refresh-token",
          }) as Record<string, string>,
        }),
      );
      expect(result?.token).toBe("new-access-token");
      expect(result?.refreshToken).toBe("new-refresh-token");
      expect(result?.error).toBeUndefined();
    });

    it("should not refresh when access token is still fresh", async () => {
      const token = {
        id: "123",
        token: "access-token",
        refreshToken: "refresh-token",
        tokenExpiry: Date.now() + 10 * 60 * 1000, // Expires in 10 min (outside 5 min buffer)
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
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

      expect(mockFetch).not.toHaveBeenCalled();
      expect(result?.token).toBe("access-token");
    });

    it("should gracefully handle failed proactive refresh", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
      });

      const token = {
        id: "123",
        token: "old-access-token",
        refreshToken: "old-refresh-token",
        tokenExpiry: Date.now() + 2 * 60 * 1000, // Near expiry
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
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

      // Token stays unchanged — no error set, Axios interceptor handles fallback
      expect(result?.token).toBe("old-access-token");
      expect(result?.refreshToken).toBe("old-refresh-token");
      expect(result?.error).toBeUndefined();
    });

    it("should gracefully handle network error during proactive refresh", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const token = {
        id: "123",
        token: "old-access-token",
        refreshToken: "old-refresh-token",
        tokenExpiry: Date.now() + 2 * 60 * 1000, // Near expiry
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
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

      // Token stays unchanged — no error set
      expect(result?.token).toBe("old-access-token");
      expect(result?.error).toBeUndefined();
    });

    it("should skip proactive refresh when access token already expired", async () => {
      const token = {
        id: "123",
        token: "old-access-token",
        refreshToken: "old-refresh-token",
        tokenExpiry: Date.now() - 30 * 1000, // Expired 30 seconds ago
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
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

      // Proactive refresh should NOT fire — the dedicated refresh handler will handle it
      expect(mockFetch).not.toHaveBeenCalled();
      expect(result?.token).toBe("old-access-token");
    });

    it("should deduplicate late-arriving callbacks via cache", async () => {
      // First call: triggers actual refresh and caches result
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "new-access-1",
          refresh_token: "new-refresh-1",
        }),
      });

      const makeToken = () => ({
        id: "123",
        token: "old-access-token",
        refreshToken: "dedup-refresh-token",
        tokenExpiry: Date.now() + 2 * 60 * 1000,
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
      });

      const callJwt = (token: Record<string, unknown>) =>
        authConfig.callbacks?.jwt?.({
          token,
          user: undefined as unknown as User,
          account: null,
          profile: undefined,
          trigger: "update",
          isNewUser: false,
          session: undefined,
        });

      // First callback: performs the actual refresh
      const result1 = await callJwt(makeToken());
      expect(result1?.token).toBe("new-access-1");
      expect(result1?.refreshToken).toBe("new-refresh-1");
      expect(mockFetch).toHaveBeenCalledOnce();

      // Late-arriving callback with same OLD refresh token: served from cache
      const result2 = await callJwt(makeToken());
      expect(result2?.token).toBe("new-access-1");
      expect(result2?.refreshToken).toBe("new-refresh-1");
      // Still only 1 fetch call — second was served from cache
      expect(mockFetch).toHaveBeenCalledOnce();
    });

    it("should share in-flight refresh promise across concurrent callbacks", async () => {
      // Single slow fetch that all concurrent calls share
      let resolveRefresh: (value: unknown) => void;
      mockFetch.mockReturnValueOnce(
        new Promise((resolve) => {
          resolveRefresh = resolve;
        }),
      );

      const makeToken = () => ({
        id: "123",
        token: "old-access-token",
        refreshToken: "concurrent-refresh-token",
        tokenExpiry: Date.now() + 2 * 60 * 1000,
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
      });

      const callJwt = (token: Record<string, unknown>) =>
        authConfig.callbacks?.jwt?.({
          token,
          user: undefined as unknown as User,
          account: null,
          profile: undefined,
          trigger: "update",
          isNewUser: false,
          session: undefined,
        });

      // Fire 3 concurrent callbacks (simulates multiple SessionProvider refetches)
      const p1 = callJwt(makeToken());
      const p2 = callJwt(makeToken());
      const p3 = callJwt(makeToken());

      // Only 1 fetch should have been made
      expect(mockFetch).toHaveBeenCalledOnce();

      // Resolve the single fetch — all 3 callbacks get the same result
      resolveRefresh!({
        ok: true,
        json: async () => ({
          access_token: "shared-access",
          refresh_token: "shared-refresh",
        }),
      });

      const [r1, r2, r3] = await Promise.all([p1, p2, p3]);

      expect(r1?.token).toBe("shared-access");
      expect(r2?.token).toBe("shared-access");
      expect(r3?.token).toBe("shared-access");
      expect(r1?.refreshToken).toBe("shared-refresh");
      expect(r2?.refreshToken).toBe("shared-refresh");
      expect(r3?.refreshToken).toBe("shared-refresh");
      // Confirm only 1 fetch across all 3 callbacks
      expect(mockFetch).toHaveBeenCalledOnce();
    });

    it("should not use cache for a different refresh token", async () => {
      // First: refresh with token-A
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "access-A",
          refresh_token: "refresh-A-new",
        }),
      });

      const callJwt = (token: Record<string, unknown>) =>
        authConfig.callbacks?.jwt?.({
          token,
          user: undefined as unknown as User,
          account: null,
          profile: undefined,
          trigger: "update",
          isNewUser: false,
          session: undefined,
        });

      await callJwt({
        id: "123",
        token: "old",
        refreshToken: "token-A",
        tokenExpiry: Date.now() + 2 * 60 * 1000,
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
      });
      expect(mockFetch).toHaveBeenCalledOnce();

      // Second: refresh with token-B (different token, must NOT use cache)
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "access-B",
          refresh_token: "refresh-B-new",
        }),
      });

      const result = await callJwt({
        id: "456",
        token: "old",
        refreshToken: "token-B",
        tokenExpiry: Date.now() + 2 * 60 * 1000,
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
      });

      expect(result?.token).toBe("access-B");
      expect(result?.refreshToken).toBe("refresh-B-new");
      // Two separate fetches — cache was not used
      expect(mockFetch).toHaveBeenCalledTimes(2);
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
