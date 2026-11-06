import { describe, expect, it } from "vitest";
import {
  alertSentence,
  schoolCountLabel,
  thresholdsSentence,
} from "./traeger-alerts";

function school(
  figures: Partial<{
    childrenTotal: number;
    childrenPresent: number;
    childrenSick: number;
    staffPresent: number;
    careRatio: number | null;
  }>,
  openEnrollments = 0,
) {
  return {
    figures: {
      childrenTotal: 0,
      childrenPresent: 0,
      childrenSick: 0,
      staffTotal: 0,
      staffPresent: 0,
      careRatio: null,
      asOf: "2026-09-09T10:00:00Z",
      ...figures,
    },
    openEnrollments,
  };
}

describe("alertSentence", () => {
  it("names the children and staff behind a staff shortage", () => {
    expect(
      alertSentence(
        "staff_shortage",
        school({ childrenPresent: 25, staffPresent: 1, careRatio: 25 }),
      ),
    ).toBe("25 Kinder auf 1 Betreuungskraft, Schlüssel 1 : 25.");
    expect(
      alertSentence(
        "staff_shortage",
        school({ childrenPresent: 45, staffPresent: 2, careRatio: 22.5 }),
      ),
    ).toBe("45 Kinder auf 2 Betreuungskräfte, Schlüssel 1 : 22,5.");
  });

  it("says when children are there without any staff", () => {
    expect(
      alertSentence("staff_shortage", school({ childrenPresent: 1 })),
    ).toBe(
      "1 Kind ist da. Keine Betreuungskraft ist in der Zeiterfassung angemeldet.",
    );
  });

  it("gives the sick children with their share", () => {
    expect(
      alertSentence(
        "sick_children",
        school({ childrenTotal: 115, childrenSick: 6 }),
      ),
    ).toMatch(/^6 von 115 Kindern sind heute krank \(5\s%\)\.$/);
  });

  it("counts the waiting children", () => {
    expect(alertSentence("open_enrollments", school({}, 1))).toBe(
      "1 Kind wartet auf eine Entscheidung.",
    );
    expect(alertSentence("open_enrollments", school({}, 6))).toBe(
      "6 Kinder warten auf eine Entscheidung.",
    );
  });
});

describe("thresholdsSentence", () => {
  it("states every threshold once and a switched-off one as off", () => {
    expect(
      thresholdsSentence({
        childrenPerStaff: 20,
        sickPercent: 10,
        openEnrollments: 1,
      }),
    ).toBe(
      "Ihre Grenzen: mehr als 20 Kinder je Betreuungskraft · ab 10 % kranke Kinder · ab 1 offenen Anmeldung",
    );
    expect(
      thresholdsSentence({
        childrenPerStaff: 0,
        sickPercent: 0,
        openEnrollments: 3,
      }),
    ).toBe(
      "Ihre Grenzen: Personal: aus · Krankmeldungen: aus · ab 3 offenen Anmeldungen",
    );
  });
});

describe("schoolCountLabel", () => {
  it("counts schools in the singular and plural", () => {
    expect(schoolCountLabel(1)).toBe("1 Schule");
    expect(schoolCountLabel(4)).toBe("4 Schulen");
  });
});
