import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useUnreadCount } from "./use-unread-count";

// ---------------------------------------------------------------------------
// Setup
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  localStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
  localStorage.clear();
});

const CACHE_KEY = "test_unread_count";

// ---------------------------------------------------------------------------
// enabled gate
// ---------------------------------------------------------------------------

describe("useUnreadCount — enabled gate", () => {
  it("returns 0 and isLoading=false when enabled is false", async () => {
    const fetcher = vi.fn();
    const { result } = renderHook(() =>
      useUnreadCount({ enabled: false, fetcher }),
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.unreadCount).toBe(0);
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("fetches when enabled is true", async () => {
    const fetcher = vi.fn().mockResolvedValue(3);
    renderHook(() => useUnreadCount({ enabled: true, fetcher }));

    await waitFor(() => expect(fetcher).toHaveBeenCalled());
  });
});

// ---------------------------------------------------------------------------
// Basic fetch
// ---------------------------------------------------------------------------

describe("useUnreadCount — basic fetch", () => {
  it("sets unreadCount after a successful fetch", async () => {
    const fetcher = vi.fn().mockResolvedValue(7);
    const { result } = renderHook(() =>
      useUnreadCount({ enabled: true, fetcher }),
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(7));
    expect(result.current.isLoading).toBe(false);
  });

  it("swallows fetch errors silently (badge must never break nav)", async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error("network fail"));
    const { result } = renderHook(() =>
      useUnreadCount({ enabled: true, fetcher }),
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.unreadCount).toBe(0);
  });

  it("calls onError with the error when fetch fails", async () => {
    const onError = vi.fn();
    const err = new Error("oops");
    const fetcher = vi.fn().mockRejectedValue(err);

    renderHook(() => useUnreadCount({ enabled: true, fetcher, onError }));

    await waitFor(() => expect(onError).toHaveBeenCalledWith(err));
  });
});

// ---------------------------------------------------------------------------
// LocalStorage cache
// ---------------------------------------------------------------------------

describe("useUnreadCount — cache", () => {
  it("shows cached count immediately before the fetch resolves", async () => {
    localStorage.setItem(
      CACHE_KEY,
      JSON.stringify({ count: 4, timestamp: Date.now() }),
    );

    let resolve: (n: number) => void = () => undefined;
    const fetcher = vi.fn(
      () =>
        new Promise<number>((r) => {
          resolve = r;
        }),
    );

    const { result } = renderHook(() =>
      useUnreadCount({ enabled: true, fetcher, cacheKey: CACHE_KEY }),
    );

    // Cached value appears synchronously / immediately
    expect(result.current.unreadCount).toBe(4);

    resolve(10);
    await waitFor(() => expect(result.current.unreadCount).toBe(10));
  });

  it("ignores a cache entry older than 60 seconds", async () => {
    localStorage.setItem(
      CACHE_KEY,
      JSON.stringify({ count: 4, timestamp: Date.now() - 61_000 }),
    );

    const fetcher = vi.fn().mockResolvedValue(9);
    const { result } = renderHook(() =>
      useUnreadCount({ enabled: true, fetcher, cacheKey: CACHE_KEY }),
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(9));
  });

  it("handles malformed cache JSON without throwing", async () => {
    localStorage.setItem(CACHE_KEY, "not-json");

    const fetcher = vi.fn().mockResolvedValue(5);
    const { result } = renderHook(() =>
      useUnreadCount({ enabled: true, fetcher, cacheKey: CACHE_KEY }),
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(5));
  });

  it("writes the fetched count back to localStorage", async () => {
    const fetcher = vi.fn().mockResolvedValue(15);
    renderHook(() =>
      useUnreadCount({ enabled: true, fetcher, cacheKey: CACHE_KEY }),
    );

    await waitFor(() => {
      const raw = localStorage.getItem(CACHE_KEY);
      expect(raw).not.toBeNull();
      const data = JSON.parse(raw!) as { count: number; timestamp: number };
      expect(data.count).toBe(15);
    });
  });

  it("does not write to localStorage when no cacheKey is given", async () => {
    const fetcher = vi.fn().mockResolvedValue(3);
    renderHook(() => useUnreadCount({ enabled: true, fetcher }));

    await waitFor(() => {
      // localStorage should be empty — no keys written
      expect(localStorage.length).toBe(0);
    });
  });
});

// ---------------------------------------------------------------------------
// refresh()
// ---------------------------------------------------------------------------

describe("useUnreadCount — refresh", () => {
  it("refresh() fetches a fresh count and updates state", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce(2).mockResolvedValueOnce(8);
    const { result } = renderHook(() =>
      useUnreadCount({ enabled: true, fetcher }),
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(2));

    await act(async () => {
      await result.current.refresh();
    });

    await waitFor(() => expect(result.current.unreadCount).toBe(8));
  });

  it("refresh(true) busts the cache", async () => {
    localStorage.setItem(
      CACHE_KEY,
      JSON.stringify({ count: 3, timestamp: Date.now() }),
    );

    const fetcher = vi.fn().mockResolvedValue(10);
    const { result } = renderHook(() =>
      useUnreadCount({ enabled: true, fetcher, cacheKey: CACHE_KEY }),
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(10));

    // Write stale value to cache
    localStorage.setItem(
      CACHE_KEY,
      JSON.stringify({ count: 99, timestamp: Date.now() }),
    );

    fetcher.mockResolvedValue(20);
    await act(async () => {
      await result.current.refresh(true);
    });

    await waitFor(() => expect(result.current.unreadCount).toBe(20));
  });

  it("does not fetch when enabled=false and refresh() is called", async () => {
    const fetcher = vi.fn().mockResolvedValue(5);
    const { result } = renderHook(() =>
      useUnreadCount({ enabled: false, fetcher }),
    );

    await act(async () => {
      await result.current.refresh();
    });

    expect(fetcher).not.toHaveBeenCalled();
    expect(result.current.unreadCount).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// Event-driven refresh
// ---------------------------------------------------------------------------

describe("useUnreadCount — event listeners", () => {
  it("refetches when a registered event fires", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce(1).mockResolvedValueOnce(6);
    const { result } = renderHook(() =>
      useUnreadCount({
        enabled: true,
        fetcher,
        eventNames: ["test-refresh-event"],
      }),
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(1));

    act(() => {
      window.dispatchEvent(new Event("test-refresh-event"));
    });

    await waitFor(() => expect(result.current.unreadCount).toBe(6));
  });

  it("removes the cache entry before refetching on event", async () => {
    localStorage.setItem(
      CACHE_KEY,
      JSON.stringify({ count: 99, timestamp: Date.now() }),
    );

    const fetcher = vi.fn().mockResolvedValue(7);
    renderHook(() =>
      useUnreadCount({
        enabled: true,
        fetcher,
        cacheKey: CACHE_KEY,
        eventNames: ["test-refresh-event"],
      }),
    );

    await waitFor(() => expect(fetcher).toHaveBeenCalled());

    act(() => {
      window.dispatchEvent(new Event("test-refresh-event"));
    });

    await waitFor(() => {
      expect(localStorage.getItem(CACHE_KEY)).toBeNull();
    });
  });

  it("listens to multiple event names", async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce(1)
      .mockResolvedValueOnce(2)
      .mockResolvedValueOnce(3);

    renderHook(() =>
      useUnreadCount({
        enabled: true,
        fetcher,
        eventNames: ["event-a", "event-b"],
      }),
    );

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));

    act(() => window.dispatchEvent(new Event("event-a")));
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));

    act(() => window.dispatchEvent(new Event("event-b")));
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(3));
  });

  it("debounces rapid event bursts into a single refetch", async () => {
    const fetcher = vi.fn().mockResolvedValue(5);
    renderHook(() =>
      useUnreadCount({
        enabled: true,
        fetcher,
        eventNames: ["burst-event"],
        eventDebounceMs: 300,
      }),
    );

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));

    act(() => {
      window.dispatchEvent(new Event("burst-event"));
      window.dispatchEvent(new Event("burst-event"));
      window.dispatchEvent(new Event("burst-event"));
    });

    // Before the debounce window closes, only the mount fetch ran
    expect(fetcher).toHaveBeenCalledTimes(1);

    act(() => vi.advanceTimersByTime(300));

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
  });
});

// ---------------------------------------------------------------------------
// refetchOnFocus
// ---------------------------------------------------------------------------

describe("useUnreadCount — refetchOnFocus", () => {
  it("refetches when window gains focus and refetchOnFocus is true", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce(1).mockResolvedValueOnce(4);
    const { result } = renderHook(() =>
      useUnreadCount({ enabled: true, fetcher, refetchOnFocus: true }),
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(1));

    act(() => window.dispatchEvent(new Event("focus")));

    await waitFor(() => expect(result.current.unreadCount).toBe(4));
  });

  it("does NOT refetch on focus when refetchOnFocus is false (default)", async () => {
    const fetcher = vi.fn().mockResolvedValue(2);
    renderHook(() => useUnreadCount({ enabled: true, fetcher }));

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));

    act(() => window.dispatchEvent(new Event("focus")));

    // No additional fetch triggered
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
});

// ---------------------------------------------------------------------------
// Concurrency guard
// ---------------------------------------------------------------------------

describe("useUnreadCount — concurrency guard", () => {
  it("does not start a second fetch while one is already in flight", async () => {
    let resolveFirst: (n: number) => void = () => undefined;
    const firstFetch = new Promise<number>((r) => {
      resolveFirst = r;
    });
    const fetcher = vi
      .fn()
      .mockReturnValueOnce(firstFetch)
      .mockResolvedValue(9);

    const { result } = renderHook(() =>
      useUnreadCount({ enabled: true, fetcher }),
    );

    // While the first fetch is still pending, call refresh() — should be a no-op
    void result.current.refresh();
    void result.current.refresh();

    expect(fetcher).toHaveBeenCalledTimes(1);

    resolveFirst(3);
    await waitFor(() => expect(result.current.unreadCount).toBe(3));
  });
});

// ---------------------------------------------------------------------------
// Tenant/session isolation
// ---------------------------------------------------------------------------

describe("useUnreadCount — tenant/session isolation", () => {
  it("discards an old tenant result and retries with the new tenant fetcher", async () => {
    const oldKey = "unread:school-a";
    const newKey = "unread:school-b";
    localStorage.setItem(
      oldKey,
      JSON.stringify({ count: 4, timestamp: Date.now() }),
    );

    let resolveOld: (count: number) => void = () => undefined;
    const oldFetcher = vi.fn(
      () =>
        new Promise<number>((resolve) => {
          resolveOld = resolve;
        }),
    );
    const newFetcher = vi.fn().mockResolvedValue(9);

    const { result, rerender } = renderHook(
      ({ cacheKey, fetcher }) =>
        useUnreadCount({ enabled: true, cacheKey, fetcher }),
      { initialProps: { cacheKey: oldKey, fetcher: oldFetcher } },
    );

    expect(result.current.unreadCount).toBe(4);
    await waitFor(() => expect(oldFetcher).toHaveBeenCalledTimes(1));

    rerender({ cacheKey: newKey, fetcher: newFetcher });

    // The old tenant's cached count is cleared during the switch commit, before
    // either request resolves.
    expect(result.current.unreadCount).toBe(0);

    resolveOld(7);

    await waitFor(() => expect(result.current.unreadCount).toBe(9));
    expect(oldFetcher).toHaveBeenCalledTimes(1);
    expect(newFetcher).toHaveBeenCalledTimes(1);
    expect(JSON.parse(localStorage.getItem(oldKey)!).count).toBe(4);
    expect(JSON.parse(localStorage.getItem(newKey)!).count).toBe(9);
  });

  it("retries the new tenant when the old tenant request rejects", async () => {
    let rejectOld: (error: Error) => void = () => undefined;
    const oldFetcher = vi.fn(
      () =>
        new Promise<number>((_resolve, reject) => {
          rejectOld = reject;
        }),
    );
    const newFetcher = vi.fn().mockResolvedValue(6);
    const onError = vi.fn();

    const { result, rerender } = renderHook(
      ({ cacheKey, fetcher }) =>
        useUnreadCount({ enabled: true, cacheKey, fetcher, onError }),
      {
        initialProps: {
          cacheKey: "unread:school-a",
          fetcher: oldFetcher,
        },
      },
    );

    await waitFor(() => expect(oldFetcher).toHaveBeenCalledTimes(1));
    rerender({ cacheKey: "unread:school-b", fetcher: newFetcher });

    rejectOld(new Error("old tenant request rejected"));

    await waitFor(() => expect(result.current.unreadCount).toBe(6));
    expect(newFetcher).toHaveBeenCalledTimes(1);
    expect(onError).not.toHaveBeenCalled();
  });

  it("does not publish or cache an in-flight result after logout", async () => {
    const cacheKey = "unread:logout";
    let resolveAfterLogout: (count: number) => void = () => undefined;
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce(5)
      .mockImplementationOnce(
        () =>
          new Promise<number>((resolve) => {
            resolveAfterLogout = resolve;
          }),
      );

    const { result, rerender } = renderHook(
      ({ enabled }) => useUnreadCount({ enabled, cacheKey, fetcher }),
      { initialProps: { enabled: true } },
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(5));
    act(() => {
      void result.current.refresh(true);
    });
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));

    rerender({ enabled: false });
    expect(result.current.unreadCount).toBe(0);
    expect(result.current.isLoading).toBe(false);

    resolveAfterLogout(12);

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.unreadCount).toBe(0);
    expect(JSON.parse(localStorage.getItem(cacheKey)!).count).toBe(5);
  });
});

// ---------------------------------------------------------------------------
// initialCount (server-preloaded seed, #2973)
// ---------------------------------------------------------------------------

describe("useUnreadCount — initialCount", () => {
  it("shows the seed, caches it, and skips the mount fetch", async () => {
    const fetcher = vi.fn().mockResolvedValue(9);
    const { result } = renderHook(() =>
      useUnreadCount({
        enabled: true,
        fetcher,
        cacheKey: CACHE_KEY,
        initialCount: 4,
      }),
    );

    expect(result.current.unreadCount).toBe(4);
    expect(result.current.isLoading).toBe(false);
    await act(async () => {
      await Promise.resolve();
    });
    expect(fetcher).not.toHaveBeenCalled();
    expect(JSON.parse(localStorage.getItem(CACHE_KEY) ?? "{}")).toMatchObject({
      count: 4,
    });
  });

  it("still refetches on a refresh event after being seeded", async () => {
    const fetcher = vi.fn().mockResolvedValue(9);
    const { result } = renderHook(() =>
      useUnreadCount({
        enabled: true,
        fetcher,
        cacheKey: CACHE_KEY,
        eventNames: ["seed-refresh"],
        initialCount: 4,
      }),
    );

    act(() => {
      window.dispatchEvent(new Event("seed-refresh"));
    });

    await waitFor(() => expect(result.current.unreadCount).toBe(9));
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("ignores the seed while disabled and fetches once enabled", async () => {
    const fetcher = vi.fn().mockResolvedValue(9);
    const { result, rerender } = renderHook(
      ({ enabled }: { enabled: boolean }) =>
        useUnreadCount({ enabled, fetcher, initialCount: 4 }),
      { initialProps: { enabled: false } },
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(0));
    expect(fetcher).not.toHaveBeenCalled();

    rerender({ enabled: true });

    await waitFor(() => expect(result.current.unreadCount).toBe(9));
  });
});
