import { describe, expect, it } from "vitest";

import { emptyForm, type EventFormState } from "./form-model";
import {
  draftPreviewQuery,
  mapOfferingSuggestion,
  suggestionQuery,
  suggestionSelection,
} from "~/lib/offering-suggestion";

import {
  buildDraftRoster,
  targetAudienceText,
} from "./offering-suggestion-model";

const form = (overrides: Partial<EventFormState> = {}): EventFormState => ({
  ...emptyForm("2026-09-14", "4"),
  ...overrides,
});

describe("buildDraftRoster", () => {
  it("plans the Klasse members and the hand-picked children on every weekday", () => {
    const roster = buildDraftRoster(
      form({
        targetGroupType: "klasse",
        targetSchoolClasses: ["1b"],
        weekdays: [3, 1],
        studentIds: ["20", "11"],
      }),
      ["12", "11", "13"],
    );
    expect(roster).toEqual({
      1: ["11", "12", "13", "20"],
      3: ["11", "12", "13", "20"],
    });
  });

  it("takes the per-weekday child lists when the series plans each day", () => {
    const roster = buildDraftRoster(
      form({
        weekdays: [1, 2],
        perWeekdayRoster: true,
        weekdayRosters: {
          1: { staffIds: [], studentIds: ["5"], primaryStaffId: "" },
          2: { staffIds: [], studentIds: ["6", "7"], primaryStaffId: "" },
        },
      }),
      [],
    );
    expect(roster).toEqual({ 1: ["5"], 2: ["6", "7"] });
  });

  it("ignores the cohort while the target is no Klasse, Jahrgang or Gruppe", () => {
    expect(
      buildDraftRoster(form({ weekdays: [1], studentIds: ["5"] }), ["9"]),
    ).toEqual({ 1: ["5"] });
  });
});

describe("suggestion queries", () => {
  it("sends the roster as one parameter per weekday", () => {
    const query = suggestionQuery({
      periodId: "4",
      type: "care",
      listKind: "edge_hours",
      endTime: "12:45",
      roster: { 3: ["11", "12"], 1: ["11"] },
      schoolClasses: ["1b"],
      gradeLevels: [],
    });
    expect(query.get("period_id")).toBe("4");
    expect(query.get("end_time")).toBe("12:45");
    expect(query.get("type")).toBe("care");
    expect(query.get("list_kind")).toBe("edge_hours");
    expect(query.getAll("roster")).toEqual(["1:11", "3:11,12"]);
    expect(query.get("school_classes")).toBe("1b");
    expect(query.has("grade_levels")).toBe(false);
  });

  it("builds the draft preview query with the source selection", () => {
    const query = draftPreviewQuery(
      "4",
      { offeringIds: ["7"], gradeLevels: [1, 2], schoolClasses: [] },
      { 1: ["11"] },
      "2026-09-14",
    );
    expect(query.get("period_id")).toBe("4");
    expect(query.get("offering_ids")).toBe("7");
    expect(query.get("grade_levels")).toBe("1,2");
    expect(query.getAll("roster")).toEqual(["1:11"]);
    expect(query.get("from")).toBe("2026-09-14");
  });
});

describe("mapOfferingSuggestion", () => {
  it("turns ids into strings and keeps null as no suggestion", () => {
    expect(mapOfferingSuggestion({ suggestion: null })).toBeNull();
    const suggestion = mapOfferingSuggestion({
      suggestion: {
        offerings: [{ id: 7, name: "Randstunde" }],
        school_classes: ["1b"],
        grade_levels: null,
        unbooked_count: 2,
        missing_count: 0,
        candidates: [{ id: 7, name: "Randstunde" }],
      },
    });
    expect(suggestion).toEqual({
      offerings: [{ id: "7", name: "Randstunde" }],
      schoolClasses: ["1b"],
      gradeLevels: [],
      combinations: [],
      unbookedCount: 2,
      missingCount: 0,
      candidates: [{ id: "7", name: "Randstunde" }],
      ambiguous: false,
    });
    expect(
      mapOfferingSuggestion({
        suggestion: {
          offerings: [{ id: 7, name: "Randstunde" }],
          unbooked_count: 2,
          missing_count: 0,
          ambiguous: true,
        },
      })?.ambiguous,
    ).toBe(true);
    expect(suggestionSelection(suggestion!)).toEqual({
      offeringIds: ["7"],
      gradeLevels: [],
      schoolClasses: ["1b"],
    });
    const combined = mapOfferingSuggestion({
      suggestion: {
        offerings: [
          { id: 7, name: "Randstunde" },
          { id: 4, name: "Ganztag" },
        ],
        school_classes: ["1a"],
        unbooked_count: 0,
        missing_count: 0,
      },
    });
    expect(suggestionSelection(combined!).offeringIds).toEqual(["7", "4"]);
  });
});

describe("mapOfferingSuggestion combinations", () => {
  it("maps complete alternatives without dropping complementary offerings", () => {
    const randstunde = { id: 7, name: "Randstunde" };
    const ganztag = { id: 4, name: "Ganztag" };
    const spaet = { id: 8, name: "Ganztag spät" };
    const suggestion = mapOfferingSuggestion({
      suggestion: {
        offerings: [ganztag, randstunde],
        combinations: [
          [ganztag, randstunde],
          [spaet, randstunde],
        ],
        unbooked_count: 0,
        missing_count: 0,
        ambiguous: true,
      },
    });
    expect(suggestion?.combinations).toEqual([
      [
        { id: "4", name: "Ganztag" },
        { id: "7", name: "Randstunde" },
      ],
      [
        { id: "8", name: "Ganztag spät" },
        { id: "7", name: "Randstunde" },
      ],
    ]);
  });
});

describe("targetAudienceText", () => {
  const groups = [
    { id: "g1", name: "Sternengruppe" },
    { id: "g2", name: "Mondgruppe" },
  ];

  it("says that a Klasse brings every child, bookings or not", () => {
    expect(
      targetAudienceText(
        form({ targetGroupType: "klasse", targetSchoolClasses: ["1b"] }),
        groups,
      ),
    ).toBe(
      "Es kommen alle Kinder der Klasse 1b, unabhängig von Anmeldungen. Spätere An- und Abmeldungen werden nicht übernommen.",
    );
    expect(
      targetAudienceText(
        form({
          targetGroupType: "klasse",
          targetSchoolClasses: ["Klasse 1a", "Klasse 1b"],
        }),
        groups,
      ),
    ).toBe(
      "Es kommen alle Kinder der gewählten Klassen, unabhängig von Anmeldungen. Spätere An- und Abmeldungen werden nicht übernommen.",
    );
  });

  it("says the same for a Jahrgang and a Gruppe", () => {
    expect(
      targetAudienceText(
        form({ targetGroupType: "jahrgang", targetGradeLevels: ["2"] }),
        groups,
      ),
    ).toBe(
      "Es kommen alle Kinder der Jahrgangsstufe 2, unabhängig von Anmeldungen. Spätere An- und Abmeldungen werden nicht übernommen.",
    );
    expect(
      targetAudienceText(
        form({ targetGroupType: "jahrgang", targetGradeLevels: ["1", "2"] }),
        groups,
      ),
    ).toBe(
      "Es kommen alle Kinder der gewählten Jahrgangsstufen, unabhängig von Anmeldungen. Spätere An- und Abmeldungen werden nicht übernommen.",
    );
    expect(
      targetAudienceText(
        form({ targetGroupType: "gruppe", educationGroupIds: ["g1"] }),
        groups,
      ),
    ).toBe(
      "Es kommen alle Kinder der Gruppe Sternengruppe, unabhängig von Anmeldungen. Spätere An- und Abmeldungen werden nicht übernommen.",
    );
    expect(
      targetAudienceText(
        form({ targetGroupType: "gruppe", educationGroupIds: ["g1", "g2"] }),
        groups,
      ),
    ).toBe(
      "Es kommen alle Kinder der gewählten Gruppen, unabhängig von Anmeldungen. Spätere An- und Abmeldungen werden nicht übernommen.",
    );
  });

  it("has no text without such a target", () => {
    expect(targetAudienceText(form(), groups)).toBeNull();
    expect(
      targetAudienceText(form({ targetGroupType: "angebot" }), groups),
    ).toBeNull();
  });
});
