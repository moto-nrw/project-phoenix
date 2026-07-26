import { describe, it, expect, vi, beforeEach } from "vitest";

const mockAuthFetch = vi.fn();
vi.mock("./api-helpers", () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

import { fetchReminders, REMINDERS_SWR_KEY } from "./reminders-api";

beforeEach(() => {
  vi.clearAllMocks();
});

describe("reminders-api", () => {
  it("unwraps the proxy envelope and returns the reminders payload", async () => {
    const payload = {
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
    mockAuthFetch.mockResolvedValue({ success: true, data: payload });

    const result = await fetchReminders();

    expect(mockAuthFetch).toHaveBeenCalledWith("/api/reminders");
    expect(result).toEqual(payload);
  });

  it("exposes a stable shared SWR key", () => {
    expect(REMINDERS_SWR_KEY).toBe("reminders");
  });
});
