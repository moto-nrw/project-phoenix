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

function session(token: string, tenantId = 1) {
  return mockSessionData({ user: { token, tenantId } });
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

    expect(first?.user?.token).toBe("token-a");
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
      expect(result?.user?.token).toBe("token-b");
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

    expect(result?.user?.token).toBe("token-new");
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

    expect(result?.user?.token).toBe("token-new");
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

    expect(result?.user?.token).toBe("token-fresh");
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
    expect(cached?.user?.token).toBe("token-fresh");
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

    expect(result?.user?.token).toBe("primed");
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
    expect(result?.user?.token).toBe("primed");
    expect(mockGetSession).toHaveBeenCalledTimes(1);
  });

  it("is dropped by clearSessionCache", async () => {
    const { clearSessionCache, getCachedSession, primeSessionCache } =
      await freshModule();
    primeSessionCache(session("primed"));
    mockGetSession.mockResolvedValue(session("fresh"));

    clearSessionCache();
    const result = await getCachedSession();

    expect(result?.user?.token).toBe("fresh");
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
