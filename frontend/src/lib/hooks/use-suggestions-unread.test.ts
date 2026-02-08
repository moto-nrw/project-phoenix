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

import { useSuggestionsUnread } from "./use-suggestions-unread";

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
    localStorage.setItem(
      "suggestions_unread_count",
      JSON.stringify(cachedData),
    );

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
    localStorage.setItem(
      "suggestions_unread_count",
      JSON.stringify(expiredData),
    );

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
    mockFetchUnreadCount.mockResolvedValueOnce(10);

    // Set cached data
    localStorage.setItem(
      "suggestions_unread_count",
      JSON.stringify({ count: 3, timestamp: Date.now() }),
    );

    const { result } = renderHook(() => useSuggestionsUnread());

    await result.current.refresh(true);

    await waitFor(() => {
      expect(result.current.unreadCount).toBe(10);
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
      "suggestions_unread_count",
      JSON.stringify({ count: 3, timestamp: Date.now() }),
    );

    renderHook(() => useSuggestionsUnread());

    // Trigger refresh event
    window.dispatchEvent(new Event("suggestions-unread-refresh"));

    await waitFor(() => {
      expect(localStorage.getItem("suggestions_unread_count")).toBeNull();
    });
  });

  it("handles invalid cached JSON", async () => {
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockFetchUnreadCount.mockResolvedValueOnce(5);

    // Set invalid JSON in cache
    localStorage.setItem("suggestions_unread_count", "invalid-json");

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
      const cached = localStorage.getItem("suggestions_unread_count");
      expect(cached).not.toBeNull();
      if (cached) {
        const data = JSON.parse(cached) as CachedData;
        expect(data.count).toBe(15);
        expect(data.timestamp).toBeGreaterThan(0);
      }
    });
  });
});
