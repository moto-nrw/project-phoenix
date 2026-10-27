import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockUseTimetableDayTimes } = vi.hoisted(() => ({
  mockUseTimetableDayTimes: vi.fn(),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTimetableDayTimes: mockUseTimetableDayTimes,
}));

import { useTimetableDayHours } from "./use-timetable-day-hours";

// The day comes from the tenant resolve since #4996.
function dayTimes(start: string | undefined, end: string | undefined) {
  return { start, end };
}

describe("useTimetableDayHours", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("uses the configured start and end times of the school", () => {
    mockUseTimetableDayTimes.mockReturnValue(dayTimes("08:30", "16:15"));

    const { result } = renderHook(() => useTimetableDayHours());

    expect(result.current).toEqual({
      dayStartHour: 8,
      dayEndHour: 17,
      dayStartTime: "08:30",
      dayEndTime: "16:15",
    });
  });

  it("keeps valid same-hour ranges and rounds the grid end up", () => {
    mockUseTimetableDayTimes.mockReturnValue(dayTimes("08:30", "08:45"));

    expect(renderHook(() => useTimetableDayHours()).result.current).toEqual({
      dayStartHour: 8,
      dayEndHour: 9,
      dayStartTime: "08:30",
      dayEndTime: "08:45",
    });
  });

  it("falls back when settings are missing, malformed or inverted", () => {
    mockUseTimetableDayTimes.mockReturnValueOnce(
      dayTimes(undefined, undefined),
    );
    expect(renderHook(() => useTimetableDayHours()).result.current).toEqual({
      dayStartHour: 9,
      dayEndHour: 17,
      dayStartTime: "09:00",
      dayEndTime: "17:00",
    });

    mockUseTimetableDayTimes.mockReturnValueOnce(dayTimes("xx", "99:00"));
    expect(renderHook(() => useTimetableDayHours()).result.current).toEqual({
      dayStartHour: 9,
      dayEndHour: 17,
      dayStartTime: "09:00",
      dayEndTime: "17:00",
    });

    mockUseTimetableDayTimes.mockReturnValueOnce(dayTimes("18:00", "12:00"));
    expect(renderHook(() => useTimetableDayHours()).result.current).toEqual({
      dayStartHour: 9,
      dayEndHour: 17,
      dayStartTime: "09:00",
      dayEndTime: "17:00",
    });
  });
});
