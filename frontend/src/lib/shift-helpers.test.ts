import { describe, expect, it } from "vitest";

import {
  formatShiftLabel,
  groupShiftsByStaffAndDate,
  mapStaffShift,
  type BackendStaffShift,
} from "./shift-helpers";

describe("mapStaffShift", () => {
  it("maps snake_case backend shape to camelCase with string ids", () => {
    const backend: BackendStaffShift = {
      id: 42,
      staff_id: 7,
      date: "2026-07-06",
      start_time: "08:00",
      end_time: "16:00",
      break_minutes: 30,
      notes: "Frühdienst",
    };

    expect(mapStaffShift(backend)).toEqual({
      id: "42",
      staffId: "7",
      date: "2026-07-06",
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 30,
      shiftTypeId: null,
      notes: "Frühdienst",
    });
  });

  it("maps shift_type_id to a string id and defaults it to null", () => {
    const typed = mapStaffShift({
      id: 42,
      staff_id: 7,
      date: "2026-07-06",
      start_time: "08:00",
      end_time: "16:00",
      break_minutes: 0,
      shift_type_id: 5,
    });
    expect(typed.shiftTypeId).toBe("5");

    const untyped = mapStaffShift({
      id: 43,
      staff_id: 7,
      date: "2026-07-06",
      start_time: "08:00",
      end_time: "16:00",
      break_minutes: 0,
    });
    expect(untyped.shiftTypeId).toBeNull();
  });

  it("truncates HH:MM:SS times and defaults missing notes", () => {
    const backend: BackendStaffShift = {
      id: 1,
      staff_id: 2,
      date: "2026-07-06",
      start_time: "08:00:00",
      end_time: "16:30:00",
      break_minutes: 0,
    };

    const mapped = mapStaffShift(backend);
    expect(mapped.startTime).toBe("08:00");
    expect(mapped.endTime).toBe("16:30");
    expect(mapped.notes).toBe("");
  });
});

describe("formatShiftLabel", () => {
  it("renders start–end", () => {
    const shift = mapStaffShift({
      id: 1,
      staff_id: 2,
      date: "2026-07-06",
      start_time: "08:00",
      end_time: "16:00",
      break_minutes: 30,
    });
    expect(formatShiftLabel(shift)).toBe("08:00–16:00");
  });
});

describe("groupShiftsByStaffAndDate", () => {
  it("groups by staff id then date, preserving order", () => {
    const shifts = [
      { id: 1, staff_id: 7, date: "2026-07-06" },
      { id: 2, staff_id: 7, date: "2026-07-06" },
      { id: 3, staff_id: 7, date: "2026-07-07" },
      { id: 4, staff_id: 8, date: "2026-07-06" },
    ].map((s) =>
      mapStaffShift({
        ...s,
        start_time: "08:00",
        end_time: "16:00",
        break_minutes: 0,
      }),
    );

    const grouped = groupShiftsByStaffAndDate(shifts);
    expect(grouped.get("7")?.get("2026-07-06")).toHaveLength(2);
    expect(grouped.get("7")?.get("2026-07-07")).toHaveLength(1);
    expect(grouped.get("8")?.get("2026-07-06")).toHaveLength(1);
    expect(grouped.get("9")).toBeUndefined();
  });
});
