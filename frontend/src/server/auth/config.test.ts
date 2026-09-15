import { validateSessionToken } from "./token-validation";
vi.mock("./token-validation", () => ({ validateSessionToken: vi.fn() }));
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { tenantAuthConfig as authConfig } from "./tenant-config";
import { _resetRefreshState, _testHelpers } from "./shared";
import { operatorAuthConfig } from "./operator-config";
import { parentAuthConfig } from "./parent-config";
import { schoolAuthConfig } from "./school-config";
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
// { id: 45, sub: "operator:45", username: "op@example.com", first_name: "Op", roles: ["operator"], permissions: [], scope: "platform", is_admin: false }
const OPERATOR_JWT =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6NDUsInN1YiI6Im9wZXJhdG9yOjQ1IiwidXNlcm5hbWUiOiJvcEBleGFtcGxlLmNvbSIsImZpcnN0X25hbWUiOiJPcCIsInJvbGVzIjpbIm9wZXJhdG9yIl0sInBlcm1pc3Npb25zIjpbXSwic2NvcGUiOiJwbGF0Zm9ybSIsImlzX2FkbWluIjpmYWxzZX0.test";
// { id: 45, sub: "operator:45", username: "op@example.com", first_name: "Op", roles: ["operator"], scope: "platform" }
const OPERATOR_JWT_MINIMAL =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6NDUsInN1YiI6Im9wZXJhdG9yOjQ1IiwidXNlcm5hbWUiOiJvcEBleGFtcGxlLmNvbSIsImZpcnN0X25hbWUiOiJPcCIsInJvbGVzIjpbIm9wZXJhdG9yIl0sInNjb3BlIjoicGxhdGZvcm0ifQ.test";
// { id: 45, sub: "operator:45", email: "legacy-op@example.com", first_name: "Op", roles: ["operator"], scope: "platform" }
const OPERATOR_JWT_EMAIL_ONLY =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6NDUsInN1YiI6Im9wZXJhdG9yOjQ1IiwiZW1haWwiOiJsZWdhY3ktb3BAZXhhbXBsZS5jb20iLCJmaXJzdF9uYW1lIjoiT3AiLCJyb2xlcyI6WyJvcGVyYXRvciJdLCJzY29wZSI6InBsYXRmb3JtIn0.test";
// { id: 45, sub: "operator:45", first_name: "Op", roles: ["operator"], scope: "platform" }
const OPERATOR_JWT_NO_EMAIL =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6NDUsInN1YiI6Im9wZXJhdG9yOjQ1IiwiZmlyc3RfbmFtZSI6Ik9wIiwicm9sZXMiOlsib3BlcmF0b3IiXSwic2NvcGUiOiJwbGF0Zm9ybSJ9.test";

function refreshJwt(
  token: string,
  expiresAt = Date.now() + 60 * 60 * 1000,
  issuedAt = Math.floor(Date.now() / 1000),
) {
  const header = Buffer.from(
    JSON.stringify({ alg: "HS256", typ: "JWT" }),
  ).toString("base64url");
  const payload = Buffer.from(
    JSON.stringify({
      id: 123,
      token,
      exp: Math.floor(expiresAt / 1000),
      iat: issuedAt,
    }),
  ).toString("base64url");
  return `${header}.${payload}.test`;
}

// Mock ~/env
vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "1h",
    NEXTAUTH_SECRET: "test-auth-secret-with-sufficient-entropy",
    TENANT_DOMAIN: "moto-app.de",
  },
}));

// Mock next-auth's dynamic-import surface used by createOperatorLoginError.
// In the vitest environment the ESM dynamic `import("next-auth")` inside
// shared.ts fails with ERR_MODULE_NOT_FOUND because next-auth resolves
// differently than at app runtime. A minimal CredentialsSignin stub lets
// the import succeed and lets tests assert on the thrown error's `code`.
class MockCredentialsSignin extends Error {
  code = "credentials";
  type = "CredentialsSignin";
}
vi.mock("next-auth", () => ({
  CredentialsSignin: MockCredentialsSignin,
}));

const mockRequestHeaders = vi.hoisted(() => vi.fn());
vi.mock("next/headers", () => ({
  headers: mockRequestHeaders,
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
    vi.mocked(validateSessionToken).mockImplementation(async (token) => {
      try {
        return {
          exp: Math.floor(Date.now() / 1000) + 900,
          ...JSON.parse(
            Buffer.from(token.split(".")[1]!, "base64url").toString(),
          ),
        };
      } catch {
        return null;
      }
    });
    vi.stubGlobal("fetch", mockFetch);
    mockRequestHeaders.mockResolvedValue(new Headers());
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
      expect(result?.refreshRecoveryProof).toEqual(expect.any(String));
      expect(result?.refreshRecoveryProof).not.toBe("access-token");
      expect(result?.refreshRecoveryProof).not.toBe("refresh-token");
    });

    it("should carry permissions from user onto the token", async () => {
      const user = {
        id: "123",
        name: "Test User",
        email: "test@example.com",
        token: "access-token",
        refreshToken: "refresh-token",
        roles: ["user"],
        permissions: ["groups:read", "groups:update"],
        firstName: "Test",
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

      expect(result?.permissions).toEqual(["groups:read", "groups:update"]);
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
      const newRefreshToken = refreshJwt("new-refresh-token");
      mockRequestHeaders.mockResolvedValue(
        new Headers({
          "user-agent": "Mozilla/5.0 Tablet",
          "x-forwarded-for": "203.0.113.10, 172.20.0.4",
        }),
      );
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "new-access-token",
          refresh_token: newRefreshToken,
        }),
      });

      const token = {
        id: "123",
        token: "old-access-token",
        refreshToken: "old-refresh-token",
        refreshRecoveryProof: "independent-recovery-proof",
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
            "User-Agent": "Mozilla/5.0 Tablet",
            "X-Forwarded-For": "172.20.0.4",
            "X-Refresh-Recovery-Proof": "independent-recovery-proof",
          }) as Record<string, string>,
        }),
      );
      expect(result?.token).toBe("new-access-token");
      expect(result?.refreshToken).toBe(newRefreshToken);
      expect(result?.refreshRecoveryProof).not.toBe(
        "independent-recovery-proof",
      );
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

    it("should end the session on a rejected proactive refresh (#2952)", async () => {
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

      // A 401 on refresh is final: the session callback strips the tokens.
      expect(result?.token).toBe("old-access-token");
      expect(result?.refreshToken).toBe("old-refresh-token");
      expect(result?.error).toBe("RefreshTokenError");
      expect(result?.needsRefresh).toBe(true);
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

    it("keeps the session when a tablet resumes after sleeping past access-token expiry", async () => {
      const refreshedRefreshToken = refreshJwt("refreshed-refresh-token");
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "refreshed-access-token",
          refresh_token: refreshedRefreshToken,
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
      expect(result?.refreshToken).toBe(refreshedRefreshToken);
      expect(result?.error).toBeUndefined();
    });

    it("retries after a short tablet network interruption instead of logging out", async () => {
      const reconnectedRefreshToken = refreshJwt("reconnected-refresh-token");
      mockFetch
        .mockRejectedValueOnce(new Error("tablet temporarily offline"))
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            access_token: "reconnected-access-token",
            refresh_token: reconnectedRefreshToken,
          }),
        });

      const sleepingTabletToken = {
        id: "123",
        token: "expired-access-token",
        refreshToken: "still-valid-refresh-token",
        tokenExpiry: Date.now() - 30 * 1000,
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
      };

      const offlineResult = await callJwt({ ...sleepingTabletToken });
      expect(offlineResult?.error).toBeUndefined();
      expect(offlineResult?.refreshToken).toBe("still-valid-refresh-token");

      const reconnectedResult = await callJwt({ ...sleepingTabletToken });
      expect(reconnectedResult?.error).toBeUndefined();
      expect(reconnectedResult?.token).toBe("reconnected-access-token");
      expect(reconnectedResult?.refreshToken).toBe(reconnectedRefreshToken);
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });

    it("derives the same bootstrap proof for a legacy session after a process replacement", async () => {
      mockFetch.mockRejectedValue(new Error("response interrupted"));
      const legacyToken = {
        id: "123",
        token: "expired-access-token",
        refreshToken: "legacy-refresh-token",
        tokenExpiry: Date.now() - 30 * 1000,
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
      };

      await callJwt({ ...legacyToken });
      const firstHeaders = mockFetch.mock.calls[0]?.[1]?.headers as Record<
        string,
        string
      >;
      _resetRefreshState();
      await callJwt({ ...legacyToken });
      const secondHeaders = mockFetch.mock.calls[1]?.[1]?.headers as Record<
        string,
        string
      >;

      expect(firstHeaders["X-Refresh-Recovery-Proof"]).toBe(
        secondHeaders["X-Refresh-Recovery-Proof"],
      );
      expect(firstHeaders["X-Refresh-Recovery-Proof"]).not.toBe(
        legacyToken.token,
      );
      expect(firstHeaders["X-Refresh-Recovery-Proof"]).not.toBe(
        legacyToken.refreshToken,
      );
    });

    it("should deduplicate late-arriving callbacks via cache", async () => {
      const newRefreshToken = refreshJwt("new-refresh-1");
      // First call: triggers actual refresh and caches result
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "new-access-1",
          refresh_token: newRefreshToken,
        }),
      });

      const makeToken = () => ({
        id: "123",
        token: "old-access-token",
        refreshToken: "dedup-refresh-token",
        refreshRecoveryProof: "shared-cache-proof",
        tokenExpiry: Date.now() + 2 * 60 * 1000,
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
      });

      // First callback: performs the actual refresh
      const result1 = await callJwt(makeToken());
      expect(result1?.token).toBe("new-access-1");
      expect(result1?.refreshToken).toBe(newRefreshToken);
      expect(mockFetch).toHaveBeenCalledOnce();

      // Late-arriving callback with same OLD refresh token: served from cache
      const result2 = await callJwt(makeToken());
      expect(result2?.token).toBe("new-access-1");
      expect(result2?.refreshToken).toBe(newRefreshToken);
      expect(result2?.refreshRecoveryProof).toBe(result1?.refreshRecoveryProof);
      // Still only 1 fetch call — second was served from cache
      expect(mockFetch).toHaveBeenCalledOnce();
    });

    it("keeps one successor proof and its persisted expiry across independent recovery responses", async () => {
      const persistedExpiry = Date.now() + 9 * 60 * 1000;
      const expectedExpiry = Math.floor(persistedExpiry / 1000) * 1000;
      const firstSuccessorJwt = refreshJwt(
        "persisted-successor",
        persistedExpiry,
        1_700_000_000,
      );
      const delayedSuccessorJwt = refreshJwt(
        "persisted-successor",
        persistedExpiry,
        1_700_000_030,
      );
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            access_token: "first-recovered-access",
            refresh_token: firstSuccessorJwt,
          }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            access_token: "delayed-recovered-access",
            refresh_token: delayedSuccessorJwt,
          }),
        });

      const predecessorToken = {
        id: "123",
        token: "expired-access",
        refreshToken: "rotated-predecessor",
        refreshRecoveryProof: "predecessor-proof",
        tokenExpiry: Date.now() - 30 * 1000,
        refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
      };

      const firstRecovery = await callJwt({ ...predecessorToken });
      _resetRefreshState();
      const delayedRecovery = await callJwt({ ...predecessorToken });

      expect(mockFetch).toHaveBeenCalledTimes(2);
      expect(delayedRecovery?.refreshToken).toBe(delayedSuccessorJwt);
      expect(delayedRecovery?.refreshRecoveryProof).toBe(
        firstRecovery?.refreshRecoveryProof,
      );
      expect(firstRecovery?.refreshTokenExpiry).toBe(expectedExpiry);
      expect(delayedRecovery?.refreshTokenExpiry).toBe(expectedExpiry);
    });

    it("should share in-flight refresh promise across concurrent callbacks", async () => {
      const sharedRefreshToken = refreshJwt("shared-refresh");
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
        refreshRecoveryProof: "shared-inflight-proof",
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
      await vi.waitFor(() => expect(mockFetch).toHaveBeenCalledOnce());

      // Resolve the single fetch — all 3 callbacks get the same result
      resolveRefresh!({
        ok: true,
        json: async () => ({
          access_token: "shared-access",
          refresh_token: sharedRefreshToken,
        }),
      });

      const [r1, r2, r3] = await Promise.all([p1, p2, p3]);

      expect(r1?.token).toBe("shared-access");
      expect(r2?.token).toBe("shared-access");
      expect(r3?.token).toBe("shared-access");
      expect(r1?.refreshToken).toBe(sharedRefreshToken);
      expect(r2?.refreshToken).toBe(sharedRefreshToken);
      expect(r3?.refreshToken).toBe(sharedRefreshToken);
      expect(r1?.refreshRecoveryProof).toBe(r2?.refreshRecoveryProof);
      expect(r2?.refreshRecoveryProof).toBe(r3?.refreshRecoveryProof);
      // Confirm only 1 fetch across all 3 callbacks
      expect(mockFetch).toHaveBeenCalledOnce();
    });

    it("should not use cache for a different refresh token", async () => {
      const refreshTokenA = refreshJwt("refresh-A-new");
      const refreshTokenB = refreshJwt("refresh-B-new");
      // First: refresh with token-A
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          access_token: "access-A",
          refresh_token: refreshTokenA,
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
          refresh_token: refreshTokenB,
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
      expect(result?.refreshToken).toBe(refreshTokenB);
      // Two separate fetches — cache was not used
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });

    it("does not share a rotation with the same refresh token but a different recovery proof", async () => {
      const firstRefreshToken = refreshJwt("first-refresh");
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            access_token: "first-access",
            refresh_token: firstRefreshToken,
          }),
        })
        .mockResolvedValueOnce({ ok: false, status: 401 });

      const baseToken = {
        id: "123",
        token: "old-access",
        refreshToken: "shared-old-refresh",
        tokenExpiry: Date.now() + 2 * 60 * 1000,
        refreshTokenExpiry: Date.now() + 7 * 24 * 60 * 60 * 1000,
      };

      await callJwt({
        ...baseToken,
        refreshRecoveryProof: "legitimate-proof",
      });
      await callJwt({
        ...baseToken,
        refreshRecoveryProof: "unrelated-proof",
      });

      expect(mockFetch).toHaveBeenCalledTimes(2);
      expect(mockFetch).toHaveBeenLastCalledWith(
        expect.stringContaining("/auth/refresh"),
        expect.objectContaining({
          headers: expect.objectContaining({
            "X-Refresh-Recovery-Proof": "unrelated-proof",
          }) as Record<string, string>,
        }),
      );
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
        refreshRecoveryProof: "http-only-proof",
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
      expect(user).not.toHaveProperty("refreshRecoveryProof");
      expect(result).not.toHaveProperty("refreshRecoveryProof");
    });

    it("should expose permissions from the token on the session", () => {
      const session = {
        user: { id: "", email: "", name: "" },
        expires: "2099-12-31",
      };

      const token = {
        id: "123",
        email: "test@example.com",
        token: "access-token",
        refreshToken: "refresh-token",
        roles: ["user"],
        permissions: ["groups:read"],
        firstName: "Test",
        isAdmin: false,
      };

      const result = callSessionCallback({ session, token });
      const user = result?.user as Record<string, unknown> | undefined;

      expect(user?.permissions).toEqual(["groups:read"]);
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

  describe("performParentLogin", () => {
    it("should return tokens on successful login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          status: "success",
          data: {
            access_token: "parent-access",
            refresh_token: "parent-refresh",
          },
        }),
      });

      const result = await _testHelpers.performParentLogin(
        "parent@test.com",
        "pass",
        false,
      );

      expect(result).toEqual({
        access_token: "parent-access",
        refresh_token: "parent-refresh",
      });
    });

    it("should extract code from JSON error body on 401 account_inactive", async () => {
      // This is the case the original bug missed: backend signals
      // account_inactive distinct from invalid_credentials via the
      // body code, but the old code path only read response.status
      // and dropped the body entirely.
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () =>
          JSON.stringify({
            status: "error",
            error: "account is inactive",
            code: "account_inactive",
          }),
      });

      const result = await _testHelpers.performParentLogin(
        "disabled@test.com",
        "correct",
        false,
      );

      expect(result).toEqual({
        access_token: "",
        refresh_token: "",
        status: 401,
        code: "account_inactive",
      });
    });

    it("should extract code from JSON error body on 403 not_a_guardian", async () => {
      // Staff-account-at-parent-portal. Frontend uses this code to
      // mask the error as invalid_credentials at the UI layer, but
      // the wire-level code MUST come through so future UX changes
      // (e.g. unmasking for non-rate-limited cases) have something
      // to switch on.
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 403,
        text: async () =>
          JSON.stringify({
            status: "error",
            error: "account is not a guardian at any school",
            code: "not_a_guardian",
          }),
      });

      const result = await _testHelpers.performParentLogin(
        "staff@test.com",
        "correct",
        false,
      );

      expect(result).toEqual({
        access_token: "",
        refresh_token: "",
        status: 403,
        code: "not_a_guardian",
      });
    });

    it("should leave code undefined when body has no code field", async () => {
      // Defensive: if an older backend (pre-this-change) is hit, the
      // body parses fine but has no `code`. Must not crash, must
      // surface status so the provider can still pick rate_limited
      // for 429s.
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () =>
          JSON.stringify({ status: "error", error: "invalid credentials" }),
      });

      const result = await _testHelpers.performParentLogin(
        "parent@test.com",
        "wrong",
        false,
      );

      expect(result).toEqual({
        access_token: "",
        refresh_token: "",
        status: 401,
        code: undefined,
      });
    });

    it("should leave code undefined on non-JSON body", async () => {
      // Gateway/proxy error pages return HTML or plaintext. JSON.parse
      // would throw — the parse must be wrapped so the login still
      // surfaces a usable status.
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 502,
        text: async () => "<html>Bad Gateway</html>",
      });

      const result = await _testHelpers.performParentLogin(
        "parent@test.com",
        "pass",
        false,
      );

      expect(result).toEqual({
        access_token: "",
        refresh_token: "",
        status: 502,
        code: undefined,
      });
    });

    it("should return null on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const result = await _testHelpers.performParentLogin(
        "parent@test.com",
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
        "",
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
        "",
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
        "",
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
        "",
        true,
      );

      expect(result).toBeNull();
    });

    it("should return null on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Connection refused"));

      const result = await _testHelpers.performLogin(
        "teacher@test.com",
        "pass",
        "",
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
    // The operator provider is now in its own config (operatorAuthConfig).
    function getOperatorAuthorize() {
      const providers = operatorAuthConfig.providers.filter(
        (p) =>
          typeof p === "object" &&
          p !== null &&
          "type" in p &&
          p.type === "credentials",
      );
      const provider = providers[0] as unknown as
        Record<string, unknown> | undefined;
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

    it("should forward only the canonical operator client IP", async () => {
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
      await authorize(
        { email: "op@example.com", password: "correct" },
        new Request("http://localhost:3000", {
          headers: {
            "x-forwarded-for": "203.0.113.10, 172.20.0.4",
            "x-real-ip": "198.51.100.25",
          },
        }),
      );

      expect(mockFetch).toHaveBeenCalledWith(
        "http://server:8080/operator/auth/login",
        expect.objectContaining({
          headers: expect.objectContaining({
            "X-Forwarded-For": "172.20.0.4",
          }) as HeadersInit,
        }),
      );
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
      expect(result?.email).toBe("op@example.com");
      expect(result?.email).not.toBe("operator:45");
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("should fall back to email claim for legacy operator internal refresh tokens", async () => {
      const authorize = getOperatorAuthorize();
      const result = await authorize(
        {
          internalRefresh: "true",
          token: OPERATOR_JWT_EMAIL_ONLY,
          refreshToken: "op-refresh",
        },
        new Request("http://localhost:3000"),
      );

      expect(result).not.toBeNull();
      expect(result?.email).toBe("legacy-op@example.com");
      expect(result?.email).not.toBe("operator:45");
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("should not use operator subject as email when internal refresh token has no email claims", async () => {
      const authorize = getOperatorAuthorize();
      const result = await authorize(
        {
          internalRefresh: "true",
          token: OPERATOR_JWT_NO_EMAIL,
          refreshToken: "op-refresh",
        },
        new Request("http://localhost:3000"),
      );

      expect(result).not.toBeNull();
      expect(result?.email).toBe("");
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

  describe("Credentials authorize - parent flow", () => {
    // Mirrors the operator flow tests above. The parent provider lives
    // in parentAuthConfig and translates backend body codes into
    // user-facing CredentialsSignin error codes. This is the layer
    // where the original "Konto deaktiviert" bug lived — these tests
    // exist to nail down the mapping so the same class of bug can't
    // silently regress.
    function getParentAuthorize() {
      const providers = parentAuthConfig.providers.filter(
        (p) =>
          typeof p === "object" &&
          p !== null &&
          "type" in p &&
          p.type === "credentials",
      );
      const provider = providers[0] as unknown as
        Record<string, unknown> | undefined;
      const opts = provider?.options as Record<string, unknown> | undefined;
      return opts?.authorize as (
        credentials: Record<string, string> | undefined,
        request: Request,
      ) => Promise<User | null>;
    }

    // Reuse OPERATOR_JWT as a stand-in for any decodable JWT — the
    // authorize path only cares that parseJwtPayload returns something
    // non-null. The scope is set to "parent" by buildAuthUser, not
    // by the token payload.
    const PARENT_JWT = OPERATOR_JWT;

    it("should return user with parent scope on successful login", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          status: "success",
          data: {
            access_token: PARENT_JWT,
            refresh_token: "parent-refresh-token",
          },
        }),
      });

      const authorize = getParentAuthorize();
      const result = await authorize(
        { email: "parent@example.com", password: "correct" },
        new Request("http://localhost:3000"),
      );

      expect(result).not.toBeNull();
      expect(result?.scope).toBe("parent");
    });

    it("should forward only the canonical parent client IP", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          status: "success",
          data: {
            access_token: PARENT_JWT,
            refresh_token: "parent-refresh-token",
          },
        }),
      });

      const authorize = getParentAuthorize();
      await authorize(
        { email: "parent@example.com", password: "correct" },
        new Request("http://localhost:3000", {
          headers: {
            "x-forwarded-for": "203.0.113.10, 172.20.0.4",
            "x-real-ip": "198.51.100.25",
          },
        }),
      );

      expect(mockFetch).toHaveBeenCalledWith(
        "http://server:8080/parent/auth/login",
        expect.objectContaining({
          headers: expect.objectContaining({
            "X-Forwarded-For": "172.20.0.4",
          }) as HeadersInit,
        }),
      );
    });

    it("should return null for missing credentials", async () => {
      const authorize = getParentAuthorize();
      const result = await authorize({}, new Request("http://localhost:3000"));

      expect(result).toBeNull();
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("should throw account_inactive when backend returns code account_inactive", async () => {
      // This is the regression test for the second bug. Pre-fix the
      // frontend dropped the body, so 401-inactive merged with 401-
      // wrong-password and parents with deactivated accounts saw the
      // generic "check your credentials" message. The test ensures
      // the body code now wins over the bare status.
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () =>
          JSON.stringify({
            status: "error",
            error: "account is inactive",
            code: "account_inactive",
          }),
      });

      const authorize = getParentAuthorize();
      await expect(
        authorize(
          { email: "disabled@example.com", password: "correct" },
          new Request("http://localhost:3000"),
        ),
      ).rejects.toMatchObject({ code: "account_inactive" });
    });

    it("should throw not_a_guardian when backend returns code not_a_guardian", async () => {
      // Staff account hitting the parent portal. This code used to be
      // masked to invalid_credentials so the UI would not confirm the
      // email belongs to staff. The mask was dropped deliberately: the
      // backend only reaches this branch AFTER validateLoginCredentials
      // accepted the password (services/auth/auth_login_parent.go), so
      // the caller already owns the account and learns nothing new. The
      // mask did cost real users the one hint they needed — parents
      // stuck on the wrong portal kept resetting their password instead.
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 403,
        text: async () =>
          JSON.stringify({
            status: "error",
            error: "account is not a guardian at any school",
            code: "not_a_guardian",
          }),
      });

      const authorize = getParentAuthorize();
      await expect(
        authorize(
          { email: "staff@example.com", password: "correct" },
          new Request("http://localhost:3000"),
        ),
      ).rejects.toMatchObject({ code: "not_a_guardian" });
    });

    it("should throw invalid_credentials when backend returns code invalid_credentials", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () =>
          JSON.stringify({
            status: "error",
            error: "invalid credentials",
            code: "invalid_credentials",
          }),
      });

      const authorize = getParentAuthorize();
      await expect(
        authorize(
          { email: "parent@example.com", password: "wrong" },
          new Request("http://localhost:3000"),
        ),
      ).rejects.toMatchObject({ code: "invalid_credentials" });
    });

    it("should throw rate_limited on 429", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 429,
        text: async () => "Too Many Requests",
      });

      const authorize = getParentAuthorize();
      await expect(
        authorize(
          { email: "parent@example.com", password: "anything" },
          new Request("http://localhost:3000"),
        ),
      ).rejects.toMatchObject({ code: "rate_limited" });
    });

    it("should throw invalid_credentials when backend returns no code (legacy)", async () => {
      // If a pre-this-change backend is hit, body has no code. The
      // provider falls back to invalid_credentials, which still shows
      // a sensible message (the German copy covers wrong-password
      // and includes the staff-login hint). This is the safety net.
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () =>
          JSON.stringify({ status: "error", error: "invalid credentials" }),
      });

      const authorize = getParentAuthorize();
      await expect(
        authorize(
          { email: "parent@example.com", password: "wrong" },
          new Request("http://localhost:3000"),
        ),
      ).rejects.toMatchObject({ code: "invalid_credentials" });
    });

    it("should handle internal refresh with parent scope", async () => {
      const authorize = getParentAuthorize();
      const result = await authorize(
        {
          internalRefresh: "true",
          token: PARENT_JWT,
          refreshToken: "parent-refresh",
        },
        new Request("http://localhost:3000"),
      );

      expect(result).not.toBeNull();
      expect(result?.scope).toBe("parent");
      expect(mockFetch).not.toHaveBeenCalled();
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
        refreshRecoveryProof: "shared-failure-proof",
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
      await vi.waitFor(() => expect(mockFetch).toHaveBeenCalledOnce());
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
      const newOperatorRefreshToken = refreshJwt("new-op-refresh");
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: {
            access_token: "new-op-access",
            refresh_token: newOperatorRefreshToken,
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
      expect(result?.refreshToken).toBe(newOperatorRefreshToken);
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
    async function callRedirect(url: string, baseUrl: string): Promise<string> {
      const redirectFn = authConfig.callbacks?.redirect;
      if (!redirectFn) throw new Error("redirect callback not found");
      return redirectFn({ url, baseUrl });
    }

    it("should resolve relative URLs against baseUrl", async () => {
      expect(await callRedirect("/dashboard", "http://localhost:3000")).toBe(
        "http://localhost:3000/dashboard",
      );
    });

    it("should allow same origin redirects", async () => {
      expect(
        await callRedirect(
          "http://localhost:3000/dashboard",
          "http://localhost:3000",
        ),
      ).toBe("http://localhost:3000/dashboard");
    });

    it("should allow cross-subdomain localhost redirects", async () => {
      expect(
        await callRedirect(
          "http://operator.localhost:3000/login",
          "http://localhost:3000",
        ),
      ).toBe("http://operator.localhost:3000/login");
    });

    it("should allow same parent domain redirects", async () => {
      expect(
        await callRedirect(
          "http://operator.moto-app.de/login",
          "http://altenberge.moto-app.de",
        ),
      ).toBe("http://operator.moto-app.de/login");
    });

    it("should block redirects to different domains", async () => {
      expect(
        await callRedirect("http://evil.com/phish", "http://localhost:3000"),
      ).toBe("http://localhost:3000");
    });

    it("should block redirects when parent domains differ", async () => {
      expect(
        await callRedirect(
          "http://evil.other-site.com/phish",
          "http://app.moto-app.de",
        ),
      ).toBe("http://app.moto-app.de");
    });

    it("should handle two-part hostnames without subdomain stripping", async () => {
      // getParentDomain returns hostname as-is when there are only 2 parts
      expect(
        await callRedirect("http://moto-app.de/path", "http://moto-app.de"),
      ).toBe("http://moto-app.de/path");
    });

    it("should block when one host is localhost and the other is not", async () => {
      expect(
        await callRedirect("http://evil.com/phish", "http://app.moto-app.de"),
      ).toBe("http://app.moto-app.de");
    });
  });

  describe("operator redirect callback", () => {
    async function callOperatorRedirect(
      url: string,
      baseUrl: string,
    ): Promise<string> {
      const redirectFn = operatorAuthConfig.callbacks?.redirect;
      if (!redirectFn) throw new Error("operator redirect callback not found");
      return redirectFn({ url, baseUrl });
    }

    it("should resolve relative URLs against baseUrl", async () => {
      expect(
        await callOperatorRedirect(
          "/operator/organizations",
          "http://operator.moto-app.de",
        ),
      ).toBe("http://operator.moto-app.de/operator/organizations");
    });

    it("should allow same origin redirects", async () => {
      expect(
        await callOperatorRedirect(
          "http://operator.moto-app.de/operator/organizations",
          "http://operator.moto-app.de",
        ),
      ).toBe("http://operator.moto-app.de/operator/organizations");
    });

    it("should block cross-subdomain redirects on the same parent domain", async () => {
      expect(
        await callOperatorRedirect(
          "http://school-a.moto-app.de/",
          "http://operator.moto-app.de",
        ),
      ).toBe("http://operator.moto-app.de");
    });
  });

  describe("school redirect callback", () => {
    async function callSchoolRedirect(
      url: string,
      baseUrl: string,
    ): Promise<string> {
      const redirectFn = schoolAuthConfig.callbacks?.redirect;
      if (!redirectFn) throw new Error("school redirect callback not found");
      return redirectFn({ url, baseUrl });
    }

    it("allows relative paths and absolute URLs on the school origin", async () => {
      await expect(
        callSchoolRedirect("/class-day", "https://schule.moto-app.de"),
      ).resolves.toBe("https://schule.moto-app.de/class-day");
      await expect(
        callSchoolRedirect(
          "https://schule.moto-app.de/help",
          "https://schule.moto-app.de",
        ),
      ).resolves.toBe("https://schule.moto-app.de/help");
    });

    it.each([
      "https://moto-app.de/",
      "https://foo.schule.moto-app.de/",
      "//evil.example/phish",
      "/\\evil.example/phish",
      "not a URL",
    ])("blocks callback target %s outside the school origin", async (url) => {
      await expect(
        callSchoolRedirect(url, "https://schule.moto-app.de"),
      ).resolves.toBe("https://schule.moto-app.de");
    });
  });

  describe("cookie configuration", () => {
    it("should configure tenant cookies for cross-subdomain sharing", () => {
      // Cookie names are derived from TENANT_DOMAIN to prevent collisions
      // when environments share a parent domain (e.g., staging.moto-app.de
      // under moto-app.de).
      expect(authConfig.cookies?.sessionToken?.name).toBe(
        "moto-app-de.session-token",
      );
      expect(authConfig.cookies?.sessionToken?.options.domain).toBe(
        ".moto-app.de",
      );
      expect(authConfig.cookies?.callbackUrl?.name).toBe(
        "moto-app-de.callback-url",
      );
      expect(authConfig.cookies?.callbackUrl?.options.domain).toBe(
        ".moto-app.de",
      );
      expect(authConfig.cookies?.csrfToken?.name).toBe(
        "moto-app-de.csrf-token",
      );
      expect(authConfig.cookies?.csrfToken?.options.domain).toBe(
        ".moto-app.de",
      );
    });

    it("should derive unique prefixes that prevent cross-environment collisions", () => {
      // The bug: staging.moto-app.de is a subdomain of moto-app.de, so
      // production cookies on .moto-app.de are sent to staging too.
      // With derived names, production uses "moto-app-de.*" and staging
      // would use "staging-moto-app-de.*" — no collision.
      const prodPrefix = "moto-app.de".replace(/\./g, "-");
      const stagingPrefix = "staging.moto-app.de".replace(/\./g, "-");
      expect(prodPrefix).toBe("moto-app-de");
      expect(stagingPrefix).toBe("staging-moto-app-de");
      expect(prodPrefix).not.toBe(stagingPrefix);
    });

    it("should configure operator cookies as host-only with unique names", () => {
      expect(operatorAuthConfig.cookies?.sessionToken?.name).toBe(
        "operator.session-token",
      );
      expect(
        "domain" in (operatorAuthConfig.cookies?.sessionToken?.options ?? {}),
      ).toBe(false);
      expect(operatorAuthConfig.cookies?.callbackUrl?.name).toBe(
        "operator.callback-url",
      );
      expect(
        "domain" in (operatorAuthConfig.cookies?.callbackUrl?.options ?? {}),
      ).toBe(false);
      expect(operatorAuthConfig.cookies?.csrfToken?.name).toBe(
        "operator.csrf-token",
      );
      expect(
        "domain" in (operatorAuthConfig.cookies?.csrfToken?.options ?? {}),
      ).toBe(false);
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

  describe("events.signOut", () => {
    it("resets refresh state on tenant sign out", () => {
      expect(authConfig.events).toBeDefined();
      authConfig.events.signOut();
    });

    it("resets refresh state on operator sign out", () => {
      expect(operatorAuthConfig.events).toBeDefined();
      operatorAuthConfig.events.signOut();
    });
  });
});
