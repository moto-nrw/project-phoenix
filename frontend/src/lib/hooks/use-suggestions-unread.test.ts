import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

// Use vi.hoisted for mock values referenced in vi.mock
const { mockUseSession, mockFetchUnreadCount } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockFetchUnreadCount: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: (): ReturnType<typeof mockUseSession> => mockUseSession(),
}));

vi.mock("~/lib/suggestions-api", () => ({
  fetchUnreadCount: mockFetchUnreadCount,
}));

// The hook prefixes its cache key with the active tenant slug (per-tenant
// metadata must not leak across a tenant switch). Pin the slug so the resolved
// key is deterministic; CACHE_KEY mirrors what the hook builds.
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: (): string => "testschool",
}));

import { useSuggestionsUnread } from "./use-suggestions-unread";

const CACHE_KEY = "suggestions_unread_count:testschool";

interface CachedData {
  count: number;
  timestamp: number;
}

describe("useSuggestionsUnread", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns 0 count when not authenticated", async () => {
    mockUseSession.mockReturnValue({ status: "unauthenticated" });

    const { result } = renderHook(() => useSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.unreadCount).toBe(0);
    expect(mockFetchUnreadCount).not.toHaveBeenCalled();
  });

  it("fetches unread count when authenticated", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockResolvedValueOnce(5);

    const { result } = renderHook(() => useSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.unreadCount).toBe(5);
    expect(mockFetchUnreadCount).toHaveBeenCalled();
  });

  it("uses cached count if available", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockResolvedValueOnce(10);

    // Set cached data
    const cachedData = {
      count: 3,
      timestamp: Date.now(),
    };
    localStorage.setItem(CACHE_KEY, JSON.stringify(cachedData));

    const { result } = renderHook(() => useSuggestionsUnread());

    // Should immediately show cached count
    expect(result.current.unreadCount).toBe(3);

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // Should eventually update with fresh data
    await waitFor(() => {
      expect(result.current.unreadCount).toBe(10);
    });
  });

  it("ignores expired cache", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockResolvedValueOnce(7);

    // Set expired cached data (older than 1 minute)
    const expiredData = {
      count: 3,
      timestamp: Date.now() - 61 * 1000,
    };
    localStorage.setItem(CACHE_KEY, JSON.stringify(expiredData));

    const { result } = renderHook(() => useSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.unreadCount).toBe(7);
  });

  it("handles fetch errors silently", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockRejectedValueOnce(new Error("Network error"));

    const { result } = renderHook(() => useSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.unreadCount).toBe(0);
  });

  it("refresh function updates count", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockResolvedValueOnce(5).mockResolvedValueOnce(8);

    const { result } = renderHook(() => useSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.unreadCount).toBe(5);
    });

    await result.current.refresh();

    await waitFor(() => {
      expect(result.current.unreadCount).toBe(8);
    });
  });

  it("skipCache parameter forces fresh fetch", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    // The shared useUnreadCount hook honors a forced refresh that lands while
    // the mount fetch is still in flight by draining it into a second fetch
    // (so a cache-busting event can't be dropped). Return the fresh value for
    // every call so that queued second fetch yields the same count, not
    // undefined; the assertion below still verifies the forced refresh wins.
    mockFetchUnreadCount.mockResolvedValue(10);

    // Set cached data
    localStorage.setItem(
      CACHE_KEY,
      JSON.stringify({ count: 3, timestamp: Date.now() }),
    );

    const { result } = renderHook(() => useSuggestionsUnread());

    await result.current.refresh(true);

    await waitFor(() => {
      expect(result.current.unreadCount).toBe(10);
    });
  });

  it("drains a forced refresh that lands mid-flight into a second fetch", async () => {
    // Documents an intentional behavior change from the old hand-rolled hook: the
    // shared useUnreadCount honors a cache-busting refresh that arrives WHILE the
    // mount fetch is still in flight by re-running the fetcher once it resolves,
    // instead of dropping it. One extra request in a rare race is the price of the
    // badge never sticking on a stale pre-event count.
    mockUseSession.mockReturnValue({ status: "authenticated" });
    let resolveMount: (n: number) => void = () => undefined;
    const mountFetch = new Promise<number>((resolve) => {
      resolveMount = resolve;
    });
    mockFetchUnreadCount
      .mockReturnValueOnce(mountFetch) // mount fetch hangs until we release it
      .mockResolvedValue(9); // the drained re-fetch

    const { result } = renderHook(() => useSuggestionsUnread());

    // The mount fetch is in flight; a forced refresh now queues (it can't run
    // concurrently) and must be drained after the mount fetch resolves.
    void result.current.refresh(true);
    resolveMount(4);

    await waitFor(() => {
      expect(mockFetchUnreadCount.mock.calls.length).toBe(2);
    });
    await waitFor(() => {
      expect(result.current.unreadCount).toBe(9);
    });
  });

  it("prevents concurrent fetches", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve(5), 100)),
    );

    const { result } = renderHook(() => useSuggestionsUnread());

    // Trigger multiple refreshes
    void result.current.refresh();
    void result.current.refresh();
    void result.current.refresh();

    vi.advanceTimersByTime(200);

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // Concurrency guard may prevent additional calls - the key behavior is
    // that multiple rapid refresh() calls don't cause N+1 fetches
    expect(mockFetchUnreadCount.mock.calls.length).toBeLessThanOrEqual(2);
  });

  it("listens for refresh events", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockResolvedValueOnce(5).mockResolvedValueOnce(12);

    renderHook(() => useSuggestionsUnread());

    await waitFor(() => {
      expect(mockFetchUnreadCount).toHaveBeenCalledTimes(1);
    });

    // Trigger refresh event
    window.dispatchEvent(new Event("suggestions-unread-refresh"));

    await waitFor(() => {
      expect(mockFetchUnreadCount).toHaveBeenCalledTimes(2);
    });
  });

  it("clears cache on refresh event", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockResolvedValueOnce(5);

    // Set cached data
    localStorage.setItem(
      CACHE_KEY,
      JSON.stringify({ count: 3, timestamp: Date.now() }),
    );

    renderHook(() => useSuggestionsUnread());

    // Trigger refresh event
    window.dispatchEvent(new Event("suggestions-unread-refresh"));

    await waitFor(() => {
      expect(localStorage.getItem(CACHE_KEY)).toBeNull();
    });
  });

  it("handles invalid cached JSON", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockResolvedValueOnce(5);

    // Set invalid JSON in cache
    localStorage.setItem(CACHE_KEY, "invalid-json");

    const { result } = renderHook(() => useSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.unreadCount).toBe(5);
  });

  it("stores fetched count in localStorage", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockResolvedValueOnce(15);

    renderHook(() => useSuggestionsUnread());

    await waitFor(() => {
      const cached = localStorage.getItem(CACHE_KEY);
      expect(cached).not.toBeNull();
      if (cached) {
        const data = JSON.parse(cached) as CachedData;
        expect(data.count).toBe(15);
        expect(data.timestamp).toBeGreaterThan(0);
      }
    });
  });
});
