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

  it("measures the delay in real time across a Berlin DST spring-forward", () => {
    // 2026-03-29 01:00 CET (+01:00); at 02:00 the clock jumps to 03:00 CEST
    // (+02:00). A "04:00" boundary is 3h of wall-clock away but only 2h of REAL
    // elapsed time, because the 02:00→03:00 hour never occurs. The delay must be
    // the real 2h (a naive wall-clock subtraction would wait 3h and fire an hour
    // late). TZ is pinned to Europe/Berlin (vitest.config.ts).
    vi.setSystemTime(new Date(2026, 2, 29, 1, 0, 0));
    mockUseSWRAuth.mockReturnValue(
      swrReturn({
        reminders: [],
        count: 0,
        enabled: true,
        next_change_at: "04:00",
      }),
    );

    renderHook(() => useReminders());

    // Just before the real 2h boundary + most of the buffer: nothing yet.
    act(() => {
      vi.advanceTimersByTime(2 * 60 * 60 * 1000 + 1000);
    });
    expect(mockMutate).not.toHaveBeenCalled();

    // Crossing the buffered 2h boundary fires exactly once (not at 3h).
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(mockMutate).toHaveBeenCalledTimes(1);
  });

  it("re-arms across midnight when next_change_at repeats verbatim", () => {
    // A tenant whose only time-based boundary is an end-of-day flip gets the
    // same next_change_at = "24:00" every day. The one-shot timer must be
    // re-armed for the NEXT midnight after it fires, not left dangling because
    // the day-agnostic string is unchanged — otherwise the precise refresh
    // silently degrades to the 60s poll from the first midnight on.
    vi.setSystemTime(new Date(2026, 6, 4, 23, 0, 0)); // 23:00 Berlin (CEST)
    mockUseSWRAuth.mockReturnValue(
      swrReturn({
        reminders: [],
        count: 0,
        enabled: true,
        next_change_at: "24:00",
      }),
    );

    const { rerender } = renderHook(() => useReminders());

    // First boundary: 1h to midnight + the 2s buffer.
    act(() => {
      vi.advanceTimersByTime(60 * 60 * 1000 + 3000);
    });
    expect(mockMutate).toHaveBeenCalledTimes(1);

    // The refetch returns the identical "24:00"; a re-render now past midnight
    // (new Berlin day) must schedule the next midnight rather than nothing.
    rerender();
    act(() => {
      vi.advanceTimersByTime(24 * 60 * 60 * 1000);
    });
    expect(mockMutate).toHaveBeenCalledTimes(2);
  });

  it("re-arms across midnight WITHOUT any re-render (self-sustaining timer)", () => {
    // The production hazard the re-arm guards against: a persistent end-of-day
    // "24:00" refetches to a payload SWR compares deep-equal and does NOT
    // re-render on, so the effect never re-runs. The timer must therefore
    // reschedule itself from inside its own callback rather than depending on a
    // render. This test never calls rerender() — the data (and thus the hook's
    // dependencies) is frozen for the whole run, exactly as SWR would leave it.
    vi.setSystemTime(new Date(2026, 6, 4, 23, 0, 0)); // 23:00 Berlin (CEST)
    mockUseSWRAuth.mockReturnValue(
      swrReturn({
        reminders: [],
        count: 0,
        enabled: true,
        next_change_at: "24:00",
      }),
    );

    renderHook(() => useReminders());

    // First midnight: 1h to the boundary + the 2s buffer.
    act(() => {
      vi.advanceTimersByTime(60 * 60 * 1000 + 3000);
    });
    expect(mockMutate).toHaveBeenCalledTimes(1);

    // No rerender. A whole day later the self-armed timer must fire again.
    act(() => {
      vi.advanceTimersByTime(24 * 60 * 60 * 1000);
    });
    expect(mockMutate).toHaveBeenCalledTimes(2);

    // And again the day after — the timer keeps re-arming indefinitely.
    act(() => {
      vi.advanceTimersByTime(24 * 60 * 60 * 1000);
    });
    expect(mockMutate).toHaveBeenCalledTimes(3);
  });

  it("does not re-arm (nor busy-refetch) a within-day boundary that is now past", () => {
    // After a normal within-day boundary fires, re-computing it yields a PAST
    // instant. The timer must not reschedule that spent one-shot — otherwise a
    // frozen (non-re-rendering) payload would refetch every 500ms forever. The
    // next boundary arrives via the effect re-running on a fresh string, not via
    // self-re-arm. Here the data is frozen, so exactly one refetch must happen.
    mockUseSWRAuth.mockReturnValue(
      swrReturn({
        reminders: [],
        count: 0,
        enabled: true,
        next_change_at: "10:05",
      }),
    );

    renderHook(() => useReminders());

    act(() => {
      vi.advanceTimersByTime(5 * 60 * 1000 + 3000);
    });
    expect(mockMutate).toHaveBeenCalledTimes(1);

    // Ten more minutes with a frozen payload: no busy-refetch loop.
    act(() => {
      vi.advanceTimersByTime(10 * 60 * 1000);
    });
    expect(mockMutate).toHaveBeenCalledTimes(1);
  });
});
