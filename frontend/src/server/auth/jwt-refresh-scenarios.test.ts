import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { tenantAuthConfig as authConfig } from "./tenant-config";
import { _resetRefreshState } from "./shared";
import type { User } from "next-auth";

// Mock ~/env
vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "1h",
    NEXTAUTH_SECRET: "test-auth-secret-with-sufficient-entropy",
  },
}));

// Mock fetch globally
const mockFetch = vi.fn();

function refreshJwt(token: string) {
  const header = Buffer.from(
    JSON.stringify({ alg: "HS256", typ: "JWT" }),
  ).toString("base64url");
  const payload = Buffer.from(
    JSON.stringify({
      id: 123,
      token,
      exp: Math.floor(Date.now() / 1000) + 60 * 60,
    }),
  ).toString("base64url");
  return `${header}.${payload}.test`;
}

/**
 * Scenario-based stress tests for the JWT callback in NextAuth config.
 *
 * These tests model real user journeys (idle return, concurrent 401s,
 * expired refresh tokens, network failures) to verify the callback
 * behaves correctly under all conditions.
 */
describe("JWT callback — refresh scenarios", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockFetch);
    _resetRefreshState();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // Helper: invoke the JWT callback with a given token (no user = subsequent call)
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

  // ── Scenario 1: Fresh token, no refresh needed ─────────────────────
  it("scenario 1: fresh token — no fetch, token unchanged", async () => {
    const token = {
      id: "1",
      token: "access-ok",
      refreshToken: "refresh-ok",
      tokenExpiry: Date.now() + 30 * 60 * 1000, // 30 min from now
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result = await callJwt(token);

    expect(mockFetch).not.toHaveBeenCalled();
    expect(result?.token).toBe("access-ok");
    expect(result?.refreshToken).toBe("refresh-ok");
    expect(result?.error).toBeUndefined();
  });

  // ── Scenario 2: Pre-expiry window (happy path) ────────────────────
  it("scenario 2: pre-expiry window — fetch called, tokens updated", async () => {
    const newRefreshToken = refreshJwt("new-refresh");
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        access_token: "new-access",
        refresh_token: newRefreshToken,
      }),
    });

    const token = {
      id: "2",
      token: "old-access",
      refreshToken: "old-refresh",
      tokenExpiry: Date.now() + 3 * 60 * 1000, // 3 min from now (inside 5-min buffer)
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result = await callJwt(token);

    expect(mockFetch).toHaveBeenCalledOnce();
    expect(result?.token).toBe("new-access");
    expect(result?.refreshToken).toBe(newRefreshToken);
    expect(result?.error).toBeUndefined();
    // tokenExpiry should be reset to ~now + accessTokenExpiry (15 min)
    expect(result?.tokenExpiry).toBeGreaterThan(Date.now() + 14 * 60 * 1000);
  });

  // ── Scenario 3: Token expired 30s ago (idle return) ───────────────
  it("scenario 3: token expired 30s ago — fetch called, tokens refreshed", async () => {
    const refreshedRefreshToken = refreshJwt("refreshed-refresh");
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        access_token: "refreshed-access",
        refresh_token: refreshedRefreshToken,
      }),
    });

    const token = {
      id: "3",
      token: "expired-access",
      refreshToken: "still-valid-refresh",
      tokenExpiry: Date.now() - 30 * 1000, // expired 30s ago
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result = await callJwt(token);

    expect(mockFetch).toHaveBeenCalledOnce();
    expect(result?.token).toBe("refreshed-access");
    expect(result?.refreshToken).toBe(refreshedRefreshToken);
    expect(result?.error).toBeUndefined();
  });

  // ── Scenario 4: Token expired 30min ago (longer idle) ─────────────
  it("scenario 4: token expired 30min ago — fetch called, tokens refreshed", async () => {
    const refreshedRefreshToken = refreshJwt("refreshed-refresh-long");
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        access_token: "refreshed-access-long",
        refresh_token: refreshedRefreshToken,
      }),
    });

    const token = {
      id: "4",
      token: "very-expired-access",
      refreshToken: "still-valid-refresh",
      tokenExpiry: Date.now() - 30 * 60 * 1000, // expired 30 min ago
      refreshTokenExpiry: Date.now() + 30 * 60 * 1000, // still valid
    };

    const result = await callJwt(token);

    expect(mockFetch).toHaveBeenCalledOnce();
    expect(result?.token).toBe("refreshed-access-long");
    expect(result?.refreshToken).toBe(refreshedRefreshToken);
    expect(result?.error).toBeUndefined();
  });

  // ── Scenario 5: Both tokens expired (legitimate logout) ──────────
  it("scenario 5: both tokens expired — no fetch, error = RefreshTokenExpired", async () => {
    const token = {
      id: "5",
      token: "dead-access",
      refreshToken: "dead-refresh",
      tokenExpiry: Date.now() - 60 * 60 * 1000, // expired 1 hr ago
      refreshTokenExpiry: Date.now() - 1000, // expired 1s ago
    };

    const result = await callJwt(token);

    expect(mockFetch).not.toHaveBeenCalled();
    expect(result?.error).toBe("RefreshTokenExpired");
    expect(result?.needsRefresh).toBe(true);
  });

  // ── Scenario 6: Refresh token expired, access still valid ─────────
  it("scenario 6: refresh token expired, access still valid — error = RefreshTokenExpired", async () => {
    const token = {
      id: "6",
      token: "good-access",
      refreshToken: "dead-refresh",
      tokenExpiry: Date.now() + 30 * 60 * 1000, // access still fine
      refreshTokenExpiry: Date.now() - 1000, // refresh expired
    };

    const result = await callJwt(token);

    expect(mockFetch).not.toHaveBeenCalled();
    expect(result?.error).toBe("RefreshTokenExpired");
  });

  // ── Scenario 7: Backend refresh returns 401 (stolen token) ────────
  it("scenario 7: backend 401 — token unchanged, no error set", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
    });

    const token = {
      id: "7",
      token: "old-access",
      refreshToken: "maybe-stolen-refresh",
      tokenExpiry: Date.now() + 3 * 60 * 1000,
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result = await callJwt(token);

    expect(mockFetch).toHaveBeenCalledOnce();
    expect(result?.token).toBe("old-access");
    expect(result?.refreshToken).toBe("maybe-stolen-refresh");
    expect(result?.error).toBeUndefined();
  });

  // ── Scenario 8: Backend refresh network timeout ───────────────────
  it("scenario 8: network timeout (AbortError) — token unchanged, no error set", async () => {
    const abortError = new DOMException(
      "The operation was aborted",
      "AbortError",
    );
    mockFetch.mockRejectedValueOnce(abortError);

    const token = {
      id: "8",
      token: "old-access",
      refreshToken: "old-refresh",
      tokenExpiry: Date.now() + 3 * 60 * 1000,
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result = await callJwt(token);

    expect(mockFetch).toHaveBeenCalledOnce();
    expect(result?.token).toBe("old-access");
    expect(result?.error).toBeUndefined();
  });

  // ── Scenario 9: Backend refresh returns malformed JSON ────────────
  it("scenario 9: malformed JSON — token unchanged, no error set", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => {
        throw new SyntaxError("Unexpected token < in JSON");
      },
    });

    const token = {
      id: "9",
      token: "old-access",
      refreshToken: "old-refresh",
      tokenExpiry: Date.now() + 3 * 60 * 1000,
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result = await callJwt(token);

    expect(mockFetch).toHaveBeenCalledOnce();
    // Malformed JSON throws inside the IIFE → caught → returns null → token unchanged
    expect(result?.token).toBe("old-access");
    expect(result?.error).toBeUndefined();
  });

  // ── Scenario 10: No infinite loop after successful refresh ────────
  it("scenario 10: after refresh, second callback does NOT re-fetch", async () => {
    const freshRefreshToken = refreshJwt("fresh-refresh");
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        access_token: "fresh-access",
        refresh_token: freshRefreshToken,
      }),
    });

    // First call: inside pre-expiry window → triggers refresh
    const token1 = {
      id: "10",
      token: "old-access",
      refreshToken: "old-refresh",
      tokenExpiry: Date.now() + 3 * 60 * 1000,
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result1 = await callJwt(token1);
    expect(mockFetch).toHaveBeenCalledOnce();
    expect(result1?.token).toBe("fresh-access");

    // Second call: tokenExpiry was reset to now + 15 min (well outside buffer)
    const result2 = await callJwt({
      ...token1,
      token: result1?.token,
      refreshToken: result1?.refreshToken,
      tokenExpiry: result1?.tokenExpiry,
      refreshTokenExpiry: result1?.refreshTokenExpiry,
    });

    // Should NOT trigger another fetch
    expect(mockFetch).toHaveBeenCalledOnce(); // still just the first
    expect(result2?.token).toBe("fresh-access");
    expect(result2?.error).toBeUndefined();
  });

  // ── Scenario 11: Missing refreshToken field ───────────────────────
  it("scenario 11: no refreshToken — no fetch, token unchanged", async () => {
    const token = {
      id: "11",
      token: "access-ok",
      // refreshToken deliberately missing
      tokenExpiry: Date.now() + 3 * 60 * 1000,
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result = await callJwt(token);

    expect(mockFetch).not.toHaveBeenCalled();
    expect(result?.token).toBe("access-ok");
    expect(result?.error).toBeUndefined();
  });

  // ── Scenario 12: Missing tokenExpiry field ────────────────────────
  it("scenario 12: no tokenExpiry — no fetch, token unchanged", async () => {
    const token = {
      id: "12",
      token: "access-ok",
      refreshToken: "refresh-ok",
      // tokenExpiry deliberately missing
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result = await callJwt(token);

    expect(mockFetch).not.toHaveBeenCalled();
    expect(result?.token).toBe("access-ok");
    expect(result?.error).toBeUndefined();
  });

  // ── Scenario 13: Post-expiry refresh failure (backend 401) ─────────
  it("scenario 13: post-expiry + backend 401 — error = RefreshTokenError", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
    });

    const token = {
      id: "13",
      token: "expired-access",
      refreshToken: "valid-refresh",
      tokenExpiry: Date.now() - 30 * 1000, // expired 30s ago
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result = await callJwt(token);

    expect(mockFetch).toHaveBeenCalledOnce();
    expect(result?.token).toBe("expired-access"); // unchanged
    expect(result?.error).toBe("RefreshTokenError");
    expect(result?.needsRefresh).toBe(true);
  });

  // ── Scenario 14: Post-expiry network timeout remains retryable ─────
  it("scenario 14: post-expiry + timeout preserves refresh session", async () => {
    const abortError = new DOMException(
      "The operation was aborted",
      "AbortError",
    );
    mockFetch.mockRejectedValueOnce(abortError);

    const token = {
      id: "14",
      token: "expired-access",
      refreshToken: "valid-refresh",
      tokenExpiry: Date.now() - 30 * 1000, // expired 30s ago
      refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
    };

    const result = await callJwt(token);

    expect(mockFetch).toHaveBeenCalledOnce();
    expect(result?.token).toBe("expired-access"); // unchanged
    expect(result?.refreshToken).toBe("valid-refresh");
    expect(result?.error).toBeUndefined();
    expect(result?.needsRefresh).toBeUndefined();
  });
});
