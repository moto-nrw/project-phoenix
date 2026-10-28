import { describe, expect, it } from "vitest";
import { setTestClock } from "~/test/clock";

import {
  roomDayPlan,
  roomPlanCareDay,
  roomPlanDayLabel,
  roomPlanStepDay,
  roomPlanStepWeek,
  roomPlanWeekDays,
  type RoomPlanBlock,
  type RoomPlanRow,
} from "./room-plan";

function block(
  id: string,
  startTime: string,
  endTime: string,
  status = "planned",
): RoomPlanBlock {
  return {
    id,
    date: "2026-09-09",
    startTime,
    endTime,
    title: `Block ${id}`,
    status,
  };
}

function describeRows(rows: RoomPlanRow[]): string[] {
  return rows.map((row) =>
    row.kind === "free"
      ? `frei ${row.start}-${row.end}`
      : `${row.instance.id} ${row.instance.status}`,
  );
}

describe("roomDayPlan (#4768)", () => {
  it("shows the whole planning day as free without blocks", () => {
    expect(describeRows(roomDayPlan([], "08:00", "17:00"))).toEqual([
      "frei 08:00-17:00",
    ]);
  });

  it("puts free times between the blocks in start order", () => {
    const rows = roomDayPlan(
      [block("b", "14:00", "15:30"), block("a", "11:30", "13:00")],
      "08:00",
      "17:00",
    );
    expect(describeRows(rows)).toEqual([
      "frei 08:00-11:30",
      "a planned",
      "frei 13:00-14:00",
      "b planned",
      "frei 15:30-17:00",
    ]);
  });

  it("lets overlapping blocks share the room without a gap", () => {
    const rows = roomDayPlan(
      [block("a", "12:00", "14:00"), block("b", "13:00", "15:00")],
      "12:00",
      "15:00",
    );
    expect(describeRows(rows)).toEqual(["a planned", "b planned"]);
  });

  it("lists a cancelled block without counting it as occupied", () => {
    const rows = roomDayPlan(
      [block("a", "10:00", "11:00", "cancelled"), block("b", "12:00", "13:00")],
      "09:00",
      "13:00",
    );
    expect(describeRows(rows)).toEqual([
      "frei 09:00-12:00",
      "a cancelled",
      "b planned",
    ]);
  });

  it("keeps free times inside the planning day", () => {
    const rows = roomDayPlan(
      [block("a", "07:00", "09:00"), block("b", "16:00", "19:00")],
      "08:00",
      "17:00",
    );
    expect(describeRows(rows)).toEqual([
      "a planned",
      "frei 09:00-16:00",
      "b planned",
    ]);
  });

  it("keeps minute-accurate planning bounds", () => {
    expect(describeRows(roomDayPlan([], "08:30", "16:15"))).toEqual([
      "frei 08:30-16:15",
    ]);
  });
});

describe("room plan days (#4768)", () => {
  it("snaps the weekend to Monday unless the school cares on weekends", () => {
    expect(roomPlanCareDay("2026-09-12", false)).toBe("2026-09-14");
    expect(roomPlanCareDay("2026-09-13", false)).toBe("2026-09-14");
    expect(roomPlanCareDay("2026-09-12", true)).toBe("2026-09-14");
    setTestClock("2026-09-12T12:00:00+02:00"); // Saturday
    expect(roomPlanCareDay("2026-09-12", true)).toBe("2026-09-12");
    expect(roomPlanCareDay("2026-09-09", false)).toBe("2026-09-09");
  });

  it("steps from care day to care day", () => {
    expect(roomPlanStepDay("2026-09-11", 1, false)).toBe("2026-09-14");
    expect(roomPlanStepDay("2026-09-14", -1, false)).toBe("2026-09-11");
    expect(roomPlanStepDay("2026-09-11", 1, true)).toBe("2026-09-14");
    setTestClock("2026-09-12T12:00:00+02:00"); // Saturday
    expect(roomPlanStepDay("2026-09-11", 1, true)).toBe("2026-09-12");
    expect(roomPlanStepDay("2026-09-14", -1, true)).toBe("2026-09-13");
    expect(roomPlanStepDay("2026-09-18", 1, true)).toBe("2026-09-21");
  });

  it("lists the week from Monday and steps by whole weeks", () => {
    expect(roomPlanWeekDays("2026-09-09", false)).toEqual([
      "2026-09-07",
      "2026-09-08",
      "2026-09-09",
      "2026-09-10",
      "2026-09-11",
    ]);
    expect(roomPlanStepWeek("2026-09-07", -1)).toBe("2026-08-31");
  });

  it("keeps the week Mo–Fr on a weekday although the weekend follows Friday", () => {
    // Test clock: Wednesday 2026-09-09.
    expect(roomPlanWeekDays("2026-09-09", true)).toHaveLength(5);
  });

  it("lists Sa/So for the week of today's weekend", () => {
    setTestClock("2026-09-13T12:00:00+02:00"); // Sunday
    expect(roomPlanWeekDays("2026-09-13", true)).toHaveLength(7);
    expect(roomPlanWeekDays("2026-09-13", true)[0]).toBe("2026-09-07");
    expect(roomPlanWeekDays("2026-09-14", true)).toHaveLength(5);
  });

  it("names the day in full", () => {
    expect(roomPlanDayLabel("2026-10-27")).toBe("Dienstag, 27.10.2026");
  });
});
