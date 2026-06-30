import { describe, it, expect, vi, beforeEach } from "vitest";

const mockUseSWRAuth = vi.fn();
vi.mock("~/lib/swr/hooks", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

import { useReminders } from "./use-reminders";

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
    mockUseSWRAuth.mockReturnValue({
      data,
      error: undefined,
      isLoading: false,
    });

    const result = useReminders();

    expect(result.reminders).toEqual(data.reminders);
    expect(result.count).toBe(1);
    expect(result.enabled).toBe(true);
    expect(result.isLoading).toBe(false);
  });

  it("falls back to safe defaults while data is undefined", () => {
    mockUseSWRAuth.mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: true,
    });

    const result = useReminders();

    expect(result.reminders).toEqual([]);
    expect(result.count).toBe(0);
    expect(result.enabled).toBe(false);
    expect(result.isLoading).toBe(true);
  });
});
