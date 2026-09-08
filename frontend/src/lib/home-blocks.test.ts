import { describe, expect, it } from "vitest";

import {
  DEFAULT_LAYOUTS,
  HOME_BLOCKS,
  appendPlacement,
  computeBoardCells,
  homeProfileFor,
  movePlacement,
  placementWithSpan,
  resolveHomeLayout,
  rowsOf,
  sanitizeHomeBlockPlacements,
  sanitizeHomeBlockPolicies,
  sanitizeHomeLayoutOverrides,
  type HomeBlockAccess,
  type HomeBlockContext,
  type HomeBlockKey,
  type HomeBlockPlacement,
} from "./home-blocks";

/** Jemand, der alles darf — der Betriebsmodus bleibt so die einzige Variable. */
const leadAccess: HomeBlockAccess = {
  isAdminScope: true,
  has: () => true,
  canOpenRequestsPage: true,
  caresForGroups: false,
  hasOwnGroups: false,
};

const careAccess: HomeBlockAccess = {
  isAdminScope: false,
  has: () => true,
  canOpenRequestsPage: true,
  caresForGroups: true,
  hasOwnGroups: true,
};

/** Führt die Einrichtung UND betreut selbst: bekommt die Vereinigung. */
const leadCareAccess: HomeBlockAccess = {
  isAdminScope: true,
  has: () => true,
  canOpenRequestsPage: true,
  caresForGroups: true,
  hasOwnGroups: true,
};

const fullContext: HomeBlockContext = {
  detailed: true,
  openCareGroupMode: false,
  nfcEnabled: true,
  birthdaysEnabled: true,
  timetableEnabled: true,
  remindersEnabled: true,
  messagingEnabled: true,
  staffMessagingEnabled: true,
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
    // Auf die Person zugeschnitten: der eigene Tag und die eigene Gruppe,
    // nicht die schulweite Betreuung.
    expect(keysOf(placements)).toContain("section.my_day");
    expect(keysOf(placements)).toContain("section.my_group");
    expect(keysOf(placements)).not.toContain("section.active_groups");
  });

  // Zwei Bausteine, die dieselben Zeilen zeigen, gehören nicht zusammen in
  // einen Standard: „Mein Tag" ist der eigene Ausschnitt des Ablaufs.
  it("stellt Mein Tag und Ablauf des Tages nicht zusammen auf, wer nur eines ist", () => {
    for (const profile of ["care", "lead"] as const) {
      const keys = keysOf(DEFAULT_LAYOUTS[profile]);
      expect(
        keys.includes("section.my_day") && keys.includes("section.day_flow"),
      ).toBe(false);
    }
  });

  // Das Issue verlangt für Mischrollen die Vereinigung: die Gruppenleitung
  // mit Adminrecht verliert ihren eigenen Tag nicht an die Leitungsansicht.
  it("gibt der Leitung, die selbst betreut, beides", () => {
    const { placements } = resolveHomeLayout(
      { ...fullContext, access: leadCareAccess },
      [],
      {},
      {},
    );

    expect(homeProfileFor(leadCareAccess)).toBe("lead_care");
    const keys = keysOf(placements);
    expect(keys[0]).toBe("section.my_day");
    expect(keys).toContain("section.my_group");
    expect(keys).toContain("section.open_requests");
    expect(keys).toContain("section.day_flow");
    expect(keys).toContain("tile.students_present");
  });

  // Ohne eigene Gruppe wäre „Meine Gruppe" eine Karte, die nur sagt, dass sie
  // leer ist.
  it("lässt Meine Gruppe weg, wer heute keine Gruppe hat", () => {
    const { placements, available } = resolveHomeLayout(
      { ...fullContext, access: { ...careAccess, hasOwnGroups: false } },
      [],
      {},
      {},
    );

    expect(keysOf(placements)).not.toContain("section.my_group");
    expect(available.map((block) => block.key)).not.toContain(
      "section.my_group",
    );
  });

  it("bietet die Nachrichten nur an, wo die Schule eine der beiden Arten eingeschaltet hat", () => {
    const off = resolveHomeLayout(
      { ...fullContext, messagingEnabled: false, staffMessagingEnabled: false },
      [],
      {},
      {},
    );
    expect(off.available.map((block) => block.key)).not.toContain(
      "section.messages",
    );

    const teamOnly = resolveHomeLayout(
      { ...fullContext, messagingEnabled: false },
      [],
      {},
      {},
    );
    expect(teamOnly.available.map((block) => block.key)).toContain(
      "section.messages",
    );
  });

  it("lässt jeden Baustein der Standardansicht auch wirklich existieren", () => {
    const known = new Set(HOME_BLOCKS.map((block) => block.key));
    for (const layout of Object.values(DEFAULT_LAYOUTS)) {
      for (const placement of layout) {
        expect(known.has(placement.key)).toBe(true);
      }
    }
  });

  // Eine Karte, die dauerhaft „ist ausgeschaltet" sagt, belegt nur einen Platz.
  it("lässt die Erinnerungen weg, wenn die Schule keine eingeschaltet hat", () => {
    const { placements, available, addable } = resolveHomeLayout(
      { ...fullContext, access: careAccess, remindersEnabled: false },
      [],
      {},
      {},
    );

    expect(keysOf(placements)).not.toContain("section.reminders");
    expect(available.map((block) => block.key)).not.toContain(
      "section.reminders",
    );
    expect(addable.map((block) => block.key)).not.toContain(
      "section.reminders",
    );
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
        { key: "section.staff_notices", span: 4, row: 0 },
        { key: "tile.students_sick", span: 1, row: 0 },
      ],
      {},
      {},
    );

    expect(placements[0]).toEqual({
      key: "section.staff_notices",
      span: 4,
      row: 0,
    });
    // Neben einer Karte über die volle Breite ist kein Platz: die Kennzahl
    // rückt in eine eigene Reihe darunter.
    expect(placements[1]).toEqual({
      key: "tile.students_sick",
      span: 1,
      row: 1,
    });
    expect(customized).toBe(true);
  });

  it("hängt Bausteine der Standardansicht hinten an, die noch nie angeordnet wurden", () => {
    // So erreicht ein später ergänzter Baustein auch ein Konto, das seine
    // Startseite längst eingerichtet hat.
    const { placements } = resolveHomeLayout(
      fullContext,
      [{ key: "section.staff_notices", span: 2, row: 0 }],
      {},
      {},
    );

    expect(placements[0]?.key).toBe("section.staff_notices");
    expect(keysOf(placements)).toContain("section.open_requests");
  });

  it("lässt entfernte Bausteine entfernt", () => {
    const { placements, addable } = resolveHomeLayout(
      fullContext,
      [{ key: "section.staff_notices", span: 2, row: 0 }],
      { "section.open_requests": false },
      {},
    );

    expect(keysOf(placements)).not.toContain("section.open_requests");
    expect(addable.map((block) => block.key)).toContain(
      "section.open_requests",
    );
  });

  it("korrigiert eine Breite, die es für den Baustein nicht gibt", () => {
    const { placements } = resolveHomeLayout(
      fullContext,
      [{ key: "tile.students_sick", span: 4, row: 0 }],
      {},
      {},
    );

    expect(placements[0]).toEqual({
      key: "tile.students_sick",
      span: 1,
      row: 0,
    });
  });

  it("stellt einen Baustein nicht zweimal auf die Fläche", () => {
    const { placements } = resolveHomeLayout(
      fullContext,
      [
        { key: "section.staff_notices", span: 2, row: 0 },
        { key: "section.staff_notices", span: 4, row: 0 },
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
    caresForGroups: true,
    hasOwnGroups: true,
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
    expect(keysOf(resolveFor(["time_tracking:own"]).placements)).toContain(
      "section.my_day",
    );
  });

  // In der Betreuungsansicht steht der Ablauf des Tages nicht: er zeigt
  // dieselben Blöcke wie „Mein Tag". Anbieten lässt er sich trotzdem, sobald
  // das Recht reicht.
  it("bietet den Ablauf des Tages nur mit schedules:read an", () => {
    expect(resolveFor([]).addable.map((block) => block.key)).not.toContain(
      "section.day_flow",
    );
    expect(
      resolveFor(["schedules:read"]).addable.map((block) => block.key),
    ).toContain("section.day_flow");
  });

  // In der Betreuungsansicht stehen die offenen Anfragen nicht von Haus aus:
  // sie betreffen nur Gruppenleitungen, die sie sich dazuholen können.
  it("bietet offene Anfragen nur an, wer das Anfragen-Modul öffnen darf", () => {
    expect(resolveFor([]).addable.map((block) => block.key)).not.toContain(
      "section.open_requests",
    );
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
      [{ key: "tile.students_present", span: 1, row: 0 }],
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
      [{ key: "section.staff_notices", span: 2, row: 0 }],
      { "section.birthdays": false },
      { "section.birthdays": "required" },
    );

    expect(keysOf(placements)).toContain("section.birthdays");
    expect(addable.map((block) => block.key)).not.toContain(
      "section.birthdays",
    );
  });

  it("lässt einen abgeschalteten Baustein trotz Anordnung weg", () => {
    const { placements, addable } = resolveHomeLayout(
      fullContext,
      [{ key: "tile.students_sick", span: 1, row: 0 }],
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
        { key: "section.staff_notices", span: 4, row: 0 },
        { key: "section.staff_notices", span: 2, row: 0 },
        { key: "tile.does_not_exist", span: 1, row: 0 },
        { key: "tile.students_sick", span: 3 },
        "kaputt",
        null,
      ]),
    ).toEqual([
      { key: "section.staff_notices", span: 4, row: 0 },
      // Eine Breite, die es für eine Kennzahl nicht gibt, wird korrigiert;
      // neben der vollen Breite ist kein Platz, also eine eigene Reihe.
      { key: "tile.students_sick", span: 1, row: 1 },
    ]);
  });

  // Anordnungen von vor den Reihen tragen keine oder überall Reihe 0: sie
  // werden nach Breite gepackt, wie das Raster sie damals zeichnete.
  it("packt eine Anordnung ohne Reihen nach Breite", () => {
    expect(
      sanitizeHomeBlockPlacements([
        { key: "tile.students_present", span: 1 },
        { key: "tile.students_sick", span: 1 },
        { key: "section.open_requests", span: 2 },
        { key: "section.staff_today", span: 2 },
        { key: "section.birthdays", span: 4 },
      ]).map((entry) => entry.row),
    ).toEqual([0, 0, 0, 1, 2]);
  });

  it("übernimmt gespeicherte Reihen und schließt Lücken", () => {
    expect(
      sanitizeHomeBlockPlacements([
        { key: "tile.students_present", span: 1, row: 2 },
        { key: "section.open_requests", span: 2, row: 5 },
        { key: "section.staff_today", span: 2, row: 5 },
      ]).map((entry) => entry.row),
    ).toEqual([0, 1, 1]);
  });

  it("verträgt null und Nicht-Objekte", () => {
    expect(sanitizeHomeLayoutOverrides(null)).toEqual({});
    expect(sanitizeHomeLayoutOverrides("nope")).toEqual({});
    expect(sanitizeHomeBlockPolicies(undefined)).toEqual({});
    expect(sanitizeHomeBlockPlacements("nope")).toEqual([]);
  });
});

describe("Reihen (#2180)", () => {
  const rows: HomeBlockPlacement[] = [
    { key: "tile.students_present", span: 1, row: 0 },
    { key: "tile.students_sick", span: 1, row: 0 },
    { key: "section.open_requests", span: 2, row: 1 },
    { key: "section.staff_today", span: 2, row: 1 },
    { key: "section.birthdays", span: 4, row: 2 },
  ];
  const rowsAsKeys = (placements: readonly HomeBlockPlacement[]) =>
    rowsOf(placements).map((entries) => entries.map((entry) => entry.key));

  // Genau der Fall aus der Rückmeldung: eine Kennzahl unter die offenen
  // Anfragen legen — sie bekommt eine eigene Reihe, statt daneben zu rutschen.
  it("gibt einem Baustein zwischen zwei Reihen eine eigene Reihe", () => {
    const next = movePlacement(rows, "tile.students_present", {
      kind: "newRow",
      before: 2,
    });

    expect(rowsAsKeys(next)).toEqual([
      ["tile.students_sick"],
      ["section.open_requests", "section.staff_today"],
      ["tile.students_present"],
      ["section.birthdays"],
    ]);
  });

  it("setzt einen Baustein an eine Stelle in einer Reihe", () => {
    const next = movePlacement(rows, "section.staff_today", {
      kind: "into",
      row: 0,
      index: 1,
    });

    expect(rowsAsKeys(next)).toEqual([
      ["tile.students_present", "section.staff_today", "tile.students_sick"],
      ["section.open_requests"],
      ["section.birthdays"],
    ]);
  });

  it("lässt eine Reihe verschwinden, die leer wird", () => {
    const next = movePlacement(rows, "section.birthdays", {
      kind: "into",
      row: 1,
      index: 0,
    });

    // Die Geburtstage brauchen die volle Breite: in Reihe 1 passen sie nicht,
    // also bleibt alles, wie es ist — und ihre eigene Reihe bleibt bestehen.
    expect(next).toBe(rows);

    const emptied = movePlacement(
      [
        { key: "tile.students_present", span: 1, row: 0 },
        { key: "tile.students_sick", span: 1, row: 1 },
        { key: "section.birthdays", span: 4, row: 2 },
      ],
      "tile.students_sick",
      { kind: "into", row: 0, index: 1 },
    );
    expect(emptied.map((entry) => entry.row)).toEqual([0, 0, 1]);
  });

  it("gibt dieselbe Anordnung zurück, wenn sich nichts ändert", () => {
    expect(
      movePlacement(rows, "section.birthdays", { kind: "newRow", before: 2 }),
    ).toBe(rows);
    expect(
      movePlacement(rows, "tile.students_sick", {
        kind: "into",
        row: 0,
        index: 1,
      }),
    ).toBe(rows);
  });

  it("teilt eine Reihe, wenn eine Breite sie überlaufen lässt", () => {
    const next = placementWithSpan(rows, "section.open_requests", 4);

    expect(rowsAsKeys(next)).toEqual([
      ["tile.students_present", "tile.students_sick"],
      ["section.open_requests"],
      ["section.staff_today"],
      ["section.birthdays"],
    ]);
  });

  it("hängt einen Baustein in die letzte Reihe oder darunter an", () => {
    expect(
      appendPlacement(rows, "section.messages", 2).map((entry) => entry.row),
    ).toEqual([0, 0, 1, 1, 2, 3]);
    expect(
      appendPlacement(rows.slice(0, 2), "section.messages", 2).map(
        (entry) => entry.row,
      ),
    ).toEqual([0, 0, 0]);
  });

  it("rechnet Reihen in Rasterzellen um", () => {
    const cells = computeBoardCells(rows, 4);

    expect(cells.get("tile.students_sick")).toEqual({
      columnStart: 2,
      columnSpan: 1,
      rowStart: 1,
      rowSpan: 1,
    });
    expect(cells.get("section.staff_today")).toEqual({
      columnStart: 3,
      columnSpan: 2,
      rowStart: 2,
      rowSpan: 2,
    });
    expect(cells.get("section.birthdays")).toEqual({
      columnStart: 1,
      columnSpan: 4,
      rowStart: 4,
      rowSpan: 2,
    });
  });

  // Auf zwei Spalten bricht eine breite Reihe innerhalb ihres Bandes um.
  it("bricht eine Reihe auf einem schmalen Raster um", () => {
    const cells = computeBoardCells(rows, 2);

    expect(cells.get("section.staff_today")).toEqual({
      columnStart: 1,
      columnSpan: 2,
      rowStart: 4,
      rowSpan: 2,
    });
    expect(cells.get("section.birthdays")?.columnSpan).toBe(2);
  });
});

describe("Reihen — volle Reihe (#2180)", () => {
  const rows: HomeBlockPlacement[] = [
    { key: "tile.students_present", span: 1, row: 0 },
    { key: "section.open_requests", span: 2, row: 1 },
    { key: "section.staff_today", span: 2, row: 1 },
  ];

  // Ein Zug über eine volle Reihe hinweg darf keine Spuren hinterlassen:
  // sonst teilt sich die Reihe, nur weil der Zeiger sie gestreift hat.
  it("lässt eine volle Reihe in Ruhe", () => {
    expect(
      movePlacement(rows, "tile.students_present", {
        kind: "into",
        row: 1,
        index: 0,
      }),
    ).toBe(rows);
  });
});
