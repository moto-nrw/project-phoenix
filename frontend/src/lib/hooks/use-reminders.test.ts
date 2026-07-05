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
