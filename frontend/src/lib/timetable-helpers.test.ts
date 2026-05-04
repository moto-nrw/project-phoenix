import { describe, expect, it } from "vitest";

import {
  assignBlockLanes,
  getCurrentTimeOffset,
  getEventBlockPosition,
  parseTimeToMinutes,
} from "./timetable-helpers";
import type { EnrichedInstance } from "./timetable-types";

// Minimal helper — assignBlockLanes only inspects startTime/endTime.
function fakeInstance(
  id: string,
  startTime: string,
  endTime: string,
): EnrichedInstance {
  return {
    id,
    date: "2026-04-29",
    startTime,
    endTime,
    title: id,
    description: undefined,
    notes: undefined,
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "activity",
    roomId: "1",
    roomName: "Raum",
    staff: [],
    students: [],
    studentIds: [],
    staffCount: 0,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    presentStudentsCount: 0,
    conflictWarnings: [],
  } as unknown as EnrichedInstance;
}

describe("parseTimeToMinutes", () => {
  it("returns minutes since midnight for valid HH:MM", () => {
    expect(parseTimeToMinutes("00:00")).toBe(0);
    expect(parseTimeToMinutes("09:30")).toBe(570);
    expect(parseTimeToMinutes("23:59")).toBe(23 * 60 + 59);
  });

  it("returns NaN for malformed input", () => {
    expect(Number.isNaN(parseTimeToMinutes("not-a-time"))).toBe(true);
    expect(Number.isNaN(parseTimeToMinutes(""))).toBe(true);
  });
});

describe("getEventBlockPosition", () => {
  it("places a 1h event at the hour line with correct height", () => {
    const { top, height } = getEventBlockPosition("10:00", "11:00", 90, 9);
    expect(top).toBe(90);
    expect(height).toBe(90);
  });

  it("computes fractional positions for half-hour events", () => {
    const { top, height } = getEventBlockPosition("09:30", "10:15", 90, 9);
    expect(top).toBe(45);
    expect(height).toBe(67.5);
  });

  it("clamps tiny events to the minimum height", () => {
    const { height } = getEventBlockPosition("10:00", "10:05", 90, 9);
    expect(height).toBeGreaterThanOrEqual(24);
  });

  it("returns negative top for events before the visible day start", () => {
    const { top } = getEventBlockPosition("08:00", "09:00", 90, 9);
    expect(top).toBe(-90);
  });
});

describe("getCurrentTimeOffset", () => {
  it("returns null when now is outside the visible window", () => {
    const before = new Date("2026-04-29T05:00:00");
    expect(getCurrentTimeOffset(90, 9, 17, before)).toBeNull();

    const after = new Date("2026-04-29T20:00:00");
    expect(getCurrentTimeOffset(90, 9, 17, after)).toBeNull();
  });

  it("returns the correct pixel offset when now is inside the window", () => {
    const noon = new Date("2026-04-29T12:30:00");
    // 12:30 = 3.5h after 09:00 → 3.5 * 90 = 315
    expect(getCurrentTimeOffset(90, 9, 17, noon)).toBe(315);
  });
});

describe("assignBlockLanes", () => {
  it("returns empty array for empty input", () => {
    expect(assignBlockLanes([])).toEqual([]);
  });

  it("gives a single non-overlapping event the full column", () => {
    const result = assignBlockLanes([fakeInstance("a", "10:00", "11:00")]);
    expect(result).toHaveLength(1);
    expect(result[0]!.lane).toBe(0);
    expect(result[0]!.laneCount).toBe(1);
  });

  it("keeps two non-overlapping events in their own clusters at full width", () => {
    const result = assignBlockLanes([
      fakeInstance("a", "10:00", "11:00"),
      fakeInstance("b", "11:00", "12:00"),
    ]);
    expect(result).toHaveLength(2);
    for (const r of result) {
      expect(r.lane).toBe(0);
      expect(r.laneCount).toBe(1);
    }
  });

  it("splits two overlapping events into 2 lanes", () => {
    const result = assignBlockLanes([
      fakeInstance("a", "10:00", "12:00"),
      fakeInstance("b", "11:00", "13:00"),
    ]);
    expect(result).toHaveLength(2);
    const a = result.find((r) => r.instance.id === "a")!;
    const b = result.find((r) => r.instance.id === "b")!;
    expect(a.lane).toBe(0);
    expect(b.lane).toBe(1);
    expect(a.laneCount).toBe(2);
    expect(b.laneCount).toBe(2);
  });

  it("reuses a freed lane when an earlier event has ended within the cluster", () => {
    // a: 10–11   b: 10:30–12   c: 11–12 → c can reuse a's lane (0)
    const result = assignBlockLanes([
      fakeInstance("a", "10:00", "11:00"),
      fakeInstance("b", "10:30", "12:00"),
      fakeInstance("c", "11:00", "12:00"),
    ]);
    const a = result.find((r) => r.instance.id === "a")!;
    const b = result.find((r) => r.instance.id === "b")!;
    const c = result.find((r) => r.instance.id === "c")!;
    expect(a.lane).toBe(0);
    expect(b.lane).toBe(1);
    expect(c.lane).toBe(0);
    // Cluster size is 2 (max parallel at any time), all members agree.
    expect(a.laneCount).toBe(2);
    expect(b.laneCount).toBe(2);
    expect(c.laneCount).toBe(2);
  });
});
