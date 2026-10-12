import { describe, expect, it } from "vitest";
import {
  presentChildCandidates,
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
  const today = "2026-08-31";
  const at = (hours: number, minutes: number) =>
    new Date(
      `2026-08-31T${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:00+02:00`,
    );

  it("returns the expected arrival while it is still ahead", () => {
    expect(
      upcomingArrivalTime(arrivalWarnings("13:45"), at(13, 0), today),
    ).toBe("13:45");
  });

  it("returns null once the expected arrival is reached", () => {
    expect(
      upcomingArrivalTime(arrivalWarnings("13:45"), at(13, 45), today),
    ).toBeNull();
    expect(
      upcomingArrivalTime(arrivalWarnings("13:45"), at(14, 0), today),
    ).toBeNull();
  });

  it("ignores other warning kinds and missing times", () => {
    expect(
      upcomingArrivalTime(arrivalWarnings(null), at(13, 0), today),
    ).toBeNull();
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
        today,
      ),
    ).toBeNull();
  });

  it("uses the Berlin clock instead of the browser clock", () => {
    // 13:00 in Berlin, but 06:00 in Chicago on the same instant.
    expect(
      upcomingArrivalTime(
        arrivalWarnings("13:45"),
        new Date("2026-08-31T11:00:00Z"),
        today,
      ),
    ).toBe("13:45");
  });

  it("does not classify arrivals on another roster date as late", () => {
    expect(
      upcomingArrivalTime(arrivalWarnings("13:45"), at(13, 0), "2026-09-01"),
    ).toBeNull();
  });
});

describe("presentChildCandidates", () => {
  // 2026-09-09 12:00 Berlin, the shared test clock instant.
  const noon = new Date("2026-09-09T10:00:00Z");
  const children = [
    { id: "1", pickupTime: "11:30" },
    { id: "2", pickupTime: "12:00" },
    { id: "3", pickupTime: "12:01" },
    { id: "4", pickupTime: "16:00" },
    { id: "5", pickupTime: null },
    { id: "6", pickupTime: "16:00" },
  ];

  it("keeps children whose Gehzeit is still ahead and those without one", () => {
    const ids = presentChildCandidates(children, new Set(), noon, "stays").map(
      (child) => child.id,
    );
    expect(ids).toEqual(["3", "4", "5", "6"]);
  });

  it("offers every present child under all", () => {
    const ids = presentChildCandidates(children, new Set(), noon, "all").map(
      (child) => child.id,
    );
    expect(ids).toEqual(["1", "2", "3", "4", "5", "6"]);
  });

  it("never offers a child that is already in the block", () => {
    const ids = presentChildCandidates(
      children,
      new Set(["4", "1"]),
      noon,
      "all",
    ).map((child) => child.id);
    expect(ids).toEqual(["2", "3", "5", "6"]);
  });
});
