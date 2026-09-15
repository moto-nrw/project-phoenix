import { describe, expect, it } from "vitest";

import { describeRosterMaintenance } from "./roster-maintenance";
import type { TemplateRosterMaintenance } from "./timetable-types";

function state(
  overrides: Partial<TemplateRosterMaintenance>,
): TemplateRosterMaintenance {
  return {
    mode: "manual",
    offeringNames: [],
    gradeLevels: [],
    schoolClasses: [],
    inactiveOfferingNames: [],
    dynamicTargets: false,
    careOfferingsDisabled: false,
    ...overrides,
  };
}

describe("describeRosterMaintenance", () => {
  it("names the offering and the class filter for an automatic roster", () => {
    const result = describeRosterMaintenance(
      state({
        mode: "automatic",
        offeringNames: ["Betreuung bis 16 Uhr"],
        schoolClasses: ["2a", "2b"],
      }),
    );

    expect(result.label).toBe("Automatisch");
    expect(result.explanation).toBe(
      "Neue Anmeldungen werden automatisch übernommen. Quelle: Betreuungsangebot „Betreuung bis 16 Uhr“. Nur Klassen 2a und 2b.",
    );
  });

  it("sorts grade filters and lists several offerings", () => {
    const result = describeRosterMaintenance(
      state({
        mode: "automatic",
        offeringNames: ["Früh", "Spät", "Musik"],
        gradeLevels: [3, 1],
      }),
    );

    expect(result.explanation).toContain(
      "Quelle: Betreuungsangebote „Früh“, „Spät“ und „Musik“.",
    );
    expect(result.explanation).toContain("Nur Jahrgänge 1 und 3.");
  });

  it("does not promise automation for a class target without an offering", () => {
    // Support case of #3140: Randstunde for a class, no offering link.
    const result = describeRosterMaintenance(
      state({ mode: "manual", dynamicTargets: true }),
    );

    expect(result.label).toBe("Manuell");
    expect(result.explanation).toBe(
      "Neue Kinder müssen Sie selbst ergänzen. Klasse, Jahrgang oder Gruppe gelten nur für neu erzeugte Termine.",
    );
    expect(result.explanation).not.toContain("automatisch übernommen");
  });

  it("explains the mixed state instead of promising full automation", () => {
    const result = describeRosterMaintenance(
      state({
        mode: "partial",
        offeringNames: ["Mittagessen"],
        dynamicTargets: true,
      }),
    );

    expect(result.label).toBe("Teils automatisch");
    expect(result.explanation).toBe(
      "Neue Anmeldungen für das Betreuungsangebot „Mittagessen“ werden automatisch übernommen. Klasse, Jahrgang oder Gruppe gelten nur für neu erzeugte Termine. Weitere Kinder müssen Sie selbst ergänzen.",
    );
  });

  it("names a switched-off source offering", () => {
    const result = describeRosterMaintenance(
      state({ mode: "manual", inactiveOfferingNames: ["Musik"] }),
    );

    expect(result.explanation).toBe(
      "Neue Kinder müssen Sie selbst ergänzen. Das Betreuungsangebot „Musik“ ist ausgeschaltet.",
    );
  });

  it("says when the school switched offerings off", () => {
    const result = describeRosterMaintenance(
      state({ mode: "manual", careOfferingsDisabled: true }),
    );

    expect(result.explanation).toBe(
      "Neue Kinder müssen Sie selbst ergänzen. Betreuungsangebote sind an Ihrer Schule ausgeschaltet.",
    );
  });
});
