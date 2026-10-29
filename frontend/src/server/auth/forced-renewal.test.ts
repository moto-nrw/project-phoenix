import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { Session, User } from "next-auth";
import type { JWT } from "next-auth/jwt";
import { tenantAuthConfig as authConfig } from "./tenant-config";
import { _resetRefreshState, withForcedSessionRenewal } from "./shared";

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "1h",
    NEXTAUTH_SECRET: "test-auth-secret-with-sufficient-entropy",
  },
}));

const mockFetch = vi.fn();

function jwt(payload: Record<string, unknown>) {
  const encode = (value: unknown) =>
    Buffer.from(JSON.stringify(value)).toString("base64url");
  return `${encode({ alg: "HS256", typ: "JWT" })}.${encode({
    exp: Math.floor(Date.now() / 1000) + 60 * 60,
    ...payload,
  })}.test`;
}

function callJwt(token: Record<string, unknown>) {
  return authConfig.callbacks?.jwt?.({
    token,
    user: undefined as unknown as User,
    account: null,
    profile: undefined,
    trigger: undefined,
    isNewUser: false,
    session: undefined,
  });
}

function callSession(token: Record<string, unknown>) {
  return authConfig.callbacks?.session?.({
    session: { user: {}, expires: "2099-01-01T00:00:00.000Z" } as Session,
    token: token as JWT,
  } as never) as Session & { error?: string };
}

/** A session whose access token is far from its expiry. */
function liveToken(id: string) {
  return {
    id,
    token: `access-${id}`,
    refreshToken: `refresh-${id}`,
    tokenExpiry: Date.now() + 14 * 60 * 1000,
    refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
  };
}

/**
 * #5194: the backend refuses an unexpired access token once its family is
 * revoked. After such a 401 the session renews at once; a revoked family
 * ends the session instead of being presented again until it expires.
 */
describe("JWT callback — forced renewal after a backend 401", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockFetch);
    _resetRefreshState();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("keeps an unexpired session untouched outside a forced renewal", async () => {
    const result = await callJwt(liveToken("plain"));

    expect(mockFetch).not.toHaveBeenCalled();
    expect(result?.token).toBe("access-plain");
  });

  it("ends the session when the revoked family refuses its refresh", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

    const result = await withForcedSessionRenewal(async () =>
      callJwt(liveToken("revoked")),
    );

    expect(mockFetch).toHaveBeenCalledOnce();
    expect(mockFetch.mock.calls[0]?.[0]).toBe(
      "http://server:8080/auth/refresh",
    );
    expect(result?.error).toBe("RefreshTokenError");
    const session = callSession(result as Record<string, unknown>);
    expect(session.error).toBe("RefreshTokenError");
    expect((session.user as { token?: string }).token).toBe("");
  });

  it("ends a written-back copy of the refused cookie without a backend call", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });
    await withForcedSessionRenewal(async () => callJwt(liveToken("late")));
    mockFetch.mockClear();

    // The same cookie arrives again from a late response, outside a renewal.
    const result = await callJwt(liveToken("late"));

    expect(mockFetch).not.toHaveBeenCalled();
    expect(result?.error).toBe("RefreshTokenError");
  });

  it("continues a live family with fresh tokens", async () => {
    const refreshToken = jwt({ id: 1, token: "rotated" });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        access_token: "access-new",
        refresh_token: refreshToken,
      }),
    });

    const result = await withForcedSessionRenewal(async () =>
      callJwt(liveToken("live")),
    );

    expect(result?.token).toBe("access-new");
    expect(result?.refreshToken).toBe(refreshToken);
    expect(result?.error).toBeUndefined();
  });

  it("keeps the session on a transient refresh failure", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 503 });

    const result = await withForcedSessionRenewal(async () =>
      callJwt(liveToken("transient")),
    );

    expect(result?.token).toBe("access-transient");
    expect(result?.error).toBeUndefined();
  });

  it("ends a preview whose admin family is revoked, with its audit end", async () => {
    mockFetch.mockImplementation(async (url: string) =>
      url.endsWith("/auth/staff-preview/end")
        ? { ok: true, status: 200 }
        : { ok: false, status: 401 },
    );
    const preview = {
      ...liveToken("admin"),
      token: "preview-access",
      tokenExpiry: Date.now() + 14 * 60 * 1000,
      previewAdminToken: "admin-access",
      previewAdminTokenExpiry: Date.now() + 14 * 60 * 1000,
      previewAdminId: "admin",
      previewTargetAccountId: "77",
      previewTargetName: "Erika Beispiel",
    };

    const result = await withForcedSessionRenewal(async () => callJwt(preview));

    const urls = mockFetch.mock.calls.map(([url]) => url as string);
    expect(urls).toEqual([
      "http://server:8080/auth/refresh",
      "http://server:8080/auth/staff-preview/end",
    ]);
    expect(result?.error).toBe("RefreshTokenError");
    expect(result?.previewTargetAccountId).toBeUndefined();
  });
});
