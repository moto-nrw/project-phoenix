import { describe, expect, it } from "vitest";

import {
  expandClosingDaysToMap,
  findClosingDayReason,
  findFirstClosingDayConflict,
  formatClosingDayRange,
  type ClosingDayRange,
} from "./closing-day-helpers";

const WEIHNACHTEN: ClosingDayRange = {
  startDate: "2026-12-24",
  endDate: "2026-12-31",
  reason: "Weihnachtsferien",
};
const PAED_TAG: ClosingDayRange = {
  startDate: "2026-11-02",
  endDate: "2026-11-02",
  reason: "Pädagogischer Tag",
};

describe("expandClosingDaysToMap", () => {
  it("expands a range into one entry per calendar day", () => {
    const map = expandClosingDaysToMap(
      [WEIHNACHTEN],
      "2026-12-01",
      "2026-12-31",
    );

    expect(map.size).toBe(8);
    expect(map.get("2026-12-24")).toBe("Weihnachtsferien");
    expect(map.get("2026-12-31")).toBe("Weihnachtsferien");
    expect(map.has("2026-12-23")).toBe(false);
  });

  it("clamps a range to the requested window", () => {
    const map = expandClosingDaysToMap(
      [WEIHNACHTEN],
      "2026-12-29",
      "2027-01-05",
    );

    expect([...map.keys()]).toEqual(["2026-12-29", "2026-12-30", "2026-12-31"]);
  });

  it("keeps the first reason when ranges overlap", () => {
    const map = expandClosingDaysToMap(
      [
        PAED_TAG,
        { startDate: "2026-11-01", endDate: "2026-11-03", reason: "Brücke" },
      ],
      "2026-11-01",
      "2026-11-03",
    );

    expect(map.get("2026-11-02")).toBe("Pädagogischer Tag");
    expect(map.get("2026-11-01")).toBe("Brücke");
  });

  it("returns an empty map for an inverted window or missing data", () => {
    expect(
      expandClosingDaysToMap([WEIHNACHTEN], "2026-12-31", "2026-12-01").size,
    ).toBe(0);
    expect(
      expandClosingDaysToMap(undefined, "2026-12-01", "2026-12-31").size,
    ).toBe(0);
  });
});

describe("findClosingDayReason", () => {
  it("finds the reason of the range covering the date", () => {
    expect(findClosingDayReason([WEIHNACHTEN, PAED_TAG], "2026-12-27")).toBe(
      "Weihnachtsferien",
    );
    expect(findClosingDayReason([WEIHNACHTEN, PAED_TAG], "2026-11-02")).toBe(
      "Pädagogischer Tag",
    );
  });

  it("returns undefined outside every range, for an empty date and no data", () => {
    expect(
      findClosingDayReason([WEIHNACHTEN, PAED_TAG], "2026-12-23"),
    ).toBeUndefined();
    expect(findClosingDayReason([WEIHNACHTEN], "")).toBeUndefined();
    expect(findClosingDayReason(undefined, "2026-12-24")).toBeUndefined();
  });
});

describe("findFirstClosingDayConflict", () => {
  it("returns the first generated occurrence inside a closing range", () => {
    expect(
      findFirstClosingDayConflict(
        [WEIHNACHTEN, PAED_TAG],
        ["2026-11-01", "2026-11-02", "2026-12-24"],
      ),
    ).toEqual({
      dateISO: "2026-11-02",
      reason: "Pädagogischer Tag",
    });
  });

  it("returns null when generated dates do not overlap", () => {
    expect(
      findFirstClosingDayConflict([WEIHNACHTEN], ["2026-12-23"]),
    ).toBeNull();
  });
});

describe("formatClosingDayRange", () => {
  it("collapses single-day ranges", () => {
    expect(formatClosingDayRange({ id: "1", ...PAED_TAG })).toBe("02.11.2026");
    expect(formatClosingDayRange({ id: "2", ...WEIHNACHTEN })).toBe(
      "24.12.2026 – 31.12.2026",
    );
  });
});
