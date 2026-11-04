import { describe, expect, it } from "vitest";

import {
  connectionBody,
  describeSelectionFilter,
  laterSegmentSentence,
  mapOfferingPreview,
  mapRosterDeviations,
  previewQuery,
  rosterDeviationGroups,
  rosterDeviationMatch,
  untouchedSegmentSentence,
  type RosterDeviation,
} from "./roster-deviation";

const deviation: RosterDeviation = {
  templateId: "7",
  offerings: [{ id: "3", name: "Randstunde" }],
  gradeLevels: [],
  schoolClasses: ["1a"],
  unbookedCount: 8,
  missingCount: 0,
  candidates: [{ id: "3", name: "Randstunde" }],
  combinations: [],
  ambiguous: false,
  unbooked: [],
  missing: [],
};

describe("rosterDeviationMatch", () => {
  it("names the matching offering", () => {
    expect(rosterDeviationMatch(deviation)).toBe("Passt zu „Randstunde“.");
  });

  it("names every offering of a combined suggestion (#5139)", () => {
    expect(
      rosterDeviationMatch({
        ...deviation,
        offerings: [
          { id: "3", name: "Randstunde" },
          { id: "4", name: "Ganztag" },
        ],
      }),
    ).toBe("Passt zu „Randstunde“ und „Ganztag“.");
  });

  it("names every fitting offering when several fit equally (#5138)", () => {
    expect(
      rosterDeviationMatch({
        ...deviation,
        candidates: [
          { id: "4", name: "Ganztag" },
          { id: "3", name: "Randstunde" },
          { id: "5", name: "Spätbetreuung" },
        ],
        ambiguous: true,
      }),
    ).toBe("Passt zu „Ganztag“, „Randstunde“ oder „Spätbetreuung“.");
  });

  it("names equal combinations whole (#5139)", () => {
    expect(
      rosterDeviationMatch({
        ...deviation,
        combinations: [
          [
            { id: "1", name: "A" },
            { id: "2", name: "B" },
          ],
          [
            { id: "3", name: "C" },
            { id: "4", name: "D" },
          ],
        ],
        ambiguous: true,
      }),
    ).toBe("Passt zu „A“ und „B“ oder zu „C“ und „D“.");
  });
});

describe("rosterDeviationGroups", () => {
  const combined: RosterDeviation = {
    ...deviation,
    offerings: [
      { id: "3", name: "Randstunde" },
      { id: "4", name: "Ganztag bis 14.30 Uhr" },
      { id: "5", name: "Ganztag bis 16 Uhr" },
    ],
  };

  it("names one child and its weekday, booked for none of the offerings", () => {
    expect(
      rosterDeviationGroups({
        ...combined,
        unbookedCount: 1,
        unbooked: [{ studentId: "9", name: "Mia Sommer", weekdays: [5] }],
      }),
    ).toEqual([
      {
        sentence:
          "Mia Sommer ist freitags für keines dieser Angebote angemeldet.",
        children: [],
      },
    ]);
  });

  it("says „dieses Angebot“ when only one offering fits", () => {
    expect(
      rosterDeviationGroups({
        ...deviation,
        unbookedCount: 1,
        unbooked: [{ studentId: "9", name: "Mia Sommer", weekdays: [1, 3] }],
      })[0]?.sentence,
    ).toBe(
      "Mia Sommer ist montags und mittwochs nicht für dieses Angebot angemeldet.",
    );
  });

  it("counts several children and lists them with their weekdays", () => {
    expect(
      rosterDeviationGroups({
        ...combined,
        unbookedCount: 2,
        unbooked: [
          { studentId: "9", name: "Mia Sommer", weekdays: [5] },
          { studentId: "8", name: "Ben Winter", weekdays: [1, 2, 3] },
        ],
      }),
    ).toEqual([
      {
        sentence: "2 Kinder sind für keines dieser Angebote angemeldet.",
        children: [
          { studentId: "9", label: "Mia Sommer: freitags" },
          {
            studentId: "8",
            label: "Ben Winter: montags, dienstags und mittwochs",
          },
        ],
      },
    ]);
  });

  it("keeps the count when the caller may not read the names", () => {
    expect(rosterDeviationGroups({ ...combined, unbookedCount: 1 })).toEqual([
      {
        sentence: "1 Kind ist für keines dieser Angebote angemeldet.",
        children: [],
      },
    ]);
  });

  it("names booked children missing from the list", () => {
    expect(
      rosterDeviationGroups({
        ...deviation,
        unbookedCount: 0,
        missingCount: 1,
        missing: [{ studentId: "7", name: "Lea Berg", weekdays: [2] }],
      }),
    ).toEqual([
      {
        sentence: "Lea Berg ist dienstags angemeldet, fehlt aber in der Liste.",
        children: [],
      },
    ]);
    expect(
      rosterDeviationGroups({
        ...deviation,
        unbookedCount: 0,
        missingCount: 2,
      }),
    ).toEqual([
      {
        sentence: "2 angemeldete Kinder fehlen in der Liste.",
        children: [],
      },
    ]);
  });

  it("names no children when several offerings fit equally (#5138)", () => {
    expect(rosterDeviationGroups({ ...deviation, ambiguous: true })).toEqual(
      [],
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
            offerings: [{ id: 3, name: "Randstunde" }],
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
            offerings: [{ id: 3, name: "Randstunde" }],
            school_classes: ["1a"],
            unbooked_count: 8,
            missing_count: 0,
            candidates: [{ id: 3, name: "Randstunde" }],
            ambiguous: true,
          },
        ],
      })[0]?.ambiguous,
    ).toBe(true);
    expect(
      mapRosterDeviations({
        deviations: [
          {
            template_id: 7,
            offerings: [{ id: 3, name: "Randstunde" }],
            unbooked_count: 0,
            missing_count: 0,
            combinations: [
              [{ id: 3, name: "Randstunde" }],
              [{ id: 4, name: "Ganztag" }],
            ],
            ambiguous: true,
          },
        ],
      })[0]?.combinations,
    ).toEqual([
      [{ id: "3", name: "Randstunde" }],
      [{ id: "4", name: "Ganztag" }],
    ]);
    expect(
      mapRosterDeviations({
        deviations: [
          {
            template_id: 7,
            unbooked_count: 1,
            missing_count: 0,
            unbooked: [{ student_id: 9, name: "Mia Sommer", weekdays: [5] }],
          },
        ],
      })[0]?.unbooked,
    ).toEqual([{ studentId: "9", name: "Mia Sommer", weekdays: [5] }]);
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
    expect(
      describeSelectionFilter({
        offeringIds: ["3", "4"],
        gradeLevels: [],
        schoolClasses: [],
      }),
    ).toBe("Alle angemeldeten Kinder der Angebote");
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
