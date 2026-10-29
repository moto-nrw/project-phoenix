import { describe, expect, it } from "vitest";

import {
  connectionBody,
  describeRosterDeviation,
  describeSelectionFilter,
  laterSegmentSentence,
  mapOfferingPreview,
  mapRosterDeviations,
  previewQuery,
  untouchedSegmentSentence,
  type RosterDeviation,
} from "./roster-deviation";

const deviation: RosterDeviation = {
  templateId: "7",
  offering: { id: "3", name: "Randstunde" },
  gradeLevels: [],
  schoolClasses: ["1a"],
  unbookedCount: 8,
  missingCount: 0,
  candidates: [{ id: "3", name: "Randstunde" }],
  ambiguous: false,
};

describe("describeRosterDeviation", () => {
  it("names the children without booking and the matching offering", () => {
    expect(describeRosterDeviation(deviation)).toBe(
      "Weicht von Anmeldungen ab: 8 Kinder ohne Buchung · passt zu „Randstunde“",
    );
  });

  it("adds missing booked children only when there are some, in singular", () => {
    expect(
      describeRosterDeviation({
        ...deviation,
        unbookedCount: 1,
        missingCount: 1,
      }),
    ).toBe(
      "Weicht von Anmeldungen ab: 1 Kind ohne Buchung · 1 angemeldetes Kind fehlt · passt zu „Randstunde“",
    );
    expect(
      describeRosterDeviation({
        ...deviation,
        unbookedCount: 0,
        missingCount: 2,
      }),
    ).toBe(
      "Weicht von Anmeldungen ab: 2 angemeldete Kinder fehlen · passt zu „Randstunde“",
    );
  });

  it("names every fitting offering and no count when several fit equally (#5138)", () => {
    expect(
      describeRosterDeviation({
        ...deviation,
        candidates: [
          { id: "4", name: "Ganztag" },
          { id: "3", name: "Randstunde" },
          { id: "5", name: "Spätbetreuung" },
        ],
        ambiguous: true,
      }),
    ).toBe(
      "Weicht von Anmeldungen ab: passt zu „Ganztag“, „Randstunde“ oder „Spätbetreuung“",
    );
  });
});

describe("mapRosterDeviations", () => {
  it("turns backend ids into strings and defaults empty lists", () => {
    expect(
      mapRosterDeviations({
        deviations: [
          {
            template_id: 7,
            offering: { id: 3, name: "Randstunde" },
            grade_levels: null,
            school_classes: ["1a"],
            unbooked_count: 8,
            missing_count: 0,
            candidates: [{ id: 3, name: "Randstunde" }],
          },
        ],
      }),
    ).toEqual([deviation]);
    expect(
      mapRosterDeviations({
        deviations: [
          {
            template_id: 7,
            offering: { id: 3, name: "Randstunde" },
            school_classes: ["1a"],
            unbooked_count: 8,
            missing_count: 0,
            candidates: [{ id: 3, name: "Randstunde" }],
            ambiguous: true,
          },
        ],
      })[0]?.ambiguous,
    ).toBe(true);
    expect(mapRosterDeviations({ deviations: null })).toEqual([]);
  });
});

describe("selection helpers", () => {
  it("describes the filter in the words of the editor", () => {
    expect(
      describeSelectionFilter({
        offeringIds: ["3"],
        gradeLevels: [],
        schoolClasses: ["1a"],
      }),
    ).toBe("Nur Kinder der Klasse 1a");
    expect(
      describeSelectionFilter({
        offeringIds: ["3"],
        gradeLevels: [],
        schoolClasses: ["1a", "1b"],
      }),
    ).toBe("Nur Kinder der Klassen 1a und 1b");
    // Schools that name their classes „Klasse 1b“ get no double word.
    expect(
      describeSelectionFilter({
        offeringIds: ["3"],
        gradeLevels: [],
        schoolClasses: ["Klasse 1b"],
      }),
    ).toBe("Nur Kinder der Klasse 1b");
    expect(
      describeSelectionFilter({
        offeringIds: ["3"],
        gradeLevels: [],
        schoolClasses: ["Klasse 1a", "Klasse 1b"],
      }),
    ).toBe("Nur Kinder der Klassen 1a und 1b");
    expect(
      describeSelectionFilter({
        offeringIds: ["3"],
        gradeLevels: [2, 1],
        schoolClasses: [],
      }),
    ).toBe("Nur Kinder der Jahrgänge 1 und 2");
    expect(
      describeSelectionFilter({
        offeringIds: ["3"],
        gradeLevels: [],
        schoolClasses: [],
      }),
    ).toBe("Alle angemeldeten Kinder des Angebots");
  });

  it("builds the preview query and the connection body with numeric ids", () => {
    const selection = {
      offeringIds: ["3", "4"],
      gradeLevels: [],
      schoolClasses: ["1b"],
    };
    expect(previewQuery(selection, "2026-10-28").toString()).toBe(
      "offering_ids=3%2C4&school_classes=1b&from=2026-10-28",
    );
    expect(previewQuery(selection).toString()).toBe(
      "offering_ids=3%2C4&school_classes=1b",
    );
    expect(connectionBody(selection, "2026-10-28")).toEqual({
      offering_ids: [3, 4],
      grade_levels: [],
      school_classes: ["1b"],
      effective_date: "2026-10-28",
    });
  });
});

describe("mapOfferingPreview and laterSegmentSentence", () => {
  const preview = mapOfferingPreview({
    from: "2026-10-28",
    weekdays: [
      {
        weekday: 1,
        added: [{ student_id: 5, name: "Ali Kaya" }],
        removed: null,
        kept_count: 4,
      },
    ],
    segments: [
      { template_id: 7, valid_from: "2026-09-01", valid_until: "2026-10-30" },
      { template_id: 9, valid_from: "2026-10-30", valid_until: null },
    ],
  });

  it("maps weekdays and segments", () => {
    expect(preview.weekdays).toEqual([
      {
        weekday: 1,
        added: [{ studentId: "5", name: "Ali Kaya" }],
        removed: [],
        keptCount: 4,
      },
    ]);
    expect(preview.segments).toEqual([
      { templateId: "7", validFrom: "2026-09-01", validUntil: "2026-10-30" },
      { templateId: "9", validFrom: "2026-10-30", validUntil: undefined },
    ]);
  });

  it("names the later periods the switch also covers", () => {
    expect(laterSegmentSentence(preview)).toBe(
      "Gilt für alle Termine ab 28.10.2026, auch für den Abschnitt ab 30.10.2026.",
    );
    expect(
      laterSegmentSentence({ ...preview, segments: [preview.segments[1]!] }),
    ).toBeNull();
  });
});

describe("untouchedSegmentSentence", () => {
  const preview = mapOfferingPreview({
    from: "2026-10-28",
    weekdays: [],
    segments: [
      { template_id: 7, valid_from: "2026-09-01", valid_until: "2026-10-30" },
      { template_id: 9, valid_from: "2026-10-30", valid_until: null },
    ],
  });

  it("names the days of another period this edit leaves alone", () => {
    expect(untouchedSegmentSentence(preview, "9")).toBe(
      "Die Termine vom 28.10.2026 bis 29.10.2026 gehören zu einem früheren Abschnitt und behalten ihre Kinderliste. „Mit Angebot verbinden“ an der Karte des Regeltermins stellt alle Abschnitte auf einmal um.",
    );
  });

  it("stays silent when the edit reaches every period", () => {
    expect(
      untouchedSegmentSentence(
        { ...preview, segments: [preview.segments[1]!] },
        "9",
      ),
    ).toBeNull();
  });
});
