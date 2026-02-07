import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

// Use vi.hoisted for mock values referenced in vi.mock
const { mockUseShellAuth, mockFetchUnreadCount, mockFetchUnviewedCount } =
  vi.hoisted(() => ({
    mockUseShellAuth: vi.fn(),
    mockFetchUnreadCount: vi.fn(),
    mockFetchUnviewedCount: vi.fn(),
  }));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: (): ReturnType<typeof mockUseShellAuth> => mockUseShellAuth(),
}));

vi.mock("~/lib/operator/suggestions-api", () => ({
  operatorSuggestionsService: {
    fetchUnreadCount: mockFetchUnreadCount,
    fetchUnviewedCount: mockFetchUnviewedCount,
  },
}));

import { useOperatorSuggestionsUnread } from "./use-operator-suggestions-unread";

interface CachedData {
  unreadComments: number;
  unviewedPosts: number;
  timestamp: number;
}

describe("useOperatorSuggestionsUnread", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns 0 count when not in operator mode", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "teacher" });

    const { result } = renderHook(() => useOperatorSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.unreadCount).toBe(0);
    expect(mockFetchUnreadCount).not.toHaveBeenCalled();
    expect(mockFetchUnviewedCount).not.toHaveBeenCalled();
  });

  it("fetches both unread and unviewed counts in operator mode", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockResolvedValueOnce(3);
    mockFetchUnviewedCount.mockResolvedValueOnce(5);

    const { result } = renderHook(() => useOperatorSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.unreadCount).toBe(8); // 3 + 5
    expect(mockFetchUnreadCount).toHaveBeenCalled();
    expect(mockFetchUnviewedCount).toHaveBeenCalled();
  });

  it("uses cached counts if available", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockResolvedValueOnce(10);
    mockFetchUnviewedCount.mockResolvedValueOnce(20);

    // Set cached data
    const cachedData = {
      unreadComments: 2,
      unviewedPosts: 3,
      timestamp: Date.now(),
    };
    localStorage.setItem(
      "operator_suggestions_unread_count",
      JSON.stringify(cachedData),
    );

    const { result } = renderHook(() => useOperatorSuggestionsUnread());

    // Should immediately show cached count
    expect(result.current.unreadCount).toBe(5); // 2 + 3

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    // Should eventually update with fresh data
    await waitFor(() => {
      expect(result.current.unreadCount).toBe(30); // 10 + 20
    });
  });

  it("ignores expired cache", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockResolvedValueOnce(7);
    mockFetchUnviewedCount.mockResolvedValueOnce(3);

    // Set expired cached data (older than 1 minute)
    const expiredData = {
      unreadComments: 2,
      unviewedPosts: 2,
      timestamp: Date.now() - 61 * 1000,
    };
    localStorage.setItem(
      "operator_suggestions_unread_count",
      JSON.stringify(expiredData),
    );

    const { result } = renderHook(() => useOperatorSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.unreadCount).toBe(10); // 7 + 3
  });

  it("handles fetch errors gracefully", async () => {
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockRejectedValueOnce(new Error("Network error"));
    mockFetchUnviewedCount.mockRejectedValueOnce(new Error("Network error"));

    const { result } = renderHook(() => useOperatorSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.unreadCount).toBe(0);
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "Failed to fetch operator unread counts:",
      expect.any(Error),
    );

    consoleErrorSpy.mockRestore();
  });

  it("refresh function updates counts", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockResolvedValueOnce(5).mockResolvedValueOnce(8);
    mockFetchUnviewedCount.mockResolvedValueOnce(2).mockResolvedValueOnce(4);

    const { result } = renderHook(() => useOperatorSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.unreadCount).toBe(7); // 5 + 2
    });

    await result.current.refresh();

    await waitFor(() => {
      expect(result.current.unreadCount).toBe(12); // 8 + 4
    });
  });

  it("skipCache parameter forces fresh fetch", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockResolvedValueOnce(10);
    mockFetchUnviewedCount.mockResolvedValueOnce(5);

    // Set cached data
    localStorage.setItem(
      "operator_suggestions_unread_count",
      JSON.stringify({
        unreadComments: 1,
        unviewedPosts: 1,
        timestamp: Date.now(),
      }),
    );

    const { result } = renderHook(() => useOperatorSuggestionsUnread());

    await result.current.refresh(true);

    await waitFor(() => {
      expect(result.current.unreadCount).toBe(15); // 10 + 5
    });
  });

  it("prevents concurrent fetches", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve(5), 100)),
    );
    mockFetchUnviewedCount.mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve(3), 100)),
    );

    const { result } = renderHook(() => useOperatorSuggestionsUnread());

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

  it("listens for unread comments refresh event", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockResolvedValueOnce(5).mockResolvedValueOnce(12);
    mockFetchUnviewedCount.mockResolvedValueOnce(2).mockResolvedValueOnce(3);

    renderHook(() => useOperatorSuggestionsUnread());

    await waitFor(() => {
      expect(mockFetchUnreadCount).toHaveBeenCalledTimes(1);
    });

    // Trigger unread refresh event
    window.dispatchEvent(new Event("operator-suggestions-unread-refresh"));

    await waitFor(() => {
      expect(mockFetchUnreadCount).toHaveBeenCalledTimes(2);
    });
  });

  it("listens for unviewed posts refresh event", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockResolvedValueOnce(5).mockResolvedValueOnce(6);
    mockFetchUnviewedCount.mockResolvedValueOnce(2).mockResolvedValueOnce(10);

    renderHook(() => useOperatorSuggestionsUnread());

    await waitFor(() => {
      expect(mockFetchUnviewedCount).toHaveBeenCalledTimes(1);
    });

    // Trigger unviewed refresh event
    window.dispatchEvent(new Event("operator-suggestions-unviewed-refresh"));

    await waitFor(() => {
      expect(mockFetchUnviewedCount).toHaveBeenCalledTimes(2);
    });
  });

  it("clears cache on refresh event", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockResolvedValueOnce(5);
    mockFetchUnviewedCount.mockResolvedValueOnce(3);

    // Set cached data
    localStorage.setItem(
      "operator_suggestions_unread_count",
      JSON.stringify({
        unreadComments: 2,
        unviewedPosts: 2,
        timestamp: Date.now(),
      }),
    );

    renderHook(() => useOperatorSuggestionsUnread());

    // Trigger refresh event
    window.dispatchEvent(new Event("operator-suggestions-unread-refresh"));

    await waitFor(() => {
      expect(
        localStorage.getItem("operator_suggestions_unread_count"),
      ).toBeNull();
    });
  });

  it("does not add event listeners when not in operator mode", () => {
    const addEventListenerSpy = vi.spyOn(window, "addEventListener");
    mockUseShellAuth.mockReturnValue({ mode: "teacher" });

    renderHook(() => useOperatorSuggestionsUnread());

    expect(addEventListenerSpy).not.toHaveBeenCalled();
  });

  it("stores fetched counts in localStorage", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockResolvedValueOnce(15);
    mockFetchUnviewedCount.mockResolvedValueOnce(8);

    renderHook(() => useOperatorSuggestionsUnread());

    await waitFor(() => {
      const cached = localStorage.getItem("operator_suggestions_unread_count");
      expect(cached).not.toBeNull();
      if (cached) {
        const data = JSON.parse(cached) as CachedData;
        expect(data.unreadComments).toBe(15);
        expect(data.unviewedPosts).toBe(8);
        expect(data.timestamp).toBeGreaterThan(0);
      }
    });
  });

  it("handles invalid cached JSON", async () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    mockFetchUnreadCount.mockResolvedValueOnce(5);
    mockFetchUnviewedCount.mockResolvedValueOnce(3);

    // Set invalid JSON in cache
    localStorage.setItem("operator_suggestions_unread_count", "invalid-json");

    const { result } = renderHook(() => useOperatorSuggestionsUnread());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.unreadCount).toBe(8); // 5 + 3
  });
});
