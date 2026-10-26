import { describe, expect, it } from "vitest";

import { roomDayPlan, type RoomPlanRow } from "./room-plan";
import type { EnrichedInstance } from "./timetable-types";

function block(
  id: string,
  startTime: string,
  endTime: string,
  status: EnrichedInstance["status"] = "planned",
): EnrichedInstance {
  const instance: EnrichedInstance = {
    id,
    date: "2026-09-09",
    startTime,
    endTime,
    title: `Block ${id}`,
    status,
    isSpontaneous: false,
    isLive: false,
    activityType: "activity",
    roomId: "3",
    roomName: "Werkraum",
    staff: [],
    studentIds: [],
    students: [],
    staffCount: 0,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    presentStudentsCount: 0,
    notScheduledStudentsCount: 0,
    requiredStaffCount: 0,
    assignedStaffCount: 0,
    conflictWarnings: [],
  };
  return instance;
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
