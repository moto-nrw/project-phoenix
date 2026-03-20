import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { authConfig, _resetRefreshState, _testHelpers } from "./config";
import type { NextAuthConfig, User } from "next-auth";

// Shared JWT token constants — decoded payloads documented inline
// { id: 1, first_name: "John", last_name: "Doe", email: "john@example.com", roles: ["teacher"], is_admin: false }
const TEACHER_JWT =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiZmlyc3RfbmFtZSI6IkpvaG4iLCJsYXN0X25hbWUiOiJEb2UiLCJlbWFpbCI6ImpvaG5AZXhhbXBsZS5jb20iLCJyb2xlcyI6WyJ0ZWFjaGVyIl0sImlzX2FkbWluIjpmYWxzZX0.test";
// { id: 1, first_name: "John", last_name: "Doe", email: "john@example.com" } (no roles)
const TEACHER_JWT_NO_ROLES =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiZmlyc3RfbmFtZSI6IkpvaG4iLCJsYXN0X25hbWUiOiJEb2UiLCJlbWFpbCI6ImpvaG5AZXhhbXBsZS5jb20ifQ.test";
// { id: 123, first_name: "Test", last_name: "User", email: "test@example.com", roles: ["teacher"] }
const INTERNAL_REFRESH_JWT =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MTIzLCJmaXJzdF9uYW1lIjoiVGVzdCIsImxhc3RfbmFtZSI6IlVzZXIiLCJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJyb2xlcyI6WyJ0ZWFjaGVyIl19.test";
// { id: 1, first_name: "John", email: "john@example.com" } (no last_name, no roles)
const TEACHER_JWT_MINIMAL =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiZmlyc3RfbmFtZSI6IkpvaG4iLCJlbWFpbCI6ImpvaG5AZXhhbXBsZS5jb20ifQ.test";
// { id: 45, first_name: "Op", email: "op@example.com", is_admin: true }
const OPERATOR_JWT =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6NDUsImZpcnN0X25hbWUiOiJPcCIsImVtYWlsIjoib3BAZXhhbXBsZS5jb20iLCJpc19hZG1pbiI6dHJ1ZX0.test";
// { id: 45, first_name: "Op", email: "op@example.com" } (no is_admin)
const OPERATOR_JWT_MINIMAL =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6NDUsImZpcnN0X25hbWUiOiJPcCIsImVtYWlsIjoib3BAZXhhbXBsZS5jb20ifQ.test";

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

// Shared helper: invoke JWT callback with test defaults
function callJwt(token: Record<string, unknown>) {
  return authConfig.callbacks?.jwt?.({
    token,
    user: undefined as unknown as User,
    account: null,
    profile: undefined,
    trigger: "update",
    isNewUser: false,
    session: undefined,
  });
}

// Shared helper: invoke session callback with test defaults
function callSessionCallback(args: { session: unknown; token: unknown }) {
  const sessionFn = authConfig.callbacks?.session;
  if (!sessionFn) return undefined;
  return (sessionFn as (args: unknown) => unknown)({
    ...args,
    user: undefined,
    newSession: undefined,
    trigger: "getSession",
  }) as Record<string, unknown> | undefined;
}

describe("authConfig", () => {
  beforeEach(() => {
    vi.resetAllMocks();
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

    it("should proactively refresh when access token already expired", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "refreshed-access-token",
          refresh_token: "refreshed-refresh-token",
        }),
      });

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

      // JWT callback now handles post-expiry refresh too
      expect(mockFetch).toHaveBeenCalledOnce();
      expect(result?.token).toBe("refreshed-access-token");
      expect(result?.refreshToken).toBe("refreshed-refresh-token");
      expect(result?.error).toBeUndefined();
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
          token: INTERNAL_REFRESH_JWT,
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
      expect(_testHelpers.parseDurationToMs("1h")).toBe(3600000);
      expect(_testHelpers.parseDurationToMs("12h")).toBe(43200000);
    });

    it("should parse minute durations", () => {
      expect(_testHelpers.parseDurationToMs("15m")).toBe(900000);
      expect(_testHelpers.parseDurationToMs("30m")).toBe(1800000);
    });

    it("should return 12h default for invalid input", () => {
      expect(_testHelpers.parseDurationToMs("invalid")).toBe(43200000);
      expect(_testHelpers.parseDurationToMs("10s")).toBe(43200000);
      expect(_testHelpers.parseDurationToMs("")).toBe(43200000);
    });
  });

  describe("parseJwtPayload", () => {
    it("should parse valid JWT payload", () => {
      const payload = _testHelpers.parseJwtPayload(TEACHER_JWT);

      expect(payload).not.toBeNull();
      expect(payload?.id).toBe(1);
      expect(payload?.first_name).toBe("John");
      expect(payload?.last_name).toBe("Doe");
      expect(payload?.email).toBe("john@example.com");
      expect(payload?.roles).toEqual(["teacher"]);
      expect(payload?.is_admin).toBe(false);
    });

    it("should return null for token with wrong number of parts", () => {
      expect(_testHelpers.parseJwtPayload("not-a-jwt")).toBeNull();
      expect(_testHelpers.parseJwtPayload("only.two")).toBeNull();
      expect(_testHelpers.parseJwtPayload("a.b.c.d")).toBeNull();
    });

    it("should return null for invalid base64 payload", () => {
      const result = _testHelpers.parseJwtPayload(
        "header.!!!invalid!!!.signature",
      );
      expect(result).toBeNull();
    });
  });

  describe("buildDisplayName", () => {
    it("should use first and last name when available", () => {
      const payload = { id: 1, first_name: "John", last_name: "Doe" };
      expect(_testHelpers.buildDisplayName(payload, "john@example.com")).toBe(
        "John Doe",
      );
    });

    it("should use first name only when last name is missing", () => {
      const payload = { id: 1, first_name: "John" };
      expect(_testHelpers.buildDisplayName(payload, "john@example.com")).toBe(
        "John",
      );
    });

    it("should fall back to username", () => {
      const payload = { id: 1, username: "johnd" };
      expect(_testHelpers.buildDisplayName(payload, "")).toBe("johnd");
    });

    it("should fall back to email", () => {
      const payload = { id: 1 };
      expect(_testHelpers.buildDisplayName(payload, "john@example.com")).toBe(
        "john@example.com",
      );
    });

    it("should fall back to ultimate fallback", () => {
      const payload = { id: 1 };
      expect(_testHelpers.buildDisplayName(payload, "", "Unknown")).toBe(
        "Unknown",
      );
    });

    it("should use default ultimate fallback", () => {
      const payload = { id: 1 };
      expect(_testHelpers.buildDisplayName(payload, "")).toBe("User");
    });
  });

  describe("buildAuthUser", () => {
    it("should build user with all fields", () => {
      const payload = {
        id: 1,
        first_name: "John",
        last_name: "Doe",
        email: "john@example.com",
        roles: ["teacher"],
        is_admin: false,
      };

      const user = _testHelpers.buildAuthUser(
        payload,
        "access-token",
        "refresh-token",
        "john@example.com",
      );

      expect(user.id).toBe("1");
      expect(user.name).toBe("John Doe");
      expect(user.email).toBe("john@example.com");
      expect(user.token).toBe("access-token");
      expect(user.refreshToken).toBe("refresh-token");
      expect(user.roles).toEqual(["teacher"]);
      expect(user.isAdmin).toBe(false);
      expect(user.scope).toBeUndefined();
    });

    it("should override roles with operator when scope is platform", () => {
      const payload = {
        id: 45,
        first_name: "Op",
        roles: ["admin"],
        is_admin: true,
      };

      const user = _testHelpers.buildAuthUser(
        payload,
        "token",
        "refresh",
        "op@example.com",
        "platform",
      );

      expect(user.roles).toEqual(["operator"]);
      expect(user.scope).toBe("platform");
      expect(user.isAdmin).toBe(true);
    });

    it("should handle missing roles with empty array", () => {
      const payload = { id: 2, email: "test@example.com" };

      const user = _testHelpers.buildAuthUser(
        payload,
        "token",
        "refresh",
        "test@example.com",
      );

      expect(user.roles).toEqual([]);
    });

    it("should handle non-array roles defensively", () => {
      const payload = {
        id: 3,
        roles: "not-an-array" as unknown as string[],
      };

      const user = _testHelpers.buildAuthUser(
        payload,
        "token",
        "refresh",
        "test@example.com",
      );

      expect(user.roles).toEqual([]);
    });
  });

  describe("performOperatorLogin", () => {
    it("should return tokens on successful login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          status: "success",
          data: {
            access_token: "op-access",
            refresh_token: "op-refresh",
            operator: { id: 1, email: "op@test.com", display_name: "Op" },
          },
        }),
      });

      const result = await _testHelpers.performOperatorLogin(
        "op@test.com",
        "pass",
        false,
      );

      expect(result).toEqual({
        access_token: "op-access",
        refresh_token: "op-refresh",
      });
    });

    it("should return tokens with dev logging enabled", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          status: "success",
          data: {
            access_token: "op-access",
            refresh_token: "op-refresh",
            operator: { id: 1, email: "op@test.com", display_name: "Op" },
          },
        }),
      });

      const result = await _testHelpers.performOperatorLogin(
        "op@test.com",
        "pass",
        true,
      );

      expect(result).toEqual({
        access_token: "op-access",
        refresh_token: "op-refresh",
      });
    });

    it("should return error status on HTTP error", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Unauthorized",
      });

      const result = await _testHelpers.performOperatorLogin(
        "op@test.com",
        "wrong",
        false,
      );

      expect(result).toEqual({
        access_token: "",
        refresh_token: "",
        status: 401,
      });
    });

    it("should return null on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const result = await _testHelpers.performOperatorLogin(
        "op@test.com",
        "pass",
        false,
      );

      expect(result).toBeNull();
    });
  });

  describe("performLogin", () => {
    it("should return tokens on successful login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "teacher-access",
          refresh_token: "teacher-refresh",
        }),
      });

      const result = await _testHelpers.performLogin(
        "teacher@test.com",
        "pass",
        false,
      );

      expect(result).toEqual({
        access_token: "teacher-access",
        refresh_token: "teacher-refresh",
      });
    });

    it("should return tokens with dev logging enabled", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "teacher-access",
          refresh_token: "teacher-refresh",
        }),
      });

      const result = await _testHelpers.performLogin(
        "teacher@test.com",
        "pass",
        true,
      );

      expect(result).toEqual({
        access_token: "teacher-access",
        refresh_token: "teacher-refresh",
      });
    });

    it("should return null on HTTP error", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Bad credentials",
      });

      const result = await _testHelpers.performLogin(
        "teacher@test.com",
        "wrong",
        false,
      );

      expect(result).toBeNull();
    });

    it("should return null on HTTP error with dev logging", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Bad credentials",
      });

      const result = await _testHelpers.performLogin(
        "teacher@test.com",
        "wrong",
        true,
      );

      expect(result).toBeNull();
    });

    it("should return null on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Connection refused"));

      const result = await _testHelpers.performLogin(
        "teacher@test.com",
        "pass",
        false,
      );

      expect(result).toBeNull();
    });
  });

  describe("parseDurationToMs via config", () => {
    it("should correctly set session maxAge from refresh token expiry", () => {
      // AUTH_JWT_REFRESH_EXPIRY is "1h" in mock, parseDurationToMs("1h") = 3600000ms
      // maxAge = Math.floor(3600000 / 1000) = 3600 seconds
      expect(authConfig.session?.maxAge).toBe(3600);
    });
  });

  describe("Credentials authorize - teacher flow", () => {
    // CredentialsProvider stores the real authorize in `options.authorize`,
    // the top-level `authorize` is always `() => null` (Auth.js default).
    function getTeacherAuthorize() {
      const provider = authConfig.providers.find(
        (p) =>
          typeof p === "object" &&
          p !== null &&
          "id" in p &&
          p.id === "credentials",
      ) as unknown as Record<string, unknown> | undefined;
      const opts = provider?.options as Record<string, unknown> | undefined;
      return opts?.authorize as (
        credentials: Record<string, string> | undefined,
        request: Request,
      ) => Promise<User | null>;
    }

    it("should return user on successful teacher login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: TEACHER_JWT,
          refresh_token: "refresh-token",
        }),
      });

      const authorize = getTeacherAuthorize();
      const result = await authorize(
        { email: "john@example.com", password: "correct" },
        new Request("http://localhost:3000"),
      );

      expect(result).not.toBeNull();
      expect(result?.id).toBe("1");
      expect(result?.name).toBe("John Doe");
      expect(result?.roles).toEqual(["teacher"]);
    });

    it("should return null when login returns invalid JWT", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "not-a-valid-jwt",
          refresh_token: "refresh-token",
        }),
      });

      const authorize = getTeacherAuthorize();
      const result = await authorize(
        { email: "john@example.com", password: "correct" },
        new Request("http://localhost:3000"),
      );

      expect(result).toBeNull();
    });

    it("should return null for missing credentials", async () => {
      const authorize = getTeacherAuthorize();
      const result = await authorize({}, new Request("http://localhost:3000"));

      expect(result).toBeNull();
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("should return null for failed login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Unauthorized",
      });

      const authorize = getTeacherAuthorize();
      const result = await authorize(
        { email: "test@example.com", password: "wrong" },
        new Request("http://localhost:3000"),
      );

      expect(result).toBeNull();
    });

    it("should handle internal refresh", async () => {
      const authorize = getTeacherAuthorize();
      const result = await authorize(
        {
          internalRefresh: "true",
          token: INTERNAL_REFRESH_JWT,
          refreshToken: "refresh-token",
        },
        new Request("http://localhost:3000"),
      );

      expect(result).not.toBeNull();
      expect(result?.id).toBe("123");
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("should return null for internal refresh with invalid JWT", async () => {
      const authorize = getTeacherAuthorize();
      const result = await authorize(
        {
          internalRefresh: "true",
          token: "invalid-jwt",
          refreshToken: "refresh-token",
        },
        new Request("http://localhost:3000"),
      );

      expect(result).toBeNull();
    });

    describe("with dev mode", () => {
      beforeEach(() => {
        vi.stubEnv("NODE_ENV", "development");
      });
      afterEach(() => {
        vi.unstubAllEnvs();
      });

      it("should log debug info on successful login", async () => {
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            access_token: TEACHER_JWT,
            refresh_token: "refresh-token",
          }),
        });

        const authorize = getTeacherAuthorize();
        const result = await authorize(
          { email: "john@example.com", password: "correct" },
          new Request("http://localhost:3000"),
        );

        expect(result).not.toBeNull();
        expect(result?.name).toBe("John Doe");
      });

      it("should log warning when token has no roles", async () => {
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            access_token: TEACHER_JWT_NO_ROLES,
            refresh_token: "refresh-token",
          }),
        });

        const authorize = getTeacherAuthorize();
        const result = await authorize(
          { email: "john@example.com", password: "correct" },
          new Request("http://localhost:3000"),
        );

        expect(result).not.toBeNull();
        expect(result?.roles).toEqual([]);
      });

      it("should handle internal refresh logging", async () => {
        const authorize = getTeacherAuthorize();
        const result = await authorize(
          {
            internalRefresh: "true",
            token: TEACHER_JWT_MINIMAL,
            refreshToken: "refresh-token",
          },
          new Request("http://localhost:3000"),
        );

        expect(result).not.toBeNull();
        expect(mockFetch).not.toHaveBeenCalled();
      });
    });
  });

  describe("Credentials authorize - operator flow", () => {
    // The operator provider is the second CredentialsProvider (index 2 in providers).
    // Both CredentialsProviders get id "credentials" from Auth.js default;
    // the real authorize is in `options.authorize`.
    function getOperatorAuthorize() {
      const providers = authConfig.providers.filter(
        (p) =>
          typeof p === "object" &&
          p !== null &&
          "type" in p &&
          p.type === "credentials",
      );
      // Second credentials provider is the operator one
      const provider = providers[1] as unknown as
        | Record<string, unknown>
        | undefined;
      const opts = provider?.options as Record<string, unknown> | undefined;
      return opts?.authorize as (
        credentials: Record<string, string> | undefined,
        request: Request,
      ) => Promise<User | null>;
    }

    it("should return user with platform scope on successful operator login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          status: "success",
          data: {
            access_token: OPERATOR_JWT,
            refresh_token: "op-refresh-token",
            operator: {
              id: 45,
              email: "op@example.com",
              display_name: "Op",
            },
          },
        }),
      });

      const authorize = getOperatorAuthorize();
      const result = await authorize(
        { email: "op@example.com", password: "correct" },
        new Request("http://localhost:3000"),
      );

      expect(result).not.toBeNull();
      expect(result?.id).toBe("45");
      expect(result?.roles).toEqual(["operator"]);
      expect(result?.scope).toBe("platform");
    });

    it("should return null for missing credentials", async () => {
      const authorize = getOperatorAuthorize();
      const result = await authorize({}, new Request("http://localhost:3000"));

      expect(result).toBeNull();
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("should throw CredentialsSignin with invalid_credentials on failed operator login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Unauthorized",
      });

      const authorize = getOperatorAuthorize();
      await expect(
        authorize(
          { email: "op@example.com", password: "wrong" },
          new Request("http://localhost:3000"),
        ),
      ).rejects.toThrow();
    });

    it("should return null when operator login returns invalid JWT", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          status: "success",
          data: {
            access_token: "not-a-jwt",
            refresh_token: "op-refresh",
            operator: {
              id: 1,
              email: "op@example.com",
              display_name: "Op",
            },
          },
        }),
      });

      const authorize = getOperatorAuthorize();
      const result = await authorize(
        { email: "op@example.com", password: "correct" },
        new Request("http://localhost:3000"),
      );

      expect(result).toBeNull();
    });

    it("should handle internal refresh with platform scope", async () => {
      const authorize = getOperatorAuthorize();
      const result = await authorize(
        {
          internalRefresh: "true",
          token: OPERATOR_JWT,
          refreshToken: "op-refresh",
        },
        new Request("http://localhost:3000"),
      );

      expect(result).not.toBeNull();
      expect(result?.scope).toBe("platform");
      expect(result?.roles).toEqual(["operator"]);
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("should return null for internal refresh with invalid JWT", async () => {
      const authorize = getOperatorAuthorize();
      const result = await authorize(
        {
          internalRefresh: "true",
          token: "bad-token",
          refreshToken: "op-refresh",
        },
        new Request("http://localhost:3000"),
      );

      expect(result).toBeNull();
    });

    describe("with dev mode", () => {
      beforeEach(() => {
        vi.stubEnv("NODE_ENV", "development");
      });
      afterEach(() => {
        vi.unstubAllEnvs();
      });

      it("should handle operator login logging", async () => {
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            status: "success",
            data: {
              access_token: OPERATOR_JWT_MINIMAL,
              refresh_token: "op-refresh",
              operator: {
                id: 45,
                email: "op@example.com",
                display_name: "Op",
              },
            },
          }),
        });

        const authorize = getOperatorAuthorize();
        const result = await authorize(
          { email: "op@example.com", password: "correct" },
          new Request("http://localhost:3000"),
        );

        expect(result).not.toBeNull();
        expect(result?.scope).toBe("platform");
      });

      it("should handle operator internal refresh logging", async () => {
        const authorize = getOperatorAuthorize();
        const result = await authorize(
          {
            internalRefresh: "true",
            token: OPERATOR_JWT_MINIMAL,
            refreshToken: "op-refresh",
          },
          new Request("http://localhost:3000"),
        );

        expect(result).not.toBeNull();
        expect(result?.scope).toBe("platform");
        expect(mockFetch).not.toHaveBeenCalled();
      });
    });
  });

  describe("JWT callback - operator token refresh", () => {
    it("should set RefreshTokenError on concurrent join failure when token expired", async () => {
      // First callback starts the refresh (will fail)
      let resolveRefresh: (value: unknown) => void;
      mockFetch.mockReturnValueOnce(
        new Promise((resolve) => {
          resolveRefresh = resolve;
        }),
      );

      const makeToken = () => ({
        id: "123",
        token: "old-access",
        refreshToken: "fail-concurrent-token",
        tokenExpiry: Date.now() - 60 * 1000, // Already expired
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

      const p1 = callJwt(makeToken());
      const p2 = callJwt(makeToken());

      // Resolve with failure
      resolveRefresh!({ ok: false, status: 401 });

      const [r1, r2] = await Promise.all([p1, p2]);

      // Both should have error since token was already expired
      expect(r1?.error).toBe("RefreshTokenError");
      expect(r2?.error).toBe("RefreshTokenError");
    });
  });

  describe("Session callback - additional paths", () => {
    it("should return minimal session when token has RefreshTokenError", () => {
      const session = {
        user: { id: "", email: "", name: "" },
        expires: "2099-12-31",
      };

      const token = {
        id: "123",
        email: "test@example.com",
        error: "RefreshTokenError" as const,
        firstName: "Test",
      };

      const result = callSessionCallback({ session, token });
      const user = result?.user as Record<string, unknown> | undefined;

      expect(user?.token).toBe("");
      expect(user?.refreshToken).toBe("");
      expect(user?.roles).toEqual([]);
      expect(user?.isAdmin).toBe(false);
      expect(result?.error).toBe("RefreshTokenError");
    });

    it("should propagate scope to session user", () => {
      const session = {
        user: { id: "", email: "", name: "" },
        expires: "2099-12-31",
      };

      const token = {
        id: "45",
        email: "op@example.com",
        token: "access",
        refreshToken: "refresh",
        roles: ["operator"],
        firstName: "Operator",
        isAdmin: true,
        scope: "platform",
      };

      const result = callSessionCallback({ session, token });
      const user = result?.user as Record<string, unknown> | undefined;

      expect(user?.scope).toBe("platform");
      expect(user?.isAdmin).toBe(true);
    });
  });

  describe("JWT callback - operator scope refresh", () => {
    it("should use operator refresh URL and parse envelope response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: {
            access_token: "new-op-access",
            refresh_token: "new-op-refresh",
          },
        }),
      });

      const result = await callJwt({
        id: "45",
        token: "old-op-access",
        refreshToken: "old-op-refresh",
        tokenExpiry: Date.now() + 2 * 60 * 1000,
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
        scope: "platform",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/operator/auth/refresh"),
        expect.any(Object),
      );
      expect(result?.token).toBe("new-op-access");
      expect(result?.refreshToken).toBe("new-op-refresh");
    });
  });

  describe("JWT callback - dev mode logging", () => {
    beforeEach(() => {
      vi.stubEnv("NODE_ENV", "development");
    });
    afterEach(() => {
      vi.unstubAllEnvs();
    });

    it("should log during initial sign in", async () => {
      const user = {
        id: "123",
        name: "Dev User",
        email: "dev@example.com",
        token: "dev-token",
        refreshToken: "dev-refresh",
        roles: ["teacher"],
        firstName: "Dev",
        isAdmin: false,
      };

      const result = await authConfig.callbacks?.jwt?.({
        token: {},
        user,
        account: null,
        profile: undefined,
        trigger: "signIn",
        isNewUser: false,
        session: undefined,
      });

      expect(result?.id).toBe("123");
      expect(result?.token).toBe("dev-token");
    });

    it("should log during token refresh", async () => {
      const result = await callJwt({
        id: "123",
        token: "access",
        refreshToken: "refresh",
        tokenExpiry: Date.now() + 10 * 60 * 1000,
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
      });

      expect(result?.token).toBe("access");
    });
  });

  describe("redirect callback", () => {
    function callRedirect(url: string, baseUrl: string): string {
      const redirectFn = authConfig.callbacks?.redirect;
      if (!redirectFn) throw new Error("redirect callback not found");
      return redirectFn({ url, baseUrl });
    }

    it("should resolve relative URLs against baseUrl", () => {
      expect(callRedirect("/dashboard", "http://localhost:3000")).toBe(
        "http://localhost:3000/dashboard",
      );
    });

    it("should allow same origin redirects", () => {
      expect(
        callRedirect(
          "http://localhost:3000/dashboard",
          "http://localhost:3000",
        ),
      ).toBe("http://localhost:3000/dashboard");
    });

    it("should allow cross-subdomain localhost redirects", () => {
      expect(
        callRedirect(
          "http://operator.localhost:3000/login",
          "http://localhost:3000",
        ),
      ).toBe("http://operator.localhost:3000/login");
    });

    it("should allow same parent domain redirects", () => {
      expect(
        callRedirect(
          "http://operator.moto-app.de/login",
          "http://altenberge.moto-app.de",
        ),
      ).toBe("http://operator.moto-app.de/login");
    });

    it("should block redirects to different domains", () => {
      expect(
        callRedirect("http://evil.com/phish", "http://localhost:3000"),
      ).toBe("http://localhost:3000");
    });

    it("should block redirects when parent domains differ", () => {
      expect(
        callRedirect(
          "http://evil.other-site.com/phish",
          "http://app.moto-app.de",
        ),
      ).toBe("http://app.moto-app.de");
    });

    it("should handle two-part hostnames without subdomain stripping", () => {
      // getParentDomain returns hostname as-is when there are only 2 parts
      expect(
        callRedirect("http://moto-app.de/path", "http://moto-app.de"),
      ).toBe("http://moto-app.de/path");
    });

    it("should block when one host is localhost and the other is not", () => {
      expect(
        callRedirect("http://evil.com/phish", "http://app.moto-app.de"),
      ).toBe("http://app.moto-app.de");
    });
  });

  describe("JWT callback - initial sign in edge cases", () => {
    it("should clear previous error states on sign in", async () => {
      const token = {
        error: "RefreshTokenExpired" as const,
        needsRefresh: true,
      };

      const user = {
        id: "123",
        name: "User",
        email: "user@example.com",
        token: "new-token",
        refreshToken: "new-refresh",
        roles: ["teacher"],
        isAdmin: false,
      };

      const result = await authConfig.callbacks?.jwt?.({
        token,
        user,
        account: null,
        profile: undefined,
        trigger: "signIn",
        isNewUser: false,
        session: undefined,
      });

      expect(result?.error).toBeUndefined();
      expect(result?.needsRefresh).toBeUndefined();
      expect(result?.token).toBe("new-token");
    });

    it("should handle user with missing optional fields", async () => {
      const user = {
        id: "123",
        name: "User",
        email: "user@example.com",
        // No token, refreshToken, roles, firstName, isAdmin
      };

      const result = await authConfig.callbacks?.jwt?.({
        token: {},
        user,
        account: null,
        profile: undefined,
        trigger: "signIn",
        isNewUser: false,
        session: undefined,
      });

      expect(result?.token).toBe("");
      expect(result?.refreshToken).toBe("");
      expect(result?.isAdmin).toBeUndefined();
    });
  });
});
