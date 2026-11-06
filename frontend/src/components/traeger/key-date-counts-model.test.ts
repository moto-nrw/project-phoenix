import { describe, expect, it } from "vitest";
import {
  capturedLate,
  formatKeyDateChange,
  formatKeyDateMonth,
  keyDateChange,
  selectKeyDateMonth,
} from "./key-date-counts-model";

const october = {
  period: "2026-10-01",
  keyDate: "2026-10-15",
  activeStudents: 10,
  activeTerminals: 1,
  schools: 1,
};
const september = { ...october, period: "2026-09-01", keyDate: "2026-09-15" };
const school = {
  id: "2",
  name: "OGS",
  slug: "ogs",
  subdomain: "ogs",
  city: "",
  counts: [
    {
      period: "2026-10-01",
      keyDate: "2026-10-15",
      activeStudents: 10,
      activeTerminals: 1,
      recordedAt: "2026-10-15T04:00:00Z",
    },
  ],
};

describe("selectKeyDateMonth", () => {
  it("falls back to the newest month and pairs it with the month before", () => {
    const selection = selectKeyDateMonth(
      [school],
      [october, september],
      "2026-01-01",
    );
    expect(selection?.month).toBe(october);
    expect(selection?.previous).toBe(september);
    expect(selection?.rows[0]?.count?.activeStudents).toBe(10);
    expect(selection?.rows[0]?.previous).toBeNull();
  });

  it("has no previous month for the first captured month and nothing without months", () => {
    expect(
      selectKeyDateMonth([school], [october, september], "2026-09-01")
        ?.previous,
    ).toBeNull();
    expect(selectKeyDateMonth([school], [], null)).toBeNull();
  });
});

describe("key-date figures", () => {
  it("formats changes and months in German", () => {
    expect(formatKeyDateChange(3)).toBe("+3");
    expect(formatKeyDateChange(-2)).toBe("−2");
    expect(formatKeyDateChange(0)).toBe("±0");
    expect(keyDateChange(5, undefined)).toBeNull();
    expect(formatKeyDateMonth("2026-10-01")).toBe("Oktober 2026");
  });

  it("marks a capture after the key date on the Berlin calendar", () => {
    const base = {
      period: "2026-10-01",
      keyDate: "2026-10-15",
      activeStudents: 1,
      activeTerminals: 1,
    };
    // 23:30 UTC on the 14th is already the 15th in Berlin: on time.
    expect(capturedLate({ ...base, recordedAt: "2026-10-14T23:30:00Z" })).toBe(
      false,
    );
    expect(capturedLate({ ...base, recordedAt: "2026-10-15T22:30:00Z" })).toBe(
      true,
    );
  });
});
