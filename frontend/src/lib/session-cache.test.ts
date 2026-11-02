/**
 * Tests for the getSession() dedupe cache (#2123).
 *
 * Every raw next-auth getSession() call is its own network round trip to
 * /api/auth/session; this cache collapses the parallel page-load fan-out into
 * one request. The staleness guard is the safety net for token refreshes: a
 * lookup that was already in flight when clearSessionCache() ran must not
 * write its stale result back into the cache afterwards.
 */
import { afterEach, describe, expect, it, vi, beforeEach } from "vitest";
import { mockSessionData } from "~/test/mocks/next-auth";

const mockGetSession = vi.fn();
const mockHandleAuthFailure = vi.fn();
vi.mock("next-auth/react", () => ({
  getSession: (...args: unknown[]) => mockGetSession(...args) as unknown,
}));
vi.mock("./auth-failure", () => ({
  handleAuthFailure: mockHandleAuthFailure,
}));

type SessionCacheModule = typeof import("./session-cache");

async function freshModule(): Promise<SessionCacheModule> {
  vi.resetModules();
  return import("./session-cache");
}

// The browser session carries no token (#5005); `id` tells sessions apart.
function session(id: string, tenantId = 1) {
  return mockSessionData({ user: { id, tenantId } });
}

describe("getCachedSession", () => {
  beforeEach(() => {
    mockGetSession.mockReset();
  });

  it("serves repeated sequential calls within the TTL from one getSession call", async () => {
    const { getCachedSession } = await freshModule();
    mockGetSession.mockResolvedValue(session("token-a"));

    const first = await getCachedSession();
    const second = await getCachedSession();

    expect(first?.user?.id).toBe("token-a");
    expect(second).toBe(first);
    expect(mockGetSession).toHaveBeenCalledTimes(1);
  });

  it("deduplicates concurrent callers into a single in-flight getSession", async () => {
    const { getCachedSession } = await freshModule();
    let resolveSession!: (value: unknown) => void;
    mockGetSession.mockReturnValue(
      new Promise((resolve) => {
        resolveSession = resolve;
      }),
    );

    const calls = [getCachedSession(), getCachedSession(), getCachedSession()];
    resolveSession(session("token-b"));
    const results = await Promise.all(calls);

    expect(mockGetSession).toHaveBeenCalledTimes(1);
    for (const result of results) {
      expect(result?.user?.id).toBe("token-b");
    }
  });

  it("refetches after the TTL expires", async () => {
    const { getCachedSession } = await freshModule();
    const now = vi.spyOn(Date, "now");
    now.mockReturnValue(1_000_000);
    mockGetSession.mockResolvedValueOnce(session("token-old"));
    await getCachedSession();

    now.mockReturnValue(1_000_000 + 10_001);
    mockGetSession.mockResolvedValueOnce(session("token-new"));
    const result = await getCachedSession();

    expect(result?.user?.id).toBe("token-new");
    expect(mockGetSession).toHaveBeenCalledTimes(2);
    now.mockRestore();
  });

  it("refetches after clearSessionCache", async () => {
    const { getCachedSession, clearSessionCache } = await freshModule();
    mockGetSession.mockResolvedValueOnce(session("token-old"));
    await getCachedSession();

    clearSessionCache();
    mockGetSession.mockResolvedValueOnce(session("token-new"));
    const result = await getCachedSession();

    expect(result?.user?.id).toBe("token-new");
    expect(mockGetSession).toHaveBeenCalledTimes(2);
  });

  it("clears API and log-shipping backoff with the session context", async () => {
    const { clearSessionCache } = await freshModule();
    const {
      isClientLogShippingPaused,
      pauseClientLogShipping,
      rateLimitBlockedError,
      recordRateLimit,
    } = await import("./rate-limit-backoff");

    recordRateLimit("17", "PATCH");
    pauseClientLogShipping("17");
    expect(rateLimitBlockedError("PATCH")).not.toBeNull();
    expect(isClientLogShippingPaused()).toBe(true);

    clearSessionCache();

    expect(rateLimitBlockedError("PATCH")).toBeNull();
    expect(isClientLogShippingPaused()).toBe(false);
  });

  it("does not let an in-flight lookup repopulate the cache across a clear", async () => {
    // Regression guard for the 401→refresh race: request A starts a session
    // lookup, a refresh clears the cache, then A resolves with the PRE-refresh
    // session. That result must not be cached — the next caller has to see the
    // post-refresh session, or retries go out with the dead token.
    const { getCachedSession, clearSessionCache } = await freshModule();
    let resolveStale!: (value: unknown) => void;
    mockGetSession.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveStale = resolve;
      }),
    );

    const staleLookup = getCachedSession();
    clearSessionCache();
    resolveStale(session("token-stale"));
    await staleLookup;

    mockGetSession.mockResolvedValueOnce(session("token-fresh"));
    const result = await getCachedSession();

    expect(result?.user?.id).toBe("token-fresh");
    expect(mockGetSession).toHaveBeenCalledTimes(2);
  });

  it("a stale in-flight lookup does not clobber the post-clear in-flight state", async () => {
    // Same race, but the next caller arrives while the stale lookup is STILL
    // pending: the stale finally-block must not null out the new lookup.
    const { getCachedSession, clearSessionCache } = await freshModule();
    let resolveStale!: (value: unknown) => void;
    let resolveFresh!: (value: unknown) => void;
    mockGetSession
      .mockReturnValueOnce(
        new Promise((resolve) => {
          resolveStale = resolve;
        }),
      )
      .mockReturnValueOnce(
        new Promise((resolve) => {
          resolveFresh = resolve;
        }),
      );

    const staleLookup = getCachedSession();
    clearSessionCache();
    const freshLookup = getCachedSession();

    resolveStale(session("token-stale"));
    await staleLookup;
    resolveFresh(session("token-fresh"));
    await freshLookup;

    // The fresh result must be the one cached now.
    const cached = await getCachedSession();
    expect(cached?.user?.id).toBe("token-fresh");
    expect(mockGetSession).toHaveBeenCalledTimes(2);
  });
});

describe("primeSessionCache", () => {
  beforeEach(() => {
    mockGetSession.mockReset();
  });

  it("serves the primed session without calling getSession", async () => {
    const { getCachedSession, primeSessionCache } = await freshModule();
    primeSessionCache(session("primed"));

    const result = await getCachedSession();

    expect(result?.user?.id).toBe("primed");
    expect(mockGetSession).not.toHaveBeenCalled();
  });

  it("wins over a lookup that was already in flight", async () => {
    const { getCachedSession, primeSessionCache } = await freshModule();
    let resolveSession!: (value: unknown) => void;
    mockGetSession.mockReturnValue(
      new Promise((resolve) => {
        resolveSession = resolve;
      }),
    );

    const stale = getCachedSession();
    primeSessionCache(session("primed"));
    resolveSession(session("stale"));
    await stale;

    const result = await getCachedSession();
    expect(result?.user?.id).toBe("primed");
    expect(mockGetSession).toHaveBeenCalledTimes(1);
  });

  it("is dropped by clearSessionCache", async () => {
    const { clearSessionCache, getCachedSession, primeSessionCache } =
      await freshModule();
    primeSessionCache(session("primed"));
    mockGetSession.mockResolvedValue(session("fresh"));

    clearSessionCache();
    const result = await getCachedSession();

    expect(result?.user?.id).toBe("fresh");
    expect(mockGetSession).toHaveBeenCalledTimes(1);
  });
});

describe("sessionFetch", () => {
  let originalFetch: typeof fetch;
  const mockFetch = vi.fn();

  beforeEach(() => {
    mockGetSession.mockReset();
    mockHandleAuthFailure.mockReset();
    originalFetch = globalThis.fetch;
    globalThis.fetch = mockFetch as typeof fetch;
    mockFetch.mockReset();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  // The BFF routes read the access token from the session cookie (auth()).
  // A Bearer header on top sends the same token twice and pushed large
  // sessions past Node's header limit (HTTP 431, #3918).
  it("sends the access token only in the session cookie, not as a header", async () => {
    const { sessionFetch } = await freshModule();
    mockGetSession.mockResolvedValue(session("token-a"));
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 200 }));

    await sessionFetch("/api/test", {
      method: "POST",
      headers: { "X-Custom": "1" },
    });

    expect(mockFetch).toHaveBeenCalledTimes(1);
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/test");
    expect(init.method).toBe("POST");
    expect(init.headers).toEqual({
      "Content-Type": "application/json",
      "X-Custom": "1",
    });
  });

  it("retries after a refresh without a Bearer header", async () => {
    const { sessionFetch } = await freshModule();
    mockGetSession
      .mockResolvedValueOnce(session("token-old"))
      .mockResolvedValueOnce(session("token-new"));
    mockHandleAuthFailure.mockResolvedValueOnce(true);
    mockFetch
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(new Response(null, { status: 200 }));

    const response = await sessionFetch("/api/test");

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    for (const [, init] of mockFetch.mock.calls as [string, RequestInit][]) {
      expect(init.headers).toEqual({ "Content-Type": "application/json" });
    }
  });

  it("normalizes a failed session lookup as unavailable", async () => {
    const { sessionFetch } = await freshModule();
    const { ApiError } = await import("./api-error");
    mockGetSession.mockRejectedValueOnce(new TypeError("Failed to fetch"));

    const error = await sessionFetch("/api/test").catch(
      (caught: unknown) => caught,
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 503,
      code: "general.unavailable",
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("refuses a session the callback stripped without sending the request", async () => {
    const { sessionFetch } = await freshModule();
    const { ApiError } = await import("./api-error");
    mockGetSession.mockResolvedValueOnce({
      ...session("token-a"),
      error: "RefreshTokenError",
    });

    const error = await sessionFetch("/api/test").catch(
      (caught: unknown) => caught,
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 401,
      code: "general.permission",
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("normalizes a missing session token as a permission failure", async () => {
    const { sessionFetch } = await freshModule();
    const { ApiError } = await import("./api-error");
    mockGetSession.mockResolvedValueOnce(null);

    const error = await sessionFetch("/api/test").catch(
      (caught: unknown) => caught,
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 401,
      code: "general.permission",
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("normalizes an initial transport failure as unavailable", async () => {
    const { sessionFetch } = await freshModule();
    const { ApiError } = await import("./api-error");
    mockGetSession.mockResolvedValue(session("token-a"));
    mockFetch.mockRejectedValueOnce(new TypeError("Failed to fetch"));

    const error = await sessionFetch("/api/test").catch(
      (caught: unknown) => caught,
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 503,
      code: "general.unavailable",
    });
  });

  it("normalizes a transport failure after refreshing the session", async () => {
    const { sessionFetch } = await freshModule();
    const { ApiError } = await import("./api-error");
    mockGetSession
      .mockResolvedValueOnce(session("token-old"))
      .mockResolvedValueOnce(session("token-new"));
    mockHandleAuthFailure.mockResolvedValueOnce(true);
    mockFetch
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockRejectedValueOnce(new TypeError("Failed to fetch"));

    const error = await sessionFetch("/api/test").catch(
      (caught: unknown) => caught,
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 503,
      code: "general.unavailable",
    });
  });

  it("normalizes an unrecoverable authentication failure as permission denied", async () => {
    const { sessionFetch } = await freshModule();
    const { ApiError } = await import("./api-error");
    mockGetSession.mockResolvedValueOnce(session("token-old"));
    mockHandleAuthFailure.mockResolvedValueOnce(false);
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 401 }));

    const error = await sessionFetch("/api/test").catch(
      (caught: unknown) => caught,
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 401,
      code: "general.permission",
    });
  });

  it("normalizes a missing token after a successful refresh as permission denied", async () => {
    const { sessionFetch } = await freshModule();
    const { ApiError } = await import("./api-error");
    mockGetSession
      .mockResolvedValueOnce(session("token-old"))
      .mockResolvedValueOnce(null);
    mockHandleAuthFailure.mockResolvedValueOnce(true);
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 401 }));

    const error = await sessionFetch("/api/test").catch(
      (caught: unknown) => caught,
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 401,
      code: "general.permission",
    });
  });
});

// #4989: the tenant SessionProvider re-primes the cache with each answer.
// Losing the sign-in must never leave a signed-in session behind, and a
// primed session keeps the cache's 10 s window.
describe("primeSessionCache when the sign-in ends or changes", () => {
  let originalFetch: typeof fetch;
  const mockFetch = vi.fn();
  let now: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    mockGetSession.mockReset();
    mockHandleAuthFailure.mockReset();
    originalFetch = globalThis.fetch;
    globalThis.fetch = mockFetch as typeof fetch;
    mockFetch.mockReset();
    now = vi.spyOn(Date, "now").mockReturnValue(1_000_000);
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    now.mockRestore();
  });

  // #5130: the BFF refuses a session of another school than the request's
  // subdomain, so a primed session may stay until shortly after the
  // provider's next poll (4 min) instead of the 10 s window of #5116.
  it("keeps a primed session until shortly after the provider's next poll", async () => {
    const { getCachedSession, primeSessionCache } = await freshModule();
    primeSessionCache(session("primed", 1));
    mockGetSession.mockResolvedValueOnce(session("primed", 2));

    now.mockReturnValue(1_000_000 + 10_001);
    expect((await getCachedSession())?.user?.tenantId).toBe(1);
    expect(mockGetSession).not.toHaveBeenCalled();

    now.mockReturnValue(1_000_000 + 270_001);
    expect((await getCachedSession())?.user?.tenantId).toBe(2);
    expect(mockGetSession).toHaveBeenCalledTimes(1);
  });

  // #5130, replaces the 10 s window of #5116: a school switch on another
  // subdomain drops this tab's session with the first refused request.
  it("drops a primed session once the BFF refuses another school's session", async () => {
    const { getCachedSession, primeSessionCache } = await freshModule();
    const { apiErrorFromBody } = await import("./api-error");
    primeSessionCache(session("primed", 1));
    mockGetSession.mockResolvedValueOnce(session("primed", 2));

    // Anker: the primed session serves without a lookup until the refusal.
    expect((await getCachedSession())?.user?.tenantId).toBe(1);
    expect(mockGetSession).not.toHaveBeenCalled();

    apiErrorFromBody("Request refused", 409, {
      code: "identity.session_tenant_mismatch",
    });

    expect((await getCachedSession())?.user?.tenantId).toBe(2);
    expect(mockGetSession).toHaveBeenCalledTimes(1);
  });

  it("never serves a session whose own end has passed", async () => {
    const { getCachedSession, primeSessionCache } = await freshModule();
    primeSessionCache(
      mockSessionData({
        user: { id: "ending" },
        expires: new Date(1_000_000 + 5_000).toISOString(),
      }),
    );
    mockGetSession.mockResolvedValueOnce(null);

    now.mockReturnValue(1_000_000 + 5_000);
    const result = await getCachedSession();

    expect(result).toBeNull();
    expect(mockGetSession).toHaveBeenCalledTimes(1);
  });

  // Abmelden: the provider answers without a session.
  it("drops the signed-in session when the provider loses the sign-in", async () => {
    const { getCachedSession, primeSessionCache, sessionFetch } =
      await freshModule();
    primeSessionCache(session("signed-in"));

    primeSessionCache(null);
    mockGetSession.mockResolvedValue(null);

    expect(await getCachedSession()).toBeNull();
    await expect(sessionFetch("/api/test")).rejects.toMatchObject({
      status: 401,
      code: "general.permission",
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("does not let a lookup from before the logout bring the session back", async () => {
    const { getCachedSession, primeSessionCache } = await freshModule();
    let resolveStale!: (value: unknown) => void;
    mockGetSession.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveStale = resolve;
      }),
    );
    const stale = getCachedSession();

    primeSessionCache(null);
    resolveStale(session("before-logout"));
    await stale;
    mockGetSession.mockResolvedValueOnce(null);

    expect(await getCachedSession()).toBeNull();
    expect(mockGetSession).toHaveBeenCalledTimes(2);
  });

  // Kontowechsel und Schulwechsel.
  it("replaces the previous account's session with the provider's next answer", async () => {
    const { getCachedSession, primeSessionCache } = await freshModule();
    primeSessionCache(session("account-a", 1));

    primeSessionCache(session("account-b", 1));
    expect((await getCachedSession())?.user?.id).toBe("account-b");

    primeSessionCache(session("account-b", 2));
    const result = await getCachedSession();
    expect(result?.user?.id).toBe("account-b");
    expect(result?.user?.tenantId).toBe(2);
    expect(mockGetSession).not.toHaveBeenCalled();
  });

  it("asks the session route after a switch flow cleared the cache", async () => {
    const { clearSessionCache, getCachedSession, primeSessionCache } =
      await freshModule();
    primeSessionCache(session("account-a", 1));
    mockGetSession.mockResolvedValueOnce(session("account-a", 2));

    clearSessionCache();
    const result = await getCachedSession();

    expect(result?.user?.tenantId).toBe(2);
    expect(mockGetSession).toHaveBeenCalledTimes(1);
  });

  // Token-Erneuerung: a 401 renews the session; the renewed one stays.
  it("keeps the renewed session after a 401 and retries once", async () => {
    const { getCachedSession, primeSessionCache, sessionFetch } =
      await freshModule();
    primeSessionCache(session("before-refresh"));
    mockHandleAuthFailure.mockResolvedValueOnce(true);
    mockGetSession.mockResolvedValueOnce(session("renewed"));
    mockFetch
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(new Response(null, { status: 200 }));

    const response = await sessionFetch("/api/test");

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect((await getCachedSession())?.user?.id).toBe("renewed");
    expect(mockGetSession).toHaveBeenCalledTimes(1);
  });

  // Abgelaufene oder widerrufene Session: one failed refresh, no loop.
  it("asks again after a revoked session and stops there", async () => {
    const { getCachedSession, primeSessionCache, sessionFetch } =
      await freshModule();
    primeSessionCache(session("revoked"));
    mockHandleAuthFailure.mockResolvedValueOnce(false);
    mockFetch.mockResolvedValue(new Response(null, { status: 401 }));
    const ended = mockSessionData({ user: { id: "revoked" } });
    Object.assign(ended, { error: "RefreshTokenError" });
    mockGetSession.mockResolvedValue(ended);

    await expect(sessionFetch("/api/test")).rejects.toMatchObject({
      status: 401,
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockHandleAuthFailure).toHaveBeenCalledTimes(1);

    // The next request asks the session route, learns the session is gone
    // and does not send anything.
    await expect(sessionFetch("/api/test")).rejects.toMatchObject({
      status: 401,
      code: "general.permission",
    });
    // The provider hears the same answer (next-auth's echo of getSession);
    // the cache keeps it instead of asking again for every request.
    primeSessionCache(ended);
    await getCachedSession();
    await expect(sessionFetch("/api/test")).rejects.toMatchObject({
      status: 401,
    });

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockHandleAuthFailure).toHaveBeenCalledTimes(1);
    expect(mockGetSession).toHaveBeenCalledTimes(1);
  });
});
