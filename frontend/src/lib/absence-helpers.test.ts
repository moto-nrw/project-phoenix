import { describe, it, expect } from "vitest";

import {
  absenceStatusMeta,
  countWorkdaysInclusive,
  dayCountFor,
  formatAbsenceRange,
  formatDayCount,
} from "./absence-helpers";

describe("absenceStatusMeta", () => {
  it("maps every workflow status to a German label", () => {
    expect(absenceStatusMeta("requested").label).toBe("Wartet");
    expect(absenceStatusMeta("question").label).toBe("Rückfrage");
    expect(absenceStatusMeta("approved").label).toBe("Genehmigt");
    expect(absenceStatusMeta("declined").label).toBe("Abgelehnt");
    expect(absenceStatusMeta("canceled").label).toBe("Storniert");
    expect(absenceStatusMeta("reported").label).toBe("Eingetragen");
  });

  it("lets the MA card override the requested label", () => {
    expect(
      absenceStatusMeta("requested", { requestedLabel: "Wartet auf Antwort" })
        .label,
    ).toBe("Wartet auf Antwort");
    // Other statuses ignore the override.
    expect(
      absenceStatusMeta("question", { requestedLabel: "Wartet auf Antwort" })
        .label,
    ).toBe("Rückfrage");
  });

  it("falls back to the raw status for unknown values", () => {
    expect(absenceStatusMeta("weird").label).toBe("weird");
  });
});

describe("countWorkdaysInclusive", () => {
  it("counts Mon-Fri inclusively", () => {
    // 2027-07-05 (Mon) .. 2027-07-11 (Sun) = 5 workdays
    expect(countWorkdaysInclusive("2027-07-05", "2027-07-11")).toBe(5);
  });

  it("returns 0 for inverted ranges", () => {
    expect(countWorkdaysInclusive("2027-07-11", "2027-07-05")).toBe(0);
  });
});

describe("dayCountFor", () => {
  it("prefers the backend-computed workingDays", () => {
    expect(
      dayCountFor({
        workingDays: 2.5,
        dateStart: "2027-07-05",
        dateEnd: "2027-07-09",
        hasBoundaryFields: true,
      }),
    ).toBe(2.5);
  });

  it("subtracts boundary half days on workdays", () => {
    expect(
      dayCountFor({
        dateStart: "2027-07-05",
        dateEnd: "2027-07-09",
        startHalfDay: true,
        endHalfDay: true,
        hasBoundaryFields: true,
      }),
    ).toBe(4);
  });

  it("uses the legacy halfDay flag when no boundary fields exist", () => {
    expect(
      dayCountFor({
        dateStart: "2027-07-05",
        dateEnd: "2027-07-05",
        halfDay: true,
        hasBoundaryFields: false,
      }),
    ).toBe(0.5);
  });
});

describe("formatDayCount", () => {
  it("formats singular, plural and decimals German-style", () => {
    expect(formatDayCount(1)).toBe("1 Tag");
    expect(formatDayCount(5)).toBe("5 Tage");
    expect(formatDayCount(2.5)).toBe("2,5 Tage");
  });
});

describe("formatAbsenceRange", () => {
  it("collapses single-day ranges", () => {
    expect(formatAbsenceRange("2027-07-05", "2027-07-05")).toBe("05.07.2027");
    expect(formatAbsenceRange("2027-07-05", "2027-07-06")).toBe(
      "05.07.2027 - 06.07.2027",
    );
  });
});
