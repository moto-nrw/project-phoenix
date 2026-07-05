import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseSWRAuth = vi.fn();
const mockMutate = vi.fn().mockResolvedValue(undefined);
vi.mock("~/lib/swr/hooks", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

import { useReminders } from "./use-reminders";

function swrReturn(
  data: unknown,
  extra: { error?: unknown; isLoading?: boolean } = {},
) {
  return {
    data,
    error: extra.error,
    isLoading: extra.isLoading ?? false,
    mutate: mockMutate,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("useReminders", () => {
  it("maps the SWR payload through", () => {
    const data = {
      reminders: [
        {
          type: "pickup_overdue",
          title: "Hannah",
          due_time: "10:00",
          minutes_away: -5,
        },
      ],
      count: 1,
      enabled: true,
    };
    mockUseSWRAuth.mockReturnValue(swrReturn(data));

    const { result } = renderHook(() => useReminders());

    expect(result.current.reminders).toEqual(data.reminders);
    expect(result.current.count).toBe(1);
    expect(result.current.enabled).toBe(true);
    expect(result.current.isLoading).toBe(false);
  });

  it("falls back to safe defaults while data is undefined", () => {
    mockUseSWRAuth.mockReturnValue(swrReturn(undefined, { isLoading: true }));

    const { result } = renderHook(() => useReminders());

    expect(result.current.reminders).toEqual([]);
    expect(result.current.count).toBe(0);
    expect(result.current.enabled).toBe(false);
    expect(result.current.isLoading).toBe(true);
  });
});

describe("useReminders — phoenix:reminders-stale revalidation", () => {
  afterEach(() => {
    // The hook removes its own listener on unmount, but guard against leaks.
    vi.clearAllMocks();
  });

  it("revalidates the tenant-scoped key when enabled and the event fires", () => {
    mockUseSWRAuth.mockReturnValue(
      swrReturn({ reminders: [], count: 0, enabled: true }),
    );

    renderHook(() => useReminders());
    expect(mockMutate).not.toHaveBeenCalled();

    act(() => {
      window.dispatchEvent(new CustomEvent("phoenix:reminders-stale"));
    });

    // The bound mutate from useSWRAuth targets the exact "{slug}:reminders" key.
    expect(mockMutate).toHaveBeenCalledTimes(1);
  });

  it("does NOT revalidate when the feature is disabled", () => {
    // Default: all reminder types off. A disabled tenant keeps its cheap idle
    // poll instead of reacting to every attendance burst.
    mockUseSWRAuth.mockReturnValue(
      swrReturn({ reminders: [], count: 0, enabled: false }),
    );

    renderHook(() => useReminders());

    act(() => {
      window.dispatchEvent(new CustomEvent("phoenix:reminders-stale"));
    });

    expect(mockMutate).not.toHaveBeenCalled();
  });

  it("stops revalidating after unmount", () => {
    mockUseSWRAuth.mockReturnValue(
      swrReturn({ reminders: [], count: 0, enabled: true }),
    );

    const { unmount } = renderHook(() => useReminders());
    unmount();

    act(() => {
      window.dispatchEvent(new CustomEvent("phoenix:reminders-stale"));
    });

    expect(mockMutate).not.toHaveBeenCalled();
  });
});

describe("useReminders — next_change_at precise timer", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // 10:00:00 local — reminders reason in wall-clock time.
    vi.setSystemTime(new Date(2026, 6, 4, 10, 0, 0));
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("refetches exactly at the next-change wall-clock minute (plus buffer)", () => {
    mockUseSWRAuth.mockReturnValue(
      swrReturn({
        reminders: [],
        count: 0,
        enabled: true,
        next_change_at: "10:05",
      }),
    );

    renderHook(() => useReminders());
    expect(mockMutate).not.toHaveBeenCalled();

    // Just shy of the target (5 min) + most of the 2s buffer: still nothing.
    act(() => {
      vi.advanceTimersByTime(5 * 60 * 1000 + 1000);
    });
    expect(mockMutate).not.toHaveBeenCalled();

    // Crossing the buffered boundary fires exactly once.
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(mockMutate).toHaveBeenCalledTimes(1);
  });

  it("does not schedule a timer when the feature is disabled", () => {
    mockUseSWRAuth.mockReturnValue(
      swrReturn({
        reminders: [],
        count: 0,
        enabled: false,
        next_change_at: "10:05",
      }),
    );

    renderHook(() => useReminders());
    act(() => {
      vi.advanceTimersByTime(60 * 60 * 1000);
    });
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it("does not schedule a timer when next_change_at is absent", () => {
    mockUseSWRAuth.mockReturnValue(
      swrReturn({ reminders: [], count: 0, enabled: true }),
    );

    renderHook(() => useReminders());
    act(() => {
      vi.advanceTimersByTime(60 * 60 * 1000);
    });
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it("refetches promptly when the next change is already in the past", () => {
    // Clock skew or a stale cache can hand us a boundary in the past; refetch
    // soon rather than a whole day later.
    mockUseSWRAuth.mockReturnValue(
      swrReturn({
        reminders: [],
        count: 0,
        enabled: true,
        next_change_at: "09:55",
      }),
    );

    renderHook(() => useReminders());
    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(mockMutate).toHaveBeenCalledTimes(1);
  });

  it("schedules an end-of-day '24:00' boundary at the next midnight", () => {
    // The backend emits "24:00" from formatMinutes(1440) for a boundary at the
    // top of the day (e.g. a 23:59 pickup flipping overdue). It must not be
    // dropped: the timer fires at the next midnight, 14h after the 10:00 clock.
    mockUseSWRAuth.mockReturnValue(
      swrReturn({
        reminders: [],
        count: 0,
        enabled: true,
        next_change_at: "24:00",
      }),
    );

    renderHook(() => useReminders());

    // Just before next midnight (14h from 10:00) + most of the buffer: nothing.
    act(() => {
      vi.advanceTimersByTime(14 * 60 * 60 * 1000 + 1000);
    });
    expect(mockMutate).not.toHaveBeenCalled();

    // Crossing the buffered midnight boundary fires exactly once.
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(mockMutate).toHaveBeenCalledTimes(1);
  });

  it("clears the scheduled timer on unmount", () => {
    mockUseSWRAuth.mockReturnValue(
      swrReturn({
        reminders: [],
        count: 0,
        enabled: true,
        next_change_at: "10:05",
      }),
    );

    const { unmount } = renderHook(() => useReminders());
    unmount();
    act(() => {
      vi.advanceTimersByTime(10 * 60 * 1000);
    });
    expect(mockMutate).not.toHaveBeenCalled();
  });
});
