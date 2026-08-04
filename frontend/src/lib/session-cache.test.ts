/**
 * Tests for the getSession() dedupe cache (#2123).
 *
 * Every raw next-auth getSession() call is its own network round trip to
 * /api/auth/session; this cache collapses the parallel page-load fan-out into
 * one request. The generation guard is the safety net for token refreshes: a
 * lookup that was already in flight when clearSessionCache() ran must not
 * write its stale result back into the cache afterwards.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";

const mockGetSession = vi.fn();
vi.mock("next-auth/react", () => ({
  getSession: (...args: unknown[]) => mockGetSession(...args) as unknown,
}));

type SessionCacheModule = typeof import("./session-cache");

async function freshModule(): Promise<SessionCacheModule> {
  vi.resetModules();
  return import("./session-cache");
}

function session(token: string, tenantId = 1) {
  return { user: { token, tenantId } };
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
