import { describe, expect, it } from "vitest";

import {
  DEFAULT_LAYOUTS,
  HOME_BLOCKS,
  homeProfileFor,
  resolveHomeLayout,
  sanitizeHomeBlockPlacements,
  sanitizeHomeBlockPolicies,
  sanitizeHomeLayoutOverrides,
  type HomeBlockAccess,
  type HomeBlockContext,
  type HomeBlockKey,
} from "./home-blocks";

/** Jemand, der alles darf — der Betriebsmodus bleibt so die einzige Variable. */
const leadAccess: HomeBlockAccess = {
  isAdminScope: true,
  has: () => true,
  canOpenRequestsPage: true,
};

const careAccess: HomeBlockAccess = {
  isAdminScope: false,
  has: () => true,
  canOpenRequestsPage: true,
};

const fullContext: HomeBlockContext = {
  detailed: true,
  openCareGroupMode: false,
  nfcEnabled: true,
  birthdaysEnabled: true,
  timetableEnabled: true,
  access: leadAccess,
};

const keysOf = (placements: readonly { key: HomeBlockKey }[]) =>
  placements.map((placement) => placement.key);

describe("resolveHomeLayout — Standardansicht", () => {
  it("zeigt ohne gespeicherte Anordnung die Ansicht der Leitung", () => {
    const { placements, customized } = resolveHomeLayout(
      fullContext,
      [],
      {},
      {},
    );

    expect(customized).toBe(false);
    expect(keysOf(placements)).toEqual(keysOf(DEFAULT_LAYOUTS.lead));
  });

  it("zeigt Betreuungskräften ihre eigene Standardansicht", () => {
    const { placements } = resolveHomeLayout(
      { ...fullContext, access: careAccess },
      [],
      {},
      {},
    );

    expect(homeProfileFor(careAccess)).toBe("care");
    expect(keysOf(placements)).toEqual(keysOf(DEFAULT_LAYOUTS.care));
    // Die Startseite ist ein Einstieg, keine Liste von allem: die
    // Standardansicht bleibt kurz, der Rest steht im Hinzufügen-Menü.
    expect(placements.length).toBeLessThanOrEqual(6);
  });

  it("lässt jeden Baustein der Standardansicht auch wirklich existieren", () => {
    const known = new Set(HOME_BLOCKS.map((block) => block.key));
    for (const layout of Object.values(DEFAULT_LAYOUTS)) {
      for (const placement of layout) {
        expect(known.has(placement.key)).toBe(true);
      }
    }
  });

  it("lässt Raum- und Aktivitätsbausteine im Modus 'binary' weg", () => {
    const { available } = resolveHomeLayout(
      { ...fullContext, detailed: false },
      [],
      {},
      {},
    );

    const keys = available.map((block) => block.key);
    expect(keys).not.toContain("tile.students_in_rooms");
    expect(keys).not.toContain("section.recent_activity");
    expect(keys).not.toContain("section.day_flow");
    expect(keys).toContain("tile.students_present");
  });
});

describe("resolveHomeLayout — eigene Anordnung", () => {
  it("übernimmt Reihenfolge und Breite aus dem Gespeicherten", () => {
    const { placements, customized } = resolveHomeLayout(
      fullContext,
      [
        { key: "section.staff_notices", span: 4 },
        { key: "tile.students_sick", span: 1 },
      ],
      {},
      {},
    );

    expect(placements[0]).toEqual({ key: "section.staff_notices", span: 4 });
    expect(placements[1]).toEqual({ key: "tile.students_sick", span: 1 });
    expect(customized).toBe(true);
  });

  it("hängt Bausteine der Standardansicht hinten an, die noch nie angeordnet wurden", () => {
    // So erreicht ein später ergänzter Baustein auch ein Konto, das seine
    // Startseite längst eingerichtet hat.
    const { placements } = resolveHomeLayout(
      fullContext,
      [{ key: "section.staff_notices", span: 2 }],
      {},
      {},
    );

    expect(placements[0]?.key).toBe("section.staff_notices");
    expect(keysOf(placements)).toContain("section.open_requests");
  });

  it("lässt entfernte Bausteine entfernt", () => {
    const { placements, addable } = resolveHomeLayout(
      fullContext,
      [{ key: "section.staff_notices", span: 2 }],
      { "section.open_requests": false },
      {},
    );

    expect(keysOf(placements)).not.toContain("section.open_requests");
    expect(addable.map((block) => block.key)).toContain("section.open_requests");
  });

  it("korrigiert eine Breite, die es für den Baustein nicht gibt", () => {
    const { placements } = resolveHomeLayout(
      fullContext,
      [{ key: "tile.students_sick", span: 4 }],
      {},
      {},
    );

    expect(placements[0]).toEqual({ key: "tile.students_sick", span: 1 });
  });

  it("stellt einen Baustein nicht zweimal auf die Fläche", () => {
    const { placements } = resolveHomeLayout(
      fullContext,
      [
        { key: "section.staff_notices", span: 2 },
        { key: "section.staff_notices", span: 4 },
      ],
      {},
      {},
    );

    expect(
      keysOf(placements).filter((key) => key === "section.staff_notices"),
    ).toHaveLength(1);
  });
});

describe("resolveHomeLayout — Berechtigung", () => {
  const accessWith = (
    permissions: readonly string[],
    canOpenRequestsPage = false,
  ): HomeBlockAccess => ({
    isAdminScope: false,
    has: (permission: string) => permissions.includes(permission),
    canOpenRequestsPage,
  });

  const resolveFor = (
    permissions: readonly string[],
    canOpenRequestsPage = false,
  ) =>
    resolveHomeLayout(
      { ...fullContext, access: accessWith(permissions, canOpenRequestsPage) },
      [],
      {},
      {},
    );

  it("lässt ohne groups:read jede Kennzahl aus den Betriebszahlen weg", () => {
    const { available, placements } = resolveFor(["users:read"]);

    const keys = available.map((block) => block.key);
    expect(keys).not.toContain("tile.students_present");
    expect(keys).not.toContain("section.active_groups");
    expect(keysOf(placements)).not.toContain("tile.students_present");
    // users:read trägt die Tagesinformationen — die bleiben.
    expect(keysOf(placements)).toContain("section.staff_notices");
  });

  it("zeigt Mein Tag nur mit time_tracking:own", () => {
    expect(keysOf(resolveFor([]).placements)).not.toContain("section.my_day");
    expect(
      keysOf(resolveFor(["time_tracking:own"]).placements),
    ).toContain("section.my_day");
  });

  it("zeigt den Ablauf des Tages nur mit schedules:read", () => {
    expect(keysOf(resolveFor([]).placements)).not.toContain("section.day_flow");
    expect(keysOf(resolveFor(["schedules:read"]).placements)).toContain(
      "section.day_flow",
    );
  });

  // In der Betreuungsansicht stehen die offenen Anfragen nicht von Haus aus:
  // sie betreffen nur Gruppenleitungen, die sie sich dazuholen können.
  it("bietet offene Anfragen nur an, wer das Anfragen-Modul öffnen darf", () => {
    expect(
      resolveFor([]).addable.map((block) => block.key),
    ).not.toContain("section.open_requests");
    expect(resolveFor([], true).addable.map((block) => block.key)).toContain(
      "section.open_requests",
    );
  });

  it("stellt der Leitung die offenen Anfragen von Haus aus auf", () => {
    const { placements } = resolveHomeLayout(
      {
        ...fullContext,
        access: { ...accessWith([], true), isAdminScope: true },
      },
      [],
      {},
      {},
    );

    expect(keysOf(placements)).toContain("section.open_requests");
  });

  it("stellt einen Baustein ohne Recht auch dann nicht auf, wenn er angeordnet ist", () => {
    const { placements, addable } = resolveHomeLayout(
      { ...fullContext, access: accessWith([]) },
      [{ key: "tile.students_present", span: 1 }],
      {},
      {},
    );

    expect(keysOf(placements)).not.toContain("tile.students_present");
    expect(addable.map((block) => block.key)).not.toContain(
      "tile.students_present",
    );
  });
});

describe("resolveHomeLayout — Vorgabe der Einrichtung", () => {
  it("stellt einen verpflichtenden Baustein auf, auch wenn er entfernt wurde", () => {
    const { placements, addable } = resolveHomeLayout(
      fullContext,
      [{ key: "section.staff_notices", span: 2 }],
      { "section.birthdays": false },
      { "section.birthdays": "required" },
    );

    expect(keysOf(placements)).toContain("section.birthdays");
    expect(addable.map((block) => block.key)).not.toContain("section.birthdays");
  });

  it("lässt einen abgeschalteten Baustein trotz Anordnung weg", () => {
    const { placements, addable } = resolveHomeLayout(
      fullContext,
      [{ key: "tile.students_sick", span: 1 }],
      {},
      { "tile.students_sick": "disabled" },
    );

    expect(keysOf(placements)).not.toContain("tile.students_sick");
    expect(addable.map((block) => block.key)).not.toContain(
      "tile.students_sick",
    );
  });

  it("behandelt 'optional' wie gar keine Vorgabe", () => {
    const { placements } = resolveHomeLayout(
      fullContext,
      [],
      {},
      { "section.staff_notices": "optional" },
    );

    expect(keysOf(placements)).toContain("section.staff_notices");
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

  it("räumt eine gespeicherte Anordnung auf", () => {
    expect(
      sanitizeHomeBlockPlacements([
        { key: "section.staff_notices", span: 4 },
        { key: "section.staff_notices", span: 2 },
        { key: "tile.does_not_exist", span: 1 },
        { key: "tile.students_sick", span: 3 },
        "kaputt",
        null,
      ]),
    ).toEqual([
      { key: "section.staff_notices", span: 4 },
      // Eine Breite, die es für eine Kennzahl nicht gibt, wird korrigiert.
      { key: "tile.students_sick", span: 1 },
    ]);
  });

  it("verträgt null und Nicht-Objekte", () => {
    expect(sanitizeHomeLayoutOverrides(null)).toEqual({});
    expect(sanitizeHomeLayoutOverrides("nope")).toEqual({});
    expect(sanitizeHomeBlockPolicies(undefined)).toEqual({});
    expect(sanitizeHomeBlockPlacements("nope")).toEqual([]);
  });
});
