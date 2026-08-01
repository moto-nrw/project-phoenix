import { describe, it, expect, vi } from "vitest";

import {
  ABSENCES_REFRESH_EVENT,
  absenceTypeNoun,
  absenceStatusMeta,
  countWorkdaysInclusive,
  dayCountFor,
  dispatchAbsencesRefresh,
  formatAbsenceRange,
  formatDayCount,
} from "./absence-helpers";

describe("absenceTypeNoun", () => {
  it("maps known types and falls back to the generic noun", () => {
    expect(absenceTypeNoun("sick")).toBe("Krankmeldung");
    expect(absenceTypeNoun("vacation")).toBe("Urlaub");
    expect(absenceTypeNoun("training")).toBe("Fortbildung");
    expect(absenceTypeNoun("other")).toBe("Abwesenheit");
  });
});

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

  it("returns 0 for a weekend-only range", () => {
    expect(countWorkdaysInclusive("2027-07-10", "2027-07-11")).toBe(0);
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

  it("uses the legacy halfDay flag when serialized boundary fields are false", () => {
    expect(
      dayCountFor({
        dateStart: "2027-07-05",
        dateEnd: "2027-07-05",
        halfDay: true,
        startHalfDay: false,
        endHalfDay: false,
        hasBoundaryFields: true,
      }),
    ).toBe(0.5);
  });

  it("keeps a full legacy day when halfDay is false", () => {
    expect(
      dayCountFor({
        dateStart: "2027-07-05",
        dateEnd: "2027-07-05",
        halfDay: false,
        hasBoundaryFields: false,
      }),
    ).toBe(1);
  });

  it("returns 0 before applying half-day boundaries to an empty range", () => {
    expect(
      dayCountFor({
        dateStart: "2027-07-10",
        dateEnd: "2027-07-11",
        startHalfDay: true,
        endHalfDay: true,
        hasBoundaryFields: true,
      }),
    ).toBe(0);
  });

  it("handles a same-day end-half-day boundary without double subtraction", () => {
    expect(
      dayCountFor({
        dateStart: "2027-07-05",
        dateEnd: "2027-07-05",
        startHalfDay: false,
        endHalfDay: true,
        hasBoundaryFields: true,
      }),
    ).toBe(0.5);
    expect(
      dayCountFor({
        dateStart: "2027-07-05",
        dateEnd: "2027-07-05",
        startHalfDay: true,
        endHalfDay: true,
        hasBoundaryFields: true,
      }),
    ).toBe(0.5);
  });

  it("does not subtract half days from weekend boundaries", () => {
    expect(
      dayCountFor({
        dateStart: "2027-07-04",
        dateEnd: "2027-07-10",
        startHalfDay: true,
        endHalfDay: true,
        hasBoundaryFields: true,
      }),
    ).toBe(5);
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

describe("dispatchAbsencesRefresh", () => {
  it("dispatches the shared refresh event", () => {
    const listener = vi.fn();
    window.addEventListener(ABSENCES_REFRESH_EVENT, listener);

    dispatchAbsencesRefresh();

    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener(ABSENCES_REFRESH_EVENT, listener);
  });
});
