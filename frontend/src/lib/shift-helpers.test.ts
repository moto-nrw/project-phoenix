import { describe, expect, it } from "vitest";

import {
  formatDeltaHours,
  formatPlannedHours,
  formatShiftLabel,
  groupShiftsByStaffAndDate,
  mapStaffScheduleOverview,
  mapStaffShift,
  type BackendStaffScheduleOverview,
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
      seriesId: null,
      detached: false,
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

describe("mapStaffScheduleOverview", () => {
  it("maps the complete weekly bridge and every int64 id to strings", () => {
    const backend: BackendStaffScheduleOverview = {
      from: "2026-07-06",
      to: "2026-07-10",
      dienstplan_in_use: true,
      weekly_summaries: [
        {
          staff_id: 7,
          week_start: "2026-07-06",
          planned_minutes: 1080,
          target_minutes: 1215,
          delta_minutes: -135,
        },
        {
          staff_id: 8,
          week_start: "2026-07-06",
          planned_minutes: 270,
          target_minutes: null,
          delta_minutes: null,
        },
      ],
      staff: [{ id: 7, first_name: "Ada", last_name: "Lovelace" }],
      shifts: [
        {
          id: 42,
          staff_id: 7,
          date: "2026-07-06",
          start_time: "08:00:00",
          end_time: "16:00:00",
          break_minutes: 30,
          shift_type_id: 5,
          notes: "Frühdienst",
        },
      ],
      assignments: [
        {
          instance_id: 81,
          staff_id: 7,
          date: "2026-07-06",
          start_time: "07:30:00",
          end_time: "09:15:00",
          activity_title: "Frühbetreuung",
          room_id: 12,
          room_name: "Sonnenraum",
          status: "planned",
          is_absent: false,
          is_substitute: true,
          absence_reason: null,
          coverage_status: "uncovered",
          coverage_reason: null,
          uncovered_intervals: [
            { start_time: "07:30:00", end_time: "08:00:00" },
          ],
        },
      ],
    };

    expect(mapStaffScheduleOverview(backend)).toEqual({
      from: "2026-07-06",
      to: "2026-07-10",
      dienstplanInUse: true,
      dienstplanUsedWeeks: [],
      weeklySummaries: [
        {
          staffId: "7",
          weekStart: "2026-07-06",
          plannedMinutes: 1080,
          targetMinutes: 1215,
          deltaMinutes: -135,
        },
        {
          staffId: "8",
          weekStart: "2026-07-06",
          plannedMinutes: 270,
          targetMinutes: null,
          deltaMinutes: null,
        },
      ],
      staff: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
      shifts: [
        {
          id: "42",
          staffId: "7",
          date: "2026-07-06",
          startTime: "08:00",
          endTime: "16:00",
          breakMinutes: 30,
          shiftTypeId: "5",
          notes: "Frühdienst",
          seriesId: null,
          detached: false,
        },
      ],
      assignments: [
        {
          instanceId: "81",
          staffId: "7",
          date: "2026-07-06",
          startTime: "07:30",
          endTime: "09:15",
          activityTitle: "Frühbetreuung",
          roomId: "12",
          roomName: "Sonnenraum",
          status: "planned",
          isAbsent: false,
          isSubstitute: true,
          absenceReason: null,
          coverageStatus: "uncovered",
          coverageReason: null,
          uncoveredIntervals: [{ startTime: "07:30", endTime: "08:00" }],
        },
      ],
    });
  });

  it("preserves absence and Dienstplan-not-used reasons without inventing warnings", () => {
    const backend: BackendStaffScheduleOverview = {
      from: "2026-07-06",
      to: "2026-07-10",
      dienstplan_in_use: false,
      staff: [],
      shifts: [],
      assignments: [
        {
          instance_id: 81,
          staff_id: 7,
          date: "2026-07-06",
          start_time: "08:00",
          end_time: "09:00",
          activity_title: "Frühbetreuung",
          room_id: 12,
          room_name: "Sonnenraum",
          status: "planned",
          is_absent: true,
          is_substitute: true,
          absence_reason: "Krank",
          coverage_status: "not_applicable",
          coverage_reason: "absent",
          uncovered_intervals: [],
        },
        {
          instance_id: 82,
          staff_id: 8,
          date: "2026-07-06",
          start_time: "09:00",
          end_time: "10:00",
          activity_title: "Lernzeit",
          room_id: 13,
          room_name: "Mondraum",
          status: "active",
          is_absent: false,
          is_substitute: false,
          absence_reason: null,
          coverage_status: "not_applicable",
          coverage_reason: "dienstplan_not_used",
          uncovered_intervals: [],
        },
      ],
    };

    const mapped = mapStaffScheduleOverview(backend);

    expect(mapped.dienstplanInUse).toBe(false);
    expect(mapped.assignments[0]).toMatchObject({
      isAbsent: true,
      isSubstitute: true,
      absenceReason: "Krank",
      coverageStatus: "not_applicable",
      coverageReason: "absent",
      uncoveredIntervals: [],
    });
    expect(mapped.assignments[1]).toMatchObject({
      coverageStatus: "not_applicable",
      coverageReason: "dienstplan_not_used",
      uncoveredIntervals: [],
    });
    // Older backends without the field still map to an empty list.
    expect(mapped.weeklySummaries).toEqual([]);
  });
});

describe("formatPlannedHours", () => {
  it("formats minutes as German decimal hours", () => {
    expect(formatPlannedHours(1215)).toBe("20,25 h");
    expect(formatPlannedHours(2400)).toBe("40 h");
    expect(formatPlannedHours(0)).toBe("0 h");
  });
});

describe("formatDeltaHours", () => {
  it("formats signed deltas with a real minus sign", () => {
    expect(formatDeltaHours(-300)).toBe("−5 h");
    expect(formatDeltaHours(-135)).toBe("−2,25 h");
    expect(formatDeltaHours(90)).toBe("+1,5 h");
    expect(formatDeltaHours(0)).toBe("±0 h");
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
