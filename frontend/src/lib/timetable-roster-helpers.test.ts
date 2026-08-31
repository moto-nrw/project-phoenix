import { describe, expect, it } from "vitest";
import {
  rosterPickupTimeLabel,
  upcomingArrivalTime,
} from "./timetable-roster-helpers";
import type { TimetableRosterRow } from "./timetable-operations-types";

describe("rosterPickupTimeLabel", () => {
  it("hides intentionally redacted pickup times without reporting a load error", () => {
    expect(rosterPickupTimeLabel(null, false, true)).toBeNull();
  });

  it("keeps failed and missing pickup times distinct", () => {
    expect(rosterPickupTimeLabel(null, false)).toBe("Nicht geladen");
    expect(rosterPickupTimeLabel(null, true)).toBe("—");
  });
});

function arrivalWarnings(
  expectedArrival: string | null,
): TimetableRosterRow["warnings"] {
  return [
    {
      kind: "arrival_after_slot_start",
      message: "Erwartete Ankunft liegt nach dem Start dieser Betreuung.",
      expectedArrival,
      slotStart: "13:00",
      expectedGroupId: null,
      expectedGroupName: null,
      currentEducationGroupId: null,
    },
  ];
}

describe("upcomingArrivalTime", () => {
  const at = (hours: number, minutes: number) =>
    new Date(2026, 7, 31, hours, minutes);

  it("returns the expected arrival while it is still ahead", () => {
    expect(upcomingArrivalTime(arrivalWarnings("13:45"), at(13, 0))).toBe(
      "13:45",
    );
  });

  it("returns null once the expected arrival is reached", () => {
    expect(
      upcomingArrivalTime(arrivalWarnings("13:45"), at(13, 45)),
    ).toBeNull();
    expect(upcomingArrivalTime(arrivalWarnings("13:45"), at(14, 0))).toBeNull();
  });

  it("ignores other warning kinds and missing times", () => {
    expect(upcomingArrivalTime(arrivalWarnings(null), at(13, 0))).toBeNull();
    expect(
      upcomingArrivalTime(
        [
          {
            kind: "missing_arrival_schedule",
            message: "Für diesen Tag ist keine erwartete Ankunft hinterlegt.",
            expectedArrival: null,
            slotStart: "13:00",
            expectedGroupId: null,
            expectedGroupName: null,
            currentEducationGroupId: null,
          },
        ],
        at(13, 0),
      ),
    ).toBeNull();
  });
});
