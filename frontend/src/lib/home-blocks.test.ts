import { describe, expect, it } from "vitest";

import {
  HOME_BLOCKS,
  resolveHomeBlocks,
  sanitizeHomeBlockPolicies,
  sanitizeHomeLayoutOverrides,
  type HomeBlockContext,
} from "./home-blocks";

const fullContext: HomeBlockContext = {
  detailed: true,
  openCareGroupMode: false,
  nfcEnabled: true,
  birthdaysEnabled: true,
};

describe("resolveHomeBlocks — Betriebsmodus", () => {
  it("zeigt ohne gespeicherte Auswahl genau die Standardansicht", () => {
    const { visible, customized } = resolveHomeBlocks(fullContext, {}, {});

    expect(customized).toBe(false);
    for (const block of HOME_BLOCKS) {
      expect(visible.has(block.key)).toBe(block.defaultVisible);
    }
  });

  it("lässt Raum- und Aktivitätsbausteine im Modus 'binary' weg", () => {
    const { available, visible } = resolveHomeBlocks(
      { ...fullContext, detailed: false },
      {},
      {},
    );

    const keys = available.map((block) => block.key);
    expect(keys).not.toContain("tile.students_in_rooms");
    expect(keys).not.toContain("tile.capacity_utilization");
    expect(keys).not.toContain("section.recent_activity");
    expect(visible.has("tile.students_present")).toBe(true);
  });

  it("lässt 'Aktive Gruppen' in der offenen Betreuung weg", () => {
    const { available } = resolveHomeBlocks(
      { ...fullContext, openCareGroupMode: true },
      {},
      {},
    );

    expect(available.map((block) => block.key)).not.toContain(
      "section.active_groups",
    );
  });
});

describe("resolveHomeBlocks — eigene Auswahl", () => {
  it("blendet aus, was die Person abgewählt hat", () => {
    const { visible, customized } = resolveHomeBlocks(
      fullContext,
      { "section.birthdays": false },
      {},
    );

    expect(visible.has("section.birthdays")).toBe(false);
    expect(customized).toBe(true);
  });

  it("zählt eine Abweichung, die dem Standard entspricht, nicht als Anpassung", () => {
    // Sonst bliebe "Zurücksetzen" nach einem Hin und Her im Dialog aktiv,
    // obwohl es nichts zurückzusetzen gibt.
    const { customized } = resolveHomeBlocks(
      fullContext,
      { "section.birthdays": true },
      {},
    );

    expect(customized).toBe(false);
  });

  it("ignoriert Einträge zu Bausteinen, die es hier nicht gibt", () => {
    const { visible } = resolveHomeBlocks(
      { ...fullContext, detailed: false },
      { "tile.capacity_utilization": true },
      {},
    );

    expect(visible.has("tile.capacity_utilization")).toBe(false);
  });

  it("lässt eine nicht verfügbare Abweichung zurücksetzen", () => {
    const { customized } = resolveHomeBlocks(
      { ...fullContext, detailed: false },
      { "tile.capacity_utilization": false },
      {},
    );

    expect(customized).toBe(true);
  });
});

describe("resolveHomeBlocks — Vorgabe der Einrichtung", () => {
  it("zeigt einen verpflichtenden Baustein trotz gegenteiliger Auswahl", () => {
    const { visible, adjustable } = resolveHomeBlocks(
      fullContext,
      { "section.birthdays": false },
      { "section.birthdays": "required" },
    );

    expect(visible.has("section.birthdays")).toBe(true);
    expect(adjustable.map((block) => block.key)).not.toContain(
      "section.birthdays",
    );
  });

  it("blendet einen deaktivierten Baustein trotz gegenteiliger Auswahl aus", () => {
    const { visible, adjustable } = resolveHomeBlocks(
      fullContext,
      { "tile.students_sick": true },
      { "tile.students_sick": "disabled" },
    );

    expect(visible.has("tile.students_sick")).toBe(false);
    expect(adjustable.map((block) => block.key)).not.toContain(
      "tile.students_sick",
    );
  });

  it("lässt eine von der Schule überstimmte Abweichung zurücksetzen", () => {
    const { customized } = resolveHomeBlocks(
      fullContext,
      { "section.birthdays": false },
      { "section.birthdays": "required" },
    );

    expect(customized).toBe(true);
  });

  it("stellt die alte Auswahl wieder her, wenn die Vorgabe zurückgenommen wird", () => {
    // Die gespeicherte Abweichung wird von der Vorgabe nur überstimmt, nicht
    // gelöscht — sonst müsste jede zurückgenommene Entscheidung der Leitung
    // die Auswahl aller Konten neu erfinden.
    const overrides = { "section.birthdays": false };

    expect(
      resolveHomeBlocks(fullContext, overrides, {
        "section.birthdays": "required",
      }).visible.has("section.birthdays"),
    ).toBe(true);

    expect(
      resolveHomeBlocks(fullContext, overrides, {}).visible.has(
        "section.birthdays",
      ),
    ).toBe(false);
  });

  it("behandelt 'optional' wie gar keine Vorgabe", () => {
    const { adjustable } = resolveHomeBlocks(
      fullContext,
      {},
      {
        "section.birthdays": "optional",
      },
    );

    expect(adjustable.map((block) => block.key)).toContain("section.birthdays");
  });
});

describe("sanitize", () => {
  it("verwirft unbekannte Schlüssel und falsche Werttypen", () => {
    expect(
      sanitizeHomeLayoutOverrides({
        "section.birthdays": false,
        "tile.does_not_exist": true,
        "tile.students_sick": "ja",
      }),
    ).toEqual({ "section.birthdays": false });
  });

  it("verwirft unbekannte Vorgabe-Werte", () => {
    expect(
      sanitizeHomeBlockPolicies({
        "section.birthdays": "disabled",
        "tile.students_sick": "mandatory",
      }),
    ).toEqual({ "section.birthdays": "disabled" });
  });

  it("verträgt null und Nicht-Objekte", () => {
    expect(sanitizeHomeLayoutOverrides(null)).toEqual({});
    expect(sanitizeHomeLayoutOverrides("nope")).toEqual({});
    expect(sanitizeHomeBlockPolicies(undefined)).toEqual({});
  });
});
