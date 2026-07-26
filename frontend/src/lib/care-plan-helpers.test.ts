import { describe, expect, it } from "vitest";

import { computeFreeCareBands, resolveDayDeviation } from "./care-plan-helpers";
import type {
  CarePlanDay,
  CarePlanInstance,
  CarePlanSlot,
} from "./student-care-plan-api";
import type { StudentStatusDay } from "./student-status-days-api";

function slot(
  time: string | null,
  source: CarePlanSlot["source"],
): CarePlanSlot {
  return { expectedTime: time, source };
}

function instance(
  id: string,
  startTime: string,
  endTime: string,
  status: CarePlanInstance["status"] = "planned",
): CarePlanInstance {
  return {
    id,
    title: `Block ${id}`,
    startTime,
    endTime,
    roomId: "1",
    status,
    activeGroupId: null,
    attendance: {
      status: "expected",
      substatus: null,
      note: null,
      checkedInAt: null,
      checkedOutAt: null,
      isUnplanned: false,
    },
  };
}

function day(overrides: Partial<CarePlanDay>): CarePlanDay {
  return {
    studentId: "1",
    date: "2026-07-20",
    weekday: 1,
    arrival: slot(null, "none"),
    instances: [],
    pickup: slot(null, "none"),
    ...overrides,
  };
}

describe("computeFreeCareBands", () => {
  it("returns the whole window when there are no instances", () => {
    const bands = computeFreeCareBands(
      day({
        arrival: slot("08:00", "schedule"),
        pickup: slot("16:00", "schedule"),
      }),
    );
    expect(bands).toEqual([{ startTime: "08:00", endTime: "16:00" }]);
  });

  it("fills the gaps between arrival, blocks, and pickup", () => {
    const bands = computeFreeCareBands(
      day({
        arrival: slot("08:00", "schedule"),
        pickup: slot("16:00", "schedule"),
        instances: [
          instance("a", "09:00", "10:00"),
          instance("b", "12:00", "13:00"),
        ],
      }),
    );
    expect(bands).toEqual([
      { startTime: "08:00", endTime: "09:00" },
      { startTime: "10:00", endTime: "12:00" },
      { startTime: "13:00", endTime: "16:00" },
    ]);
  });

  it("emits no band when a block starts exactly at arrival and ends at pickup", () => {
    const bands = computeFreeCareBands(
      day({
        arrival: slot("09:00", "schedule"),
        pickup: slot("10:00", "schedule"),
        instances: [instance("a", "09:00", "10:00")],
      }),
    );
    expect(bands).toEqual([]);
  });

  it("drops sub-threshold (<10min) gaps as transitions", () => {
    const bands = computeFreeCareBands(
      day({
        arrival: slot("09:00", "schedule"),
        pickup: slot("12:00", "schedule"),
        instances: [
          instance("a", "09:00", "10:00"),
          instance("b", "10:05", "12:00"), // 5-min seam -> no band
        ],
      }),
    );
    expect(bands).toEqual([]);
  });

  it("ignores cancelled instances (their slot becomes free)", () => {
    const bands = computeFreeCareBands(
      day({
        arrival: slot("09:00", "schedule"),
        pickup: slot("11:00", "schedule"),
        instances: [instance("a", "09:00", "10:00", "cancelled")],
      }),
    );
    expect(bands).toEqual([{ startTime: "09:00", endTime: "11:00" }]);
  });

  it("falls back to instance bounds when arrival/pickup have no time", () => {
    const bands = computeFreeCareBands(
      day({
        arrival: slot(null, "none"),
        pickup: slot(null, "none"),
        instances: [
          instance("a", "09:00", "10:00"),
          instance("b", "11:00", "12:00"),
        ],
      }),
    );
    expect(bands).toEqual([{ startTime: "10:00", endTime: "11:00" }]);
  });

  it("returns nothing when neither a window bound nor instances exist", () => {
    expect(computeFreeCareBands(day({}))).toEqual([]);
  });

  it("merges overlapping instances into one occupied stretch", () => {
    const bands = computeFreeCareBands(
      day({
        arrival: slot("08:00", "schedule"),
        pickup: slot("12:00", "schedule"),
        instances: [
          instance("a", "09:00", "10:30"),
          instance("b", "10:00", "11:00"), // overlaps a -> merged 09:00–11:00
        ],
      }),
    );
    expect(bands).toEqual([
      { startTime: "08:00", endTime: "09:00" },
      { startTime: "11:00", endTime: "12:00" },
    ]);
  });

  it("ignores instances that fall entirely outside the arrival→pickup window", () => {
    const bands = computeFreeCareBands(
      day({
        arrival: slot("09:00", "schedule"),
        pickup: slot("11:00", "schedule"),
        instances: [
          instance("early", "07:00", "08:00"), // ends before the window
          instance("inside", "09:30", "10:00"),
          instance("late", "12:00", "13:00"), // starts after the window
        ],
      }),
    );
    expect(bands).toEqual([
      { startTime: "09:00", endTime: "09:30" },
      { startTime: "10:00", endTime: "11:00" },
    ]);
  });
});

describe("resolveDayDeviation", () => {
  const statusDay = (
    date: string,
    status: StudentStatusDay["status"],
    cleared = false,
  ): StudentStatusDay =>
    ({
      id: "1",
      student_id: "1",
      date,
      status,
      label: status === "sick" ? "Krank" : "Entschuldigt",
      reported_at: "",
      cleared_at: cleared ? "2026-07-20T00:00:00Z" : null,
      source: "manual",
      note: null,
      created_at: "",
      updated_at: "",
    }) as StudentStatusDay;

  it("returns the active status-day row for the date", () => {
    const result = resolveDayDeviation("2026-07-20", [
      statusDay("2026-07-20", "sick"),
    ]);
    expect(result).toEqual({ kind: "sick", label: "Krank", note: null });
  });

  it("ignores a cleared status-day row", () => {
    const result = resolveDayDeviation("2026-07-20", [
      statusDay("2026-07-20", "sick", true),
    ]);
    expect(result).toBeNull();
  });

  it("falls back to the live sick flag only for today", () => {
    expect(
      resolveDayDeviation("2026-07-20", [], { isSick: true, isToday: true }),
    ).toEqual({ kind: "sick", label: "Krank" });
    expect(
      resolveDayDeviation("2026-07-20", [], { isSick: true, isToday: false }),
    ).toBeNull();
  });

  it("falls back to the live excused flag for today", () => {
    expect(
      resolveDayDeviation("2026-07-20", [], { isExcused: true, isToday: true }),
    ).toEqual({ kind: "excused", label: "Entschuldigt" });
  });

  it("returns the class_trip status-day row", () => {
    const result = resolveDayDeviation("2026-07-20", [
      statusDay("2026-07-20", "class_trip"),
    ]);
    expect(result).toEqual({
      kind: "class_trip",
      label: "Entschuldigt",
      note: null,
    });
  });

  it("returns null when there is no deviation", () => {
    expect(resolveDayDeviation("2026-07-20", [])).toBeNull();
  });
});
