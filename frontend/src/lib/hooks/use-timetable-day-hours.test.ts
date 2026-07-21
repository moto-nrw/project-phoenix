import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockUseSettingsSchema } = vi.hoisted(() => ({
  mockUseSettingsSchema: vi.fn(),
}));

vi.mock("~/lib/hooks/use-settings-schema", () => ({
  useSettingsSchema: mockUseSettingsSchema,
}));

import { useTimetableDayHours } from "./use-timetable-day-hours";

function schema(startValue: unknown, endValue: unknown) {
  return {
    tabs: [
      {
        categories: [
          {
            items: [
              { key: "timetable.day_start_time", value: startValue },
              { key: "timetable.day_end_time", value: endValue },
            ],
          },
        ],
      },
    ],
  };
}

describe("useTimetableDayHours", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("uses configured start and end hours from settings schema", () => {
    mockUseSettingsSchema.mockReturnValue({
      data: schema("08:30", "16:15"),
    });

    const { result } = renderHook(() => useTimetableDayHours());

    expect(result.current).toEqual({ dayStartHour: 8, dayEndHour: 16 });
  });

  it("falls back when settings are missing, malformed or inverted", () => {
    mockUseSettingsSchema.mockReturnValueOnce({ data: null });
    expect(renderHook(() => useTimetableDayHours()).result.current).toEqual({
      dayStartHour: 9,
      dayEndHour: 17,
    });

    mockUseSettingsSchema.mockReturnValueOnce({
      data: schema("xx", "99:00"),
    });
    expect(renderHook(() => useTimetableDayHours()).result.current).toEqual({
      dayStartHour: 9,
      dayEndHour: 17,
    });

    mockUseSettingsSchema.mockReturnValueOnce({
      data: schema("18:00", "12:00"),
    });
    expect(renderHook(() => useTimetableDayHours()).result.current).toEqual({
      dayStartHour: 18,
      dayEndHour: 17,
    });
  });
});
