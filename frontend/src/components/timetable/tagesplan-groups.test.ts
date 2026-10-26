import { describe, expect, it } from "vitest";

import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

import { groupParallelBlocks } from "./tagesplan-groups";

function makeInstance(
  overrides: Partial<PlannedTimetableInstance> & { id: string },
): PlannedTimetableInstance {
  return {
    title: "Lernzeit",
    date: "2026-09-09",
    startTime: "09:00",
    endTime: "09:45",
    roomId: "5",
    roomName: "Lernraum",
    status: "planned",
    isOverdue: false,
    minutesUntilStart: 60,
    expectedStudentsCount: 8,
    presentStudentsCount: 0,
    notScheduledStudentsCount: 0,
    assignedStaffIds: [],
    isAssigned: false,
    isPrimary: false,
    isSubstitute: false,
    isAbsent: false,
    rosterPreview: [],
    ...overrides,
  };
}

describe("groupParallelBlocks", () => {
  it("keeps a regular block ending at midnight in its last group", () => {
    const groups = groupParallelBlocks(
      [
        makeInstance({
          id: "regular",
          startTime: "23:00",
          endTime: "24:00",
        }),
      ],
      null,
    );

    expect(groups[0]?.endTime).toBe("24:00");
  });

  it("keeps an open block with a midnight placeholder in its last group", () => {
    const groups = groupParallelBlocks(
      [
        makeInstance({
          id: "running",
          startTime: "23:00",
          endTime: "24:00",
          status: "active",
          isSpontaneous: true,
        }),
      ],
      "23:30",
    );

    expect(groups[0]?.endTime).toBe("24:00");
  });

  it("keeps an active spontaneous block at 23:59 in the last overlap group", () => {
    const groups = groupParallelBlocks(
      [
        makeInstance({
          id: "running",
          title: "Spontanes Basteln",
          startTime: "23:00",
          endTime: "23:01",
          status: "active",
          isSpontaneous: true,
        }),
        makeInstance({
          id: "last-minute",
          title: "Abschluss",
          startTime: "23:59",
          endTime: "23:59",
        }),
      ],
      "23:59",
    );

    expect(groups).toHaveLength(1);
    expect(groups[0]).toMatchObject({ laneCount: 2, endTime: "24:00" });
    expect(groups[0]?.cells.map((cell) => cell.instance.id)).toEqual([
      "running",
      "last-minute",
    ]);
  });

  it("keeps an open block past its stale placeholder end for the now line", () => {
    const groups = groupParallelBlocks(
      [
        makeInstance({
          id: "running",
          startTime: "09:00",
          endTime: "09:30",
          status: "active",
          isSpontaneous: true,
        }),
      ],
      "10:00",
    );

    expect(groups[0]?.endTime).toBe("10:01");
  });
});
